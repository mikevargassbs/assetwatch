# Deployment Guide

Production deployment guide for the SBS-BSP CCTV Hardware Lifecycle
Management System. This is a single self-contained Go binary (frontend
embedded) plus a PostgreSQL 16 database — no other runtime dependencies.

For local development, see [README.md](README.md). This document covers
deploying to a production server.

This guide targets **Windows Server**, deploying as a **native binary**
under [NSSM](https://nssm.cc/) — no Docker required. It assumes
**PostgreSQL 16 is already installed and running** on (or reachable
from) the server, so there's no Postgres install step — only creating
the application's database and login.

## Contents

- [1. Prerequisites](#1-prerequisites)
- [2. Building the binaries](#2-building-the-binaries)
- [3. Native binary deployment](#3-native-binary-deployment)
- [3.7 Directory layout on the server](#37-directory-layout-on-the-server)
- [4. Reverse proxy & TLS](#4-reverse-proxy--tls)
- [5. Post-deploy verification](#5-post-deploy-verification)
- [6. Upgrades](#6-upgrades)
- [7. Backups & restore](#7-backups--restore)
- [8. Environment variable reference](#8-environment-variable-reference)
- [9. Security checklist](#9-security-checklist)
- [10. Troubleshooting](#10-troubleshooting)
- [11. Alternative: Docker Compose deployment](#11-alternative-docker-compose-deployment)

---

## 1. Prerequisites

- Windows Server with administrator access.
- **PostgreSQL 16 already installed and running** — either on this
  server or reachable over the network. You'll need `psql` access (or
  pgAdmin) with a superuser role to create the application's database
  and login in [§3.1](#31-create-the-application-database).
- A domain name pointed at the server (for TLS), unless you're serving
  over the internal network only.
- **Go 1.25+ and Node.js 22+** on a build machine to build the binaries
  (see [§2](#2-building-the-binaries)) — this does not need to be the
  production server itself. Prebuilt binaries already exist under
  `dist/windows-amd64/` if you don't need to rebuild.
- [NSSM](https://nssm.cc/) downloaded on the server, to run the binary as
  a Windows service — see [§3.6](#36-run-as-a-windows-service-nssm).
- Outbound network access if you want defect-report emails to actually
  send (SMTP is optional — see [§8](#8-environment-variable-reference)).

---

## 2. Building the binaries

The backend is plain Go with no cgo dependencies, so it cross-compiles
cleanly to Windows from any OS. Do this on a build machine (your
workstation, a CI runner — it doesn't need to be the production server).
Two binaries are produced:

- `sbs-api.exe` — the API server, and (once the frontend is built —
  step 1 below) it serves the whole web UI too, so **this one binary is
  the entire application**.
- `sbs-seed-admin.exe` — a one-off CLI that bootstraps the first Admin
  user (there's no public self-registration).

### 2.1 Build from Windows (PowerShell)

```powershell
# 1. Build the frontend first — its output gets embedded into the Go binary.
cd web; npm install; npm run build; cd ..

# 2. Build both binaries for Windows, stamping version info into the binary:
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
$version = "1.0.9"                                  # bump per release
$commit = (git rev-parse --short HEAD)
$buildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$versionPkg = "sbs-bsp-cctv/internal/version"
$ldflags = "-s -w -X $versionPkg.Version=$version -X $versionPkg.Commit=$commit -X $versionPkg.BuildDate=$buildDate"
go build -ldflags="$ldflags" -o dist/windows-amd64/sbs-api.exe ./cmd/api
go build -ldflags="$ldflags" -o dist/windows-amd64/sbs-seed-admin.exe ./cmd/seed-admin
```

### 2.2 Or cross-compile from macOS/Linux

```bash
cd web && npm install && npm run build && cd ..

VERSION="v1.0.0"   # bump per release
COMMIT=$(git rev-parse --short HEAD)
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG="sbs-bsp-cctv/internal/version"
LDFLAGS="-s -w -X $VERSION_PKG.Version=$VERSION -X $VERSION_PKG.Commit=$COMMIT -X $VERSION_PKG.BuildDate=$BUILD_DATE"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o dist/windows-amd64/sbs-api.exe ./cmd/api
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o dist/windows-amd64/sbs-seed-admin.exe ./cmd/seed-admin
```

> If you build `cmd/api` **before** running `npm run build`, it still
> compiles fine (a placeholder keeps `go:embed` happy) — it just won't
> have a real UI embedded. Always build the frontend first for a real
> release.

`-s -w` strips debug symbols to shrink the binary; the `-X` flags stamp
`internal/version` with the release version, commit, and build date, so
running binaries can be identified without a debugger — check via
`GET /healthz`, which returns `{"status":"ok","version":...,"commit":...,"buildDate":...}`,
or via the startup log line. Omit `-s -w` if you need to debug with `dlv`;
omit the `-X` flags for a quick local build (version falls back to `dev`).

This produces `dist/windows-amd64/sbs-api.exe` and
`dist/windows-amd64/sbs-seed-admin.exe`. Continue with
[§3 Native binary deployment](#3-native-binary-deployment).

---

## 3. Native binary deployment

### 3.1 Create the application database

Postgres is already running, so just create the app's database and login
on it. From `psql` (or pgAdmin) as a superuser:

```sql
CREATE USER sbsadmin WITH PASSWORD 'change-me-to-something-random';
CREATE DATABASE assetwatch_db OWNER sbsadmin;
```

### 3.2 Copy the release binaries to the server

Copy `dist/windows-amd64/sbs-api.exe` and
`dist/windows-amd64/sbs-seed-admin.exe` (built in [§2](#2-building-the-binaries))
to the target machine, e.g. `C:\sbs-cctv\`.

### 3.3 Run migrations

From any machine with Go and network access to the production database
(this can be your workstation — it doesn't need to be the server):

```powershell
go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations postgres "postgres://sbsadmin:change-me@<db-host>:5432/assetwatch_db?sslmode=disable" up
```

### 3.4 Create the environment file

```powershell
New-Item -ItemType Directory -Force C:\sbs-cctv
@'
DATABASE_URL=postgres://sbsadmin:change-me@localhost:5432/assetwatch_db?sslmode=disable
JWT_SECRET=change-me-to-a-long-random-string
ALLOWED_EMAIL_DOMAIN=sbs.com.pg
PORT=4083
'@ | Out-File -Encoding utf8 C:\sbs-cctv\.env
```

Generate strong random values instead of placeholders — e.g. in
PowerShell:

```powershell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))   # POSTGRES password
[Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Maximum 256 }))   # JWT_SECRET
```

Restrict the `.env` file's ACLs so only the account running the service
(and admins) can read it.

### 3.5 Seed the first Admin

```powershell
cd C:\sbs-cctv
$env:DATABASE_URL = "postgres://sbsadmin:change-me@localhost:5432/assetwatch_db?sslmode=disable"
$env:JWT_SECRET = "change-me-to-a-long-random-string"
.\sbs-seed-admin.exe -email you@sbs.com.pg -name "Your Name" -password "ChangeMe123!"
```

Change the password immediately after first login.

### 3.6 Run as a Windows service (NSSM)

**Install NSSM** (if not already on the server):

```powershell
# Download and unzip (adjust version as needed; check https://nssm.cc/download)
Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "$env:TEMP\nssm.zip"
Expand-Archive -Path "$env:TEMP\nssm.zip" -DestinationPath "$env:TEMP\nssm" -Force
New-Item -ItemType Directory -Force "C:\nssm"
Copy-Item "$env:TEMP\nssm\nssm-2.24\win64\nssm.exe" "C:\nssm\nssm.exe"

# Add C:\nssm to PATH for this session (or add it permanently via System Properties)
$env:PATH += ";C:\nssm"
```

(Alternatively, install via `choco install nssm` if Chocolatey is
available, or download the zip manually from
[nssm.cc/download](https://nssm.cc/download) and extract `nssm.exe` from
the `win64` folder.)

**Create log directory and register the service:**

```powershell
New-Item -ItemType Directory -Force C:\sbs-cctv\logs

nssm install sbs-api "C:\sbs-bsp-cctv\sbs-api.exe"
nssm set sbs-api AppDirectory "C:\sbs-bsp-cctv"
nssm set sbs-api AppEnvironmentExtra `
  DATABASE_URL=postgres://sbsadmin:change-me@localhost:5432/assetwatch_db?sslmode=disable `
  JWT_SECRET=change-me-to-a-long-random-string `
  ALLOWED_EMAIL_DOMAIN=sbs.com.pg `
  PORT=4083

# Capture stdout/stderr to rotating log files
nssm set sbs-api AppStdout "C:\sbs-bsp-cctv\logs\sbs-api.log"
nssm set sbs-api AppStderr "C:\sbs-bsp-cctv\logs\sbs-api.log"
nssm set sbs-api AppRotateFiles 1
nssm set sbs-api AppRotateOnline 1
nssm set sbs-api AppRotateBytes 10485760   # rotate at 10MB

# Auto-restart on crash, auto-start on boot
nssm set sbs-api AppExit Default Restart
nssm set sbs-api Start SERVICE_AUTO_START

nssm start sbs-api
nssm status sbs-api
```

`AppEnvironmentExtra` keeps the secrets out of the service's own
configuration file; alternatively, NSSM will also happily source
`C:\sbs-bsp-cctv\.env` if you set `AppDirectory` and have the app read it via
its own `.env` loader — either approach works, just don't do both with
conflicting values.

Check the service is healthy:

```powershell
Get-Service sbs-api
Get-Content C:\sbs-cctv\logs\sbs-api.log -Tail 50 -Wait
```

**Common NSSM commands for later:**

```powershell
nssm restart sbs-api
nssm stop sbs-api
nssm edit sbs-api      # opens the GUI editor for all settings
nssm remove sbs-api confirm   # uninstalls the service (does not delete files)
```

Continue with [§3.7 Directory layout on the server](#37-directory-layout-on-the-server).

---

### 3.7 Directory layout on the server

Everything the app needs lives under one root, e.g. `C:\sbs-cctv\` (the
`AppDirectory` NSSM was pointed at in [§3.6](#36-run-as-a-windows-service-nssm)).
Uploaded files are written to `data\uploads\...` **relative to that working
directory**, so it must exist (and stay put — see the troubleshooting note
in [§10](#10-troubleshooting)) before the service starts:

```
C:\sbs-cctv\
├── sbs-api.exe                    # from §2 / §3.2
├── sbs-seed-admin.exe              # from §2 / §3.2 (only needed to run once, §3.5)
├── .env                            # from §3.4 (if not using AppEnvironmentExtra)
├── logs\
│   └── sbs-api.log                 # NSSM stdout/stderr, auto-rotated (§3.6)
└── data\
    └── uploads\
        ├── acceptance\             # client acceptance documents
        └── installation\           # installation photos
```

- `logs\` and `data\uploads\` are created automatically (by NSSM and the
  app respectively) if missing, but they must be on the **same persistent
  volume** as the rest of the deployment — don't point `AppDirectory` at a
  path that gets wiped or replaced on redeploy.
- Only `data\uploads\` needs to be included in backups/DR — everything
  else is either reproducible from a rebuild (`sbs-api.exe`,
  `sbs-seed-admin.exe`) or reconstructible from secrets you already store
  elsewhere (`.env`). See [§7](#7-backups--restore).
- The two binaries can technically live anywhere as long as
  `AppDirectory` is set correctly, but keeping them alongside `data\` and
  `logs\` under one root keeps upgrades ([§6](#6-upgrades)) simple — stop
  the service, replace the `.exe`, start it again.

Continue with [§4 Reverse proxy & TLS](#4-reverse-proxy--tls).

---

## 4. Reverse proxy & TLS

Put a reverse proxy in front of port `4083` for TLS termination. Do not
expose `4083` directly to the internet.

**IIS with Application Request Routing (ARR)** — the native Windows
Server option:

1. Install the **URL Rewrite** and **Application Request Routing**
   modules (via Web Platform Installer or direct download from
   iis.net).
2. In IIS Manager, enable ARR's proxy feature (Server node →
   *Application Request Routing Cache* → *Server Proxy Settings* →
   check *Enable proxy*).
3. Create a site/binding for your domain on ports 80/443, bind your TLS
   certificate (import a cert via IIS Manager → *Server Certificates*,
   or issue one with `win-acme` for automatic Let's Encrypt renewal).
4. Add a URL Rewrite rule on that site to reverse-proxy to
   `http://localhost:4083`:

```xml
<rule name="ReverseProxyToApi" stopProcessing="true">
  <match url="(.*)" />
  <action type="Rewrite" url="http://localhost:4083/{R:1}" />
</rule>
```

5. Raise the upload size limit for acceptance-document uploads (IIS
   defaults to ~28MB, which is close to the app's own limit — set
   `maxAllowedContentLength` in `web.config`'s
   `<requestFiltering>` to be safe, e.g. `30000000`).

**win-acme** (`winacme` / `wacs.exe`) is the simplest way to get and
auto-renew a Let's Encrypt certificate on Windows Server and bind it into
IIS.

**Alternative**: Caddy or nginx for Windows work the same way as their
Linux counterparts (a `reverse_proxy localhost:4083` directive, or
`proxy_pass http://localhost:4083;`) if you'd rather not use IIS.

Open firewall ports 80 and 443 only; keep 4083 and 5432 closed to the
outside world:

```powershell
New-NetFirewallRule -DisplayName "HTTP" -Direction Inbound -Protocol TCP -LocalPort 80 -Action Allow
New-NetFirewallRule -DisplayName "HTTPS" -Direction Inbound -Protocol TCP -LocalPort 443 -Action Allow
```

---

## 5. Post-deploy verification

1. `curl -I https://your-domain.example` — should return `200`.
2. Log in with the seeded Admin account through the UI.
3. Confirm roles are present: `admin`, `pm_pc`, `encoder`, `configurator`,
   `qc`, `logistics`, `field_technician`, `bsp_acceptance_officer`,
   `reports_viewer` (seeded by migration `00002_users_roles_audit.sql`).
4. Create a second user from **Admin → Users** (or
   `POST /api/v1/users`) and confirm the `ALLOWED_EMAIL_DOMAIN` restriction
   is enforced (an email outside the domain should be rejected).
5. Upload a test acceptance document and confirm it persists after a
   service restart (`data\uploads\acceptance\`).

---

## 6. Upgrades

Rebuild the binaries per [§2](#2-building-the-binaries), then on the
server:

```powershell
nssm stop sbs-api
# replace C:\sbs-cctv\sbs-api.exe with the new binary
nssm start sbs-api
```

Always run new migrations (`db/migrations/`) as part of every upgrade that
includes them — they are never applied automatically (see [§3.3](#33-run-migrations)).

---

## 7. Backups & restore

The database holds every stage record and the full audit trail — back it
up on a schedule.

**Backup:**

```powershell
pg_dump -U sbsadmin assetwatch_db | Out-File -Encoding ascii "backup-$(Get-Date -Format yyyy-MM-dd).sql"
```

(Or use `pg_dump -Fc` for a compressed custom-format dump, restorable with
`pg_restore`.)

Also back up (or snapshot) the `data\uploads\acceptance\` directory —
acceptance documents live on disk, not in the database.

**Restore:**

```powershell
psql -U sbsadmin -d assetwatch_db -f backup-2026-07-17.sql
```

Schedule `pg_dump` via Windows Task Scheduler, and store backups off the
server (S3, another host, etc.).

---

## 8. Environment variable reference

| Variable               | Required | Default        | Notes                                                                 |
|-------------------------|:--------:|----------------|--------------------------------------------------------------------------|
| `DATABASE_URL`          | yes      | —              | `postgres://user:pass@host:port/db?sslmode=disable`                    |
| `JWT_SECRET`            | yes      | —              | Signing key for access tokens. Long random value in production. Rotating it invalidates all active sessions. |
| `ALLOWED_EMAIL_DOMAIN`  | no       | `sbs.com.pg`   | Only accounts with this email domain can log in.                       |
| `PORT`                  | no       | `4083`         | HTTP port the API (and embedded frontend) listens on.                  |
| `SMTP_HOST`             | no       | *(unset)*      | If unset, "email defect report to supplier" records the intent without sending, instead of failing. |
| `SMTP_PORT`             | no       | —              | e.g. `587`                                                              |
| `SMTP_USERNAME`         | no       | —              | Leave unset for an unauthenticated relay.                               |
| `SMTP_PASSWORD`         | no       | —              |                                                                          |
| `SMTP_FROM`             | no       | —              | Required (along with `SMTP_HOST`) for outbound mail to actually send.   |

---

## 9. Security checklist

- [ ] `JWT_SECRET` is a long random value, not the dev default.
- [ ] The `sbsadmin` Postgres password (embedded in `DATABASE_URL`) is a
      long random value, not a placeholder.
- [ ] Postgres is not exposed to the internet (firewall blocks 5432
      externally).
- [ ] TLS is terminated at the reverse proxy; port 4083 is not directly
      internet-facing.
- [ ] `ALLOWED_EMAIL_DOMAIN` matches your organization's real domain.
- [ ] The seeded Admin's initial password is changed after first login.
- [ ] Backups are scheduled and tested (a restore has actually been
      tried, not just the dump command).
- [ ] The `sbs-api` NSSM service runs as a dedicated low-privilege
      service account, not as `LocalSystem` or an admin account (`nssm
      set sbs-api ObjectName <domain\user> <password>`).
- [ ] `C:\sbs-cctv\.env` has restrictive NTFS permissions (only the
      service account and admins can read it).
- [ ] `data\uploads\` is included in your backup/DR plan — it isn't in
      the database.

---

## 10. Troubleshooting

**`go build` / `npm run build` fails on the build machine:**
Confirm Go 1.25+ and Node.js 22+ are installed (`go version`, `node
-v`). Run `npm install` in `web/` before `npm run build` if
`node_modules` is missing or out of date.

**API service won't start, or `Get-Service sbs-api` shows it stopping
immediately:**
Check `AppEnvironmentExtra` / `C:\sbs-cctv\.env` has a `DATABASE_URL`
matching the actual Postgres user/password created in
[§3.1](#31-create-the-application-database). Check NSSM's configured
stdout/stderr log file (`C:\sbs-cctv\logs\sbs-api.log`, or run `nssm get
sbs-api AppStdout`) for the actual error.

**`nssm` isn't recognized as a command:**
It's not on `PATH` for the current session — either re-run the `$env:PATH
+= ";C:\nssm"` line from [§3.6](#36-run-as-a-windows-service-nssm), or add
`C:\nssm` to the system `PATH` permanently via System Properties →
Environment Variables, then open a new PowerShell window.

**Login fails for a valid user:**
Confirm the account's email ends in `ALLOWED_EMAIL_DOMAIN`; it's enforced
both by a DB constraint and in the application layer.

**"relation does not exist" errors after deploying a new version:**
Migrations weren't run. Apply them per [§3.3](#33-run-migrations) — they
are never automatic.

**Uploaded documents disappear after a redeploy:**
Confirm `data\uploads\acceptance\` is on persistent storage and not
accidentally overwritten when the binary is replaced.

**Emails aren't sending for defect reports:**
Expected if `SMTP_HOST`/`SMTP_FROM` are unset — this is a supported
no-op mode. The API response includes `"sent": false` and the record is
still marked as declared; send manually or configure SMTP.

---

## 11. Alternative: Docker Compose deployment

If a different server (or a future environment) can run Docker, the repo
also ships a Docker Compose path via `docker-compose.prod.yml` — Postgres
and the API run as containers, and upgrades are a `git pull && docker
compose -f docker-compose.prod.yml up -d --build`. It is not used for the
current production server since Docker isn't installable there; the
native binary path above (§1–§10) is the one actually in use. If you stand
up a Docker-capable host later, ask for the Docker Compose walkthrough or
check `docker-compose.prod.yml` and the `Dockerfile` directly — they are
self-explanatory alongside the env var table in [§8](#8-environment-variable-reference)
(add `POSTGRES_USER=sbsadmin`, `POSTGRES_PASSWORD`, `POSTGRES_DB` to
`.env` for that path).
