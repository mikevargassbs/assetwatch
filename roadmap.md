# SBS-BSP CCTV Hardware Lifecycle Management System — Roadmap

## 1. Overview

A system to track IT/CCTV hardware and equipment across its full lifecycle:
arrival → pre-deployment configuration & QC → logistics/shipping → site
installation → client acceptance sign-off, with full audit trails at every
step, plus a defective/damaged/wrong-item (RMA) side-process and a
reporting module.

## 2. Tech Stack

| Layer          | Choice                                                            |
|----------------|--------------------------------------------------------------------|
| Backend        | Go (Golang) — REST/JSON API                                       |
| Database       | PostgreSQL (JSONB for dynamic `meta_data`)                        |
| Auth           | JWT (access + refresh tokens)                                     |
| Authorization  | Role-based access control (RBAC)                                  |
| Frontend       | SPA, served on port **8083**                                      |
| File storage   | Local disk or S3-compatible bucket (scanned sign-off docs, photos) |
| PDF/Barcode    | Server-side generation (barcode stickers, packing lists, reports)  |
| E-signature    | Signed, expiring link + JWT token for client sign-off page         |
| OS targets     | Windows Server/desktop and Ubuntu (Linux) — single Go binary, cross-compiled for both |
| Responsive UI  | Mobile-first CSS (flex/grid + breakpoints), works on desktop, tablet, and phone |

### Suggested Go project layout

```
/cmd/api                → main entrypoint
/cmd/frontend            → static file server / SPA host (port 8083)
/internal/auth           → JWT issuing/verification, middleware
/internal/rbac           → role & permission checks
/internal/audit          → audit trail writer (middleware + service)
/internal/hardware       → stage 1A/1B domain logic
/internal/logistics      → stage 2 domain logic
/internal/installation   → stage 3 domain logic
/internal/acceptance     → stage 4 domain logic + e-signature
/internal/defective      → stage 5 / RMA domain logic
/internal/reporting      → report generation (PDF/CSV)
/internal/metadata       → dynamic meta_data field registry
/db/migrations           → SQL migrations
/web                     → frontend source
```

## 3. Core Design Decisions

### 3.1 Dynamic fields via `meta_data`

Every stage table has a fixed set of "core" columns (the fields that drive
business logic, joins, and reporting filters) plus a `meta_data JSONB`
column for anything else. New fields (e.g. a new Axis feature flag, a new
DNS setting) get added by:

1. Inserting a row into a `meta_data_field_definitions` table
   (`stage`, `field_key`, `label`, `data_type`, `is_required`, `sort_order`).
2. Frontend renders the extra fields dynamically from that definition table.
3. No migration needed for new optional fields — only core/reportable
   fields ever require a schema migration + index.

This keeps migrations rare and keeps Stage 1‑A (which has the most
fields today, and will grow) flexible.

### 3.2 Login restrictions & super user

- **Domain-restricted signup/login**: only email addresses ending in
  `@sbs.com.pg` may register or log in. Enforced server-side at account
  creation time (reject any other domain with a clear error) — this is
  a validation rule, not just a UI restriction.
- **Super user bootstrap**: the first user (you) is seeded directly in
  the database (via a seed migration or a one-off CLI command, e.g.
  `go run ./cmd/seed-admin`) with the `Admin` role, so there's no
  chicken-and-egg problem of needing an admin to create the first admin.
- Going forward, **only Admins can create new user accounts** — there
  is no public self-registration page. Admin creates the account
  (email must be `@sbs.com.pg`), assigns a role, and the system emails
  the new user a link to set their password (or Admin sets a temp
  password directly).
- Every account-creation and role-change event is written to
  `audit_trails` (see 3.3 below).

### 3.3 Audit trail

A single `audit_trails` table (or partitioned by month) records every
create/update/status-change/sign-off/print event across all stages:

```
id, entity_type, entity_id, action, performed_by, performed_at,
old_value JSONB, new_value JSONB, ip_address, notes
```

Populated via a shared `internal/audit` service called from every
stage's service layer (not left to controllers) so it can't be
accidentally skipped.

### 3.4 RBAC roles (initial set — adjust as needed)

- **Admin** — full access, user management, field-definition management
- **PM/PC** — receiving verification, branch allocation, store
  check-in/out custody, escalation point for site-side discrepancies
- **Encoder** — Stage 1‑A data entry
- **Configurator** — Stage 1‑B data entry
- **QC** — quality control sign-off (Stage 1‑A & 1‑B)
- **Logistics** — Stage 2
- **Field Technician** — Stage 3
- **Client** — Stage 4 sign-off only (scoped to their own record via
  signed link, not a full login)
- **BSP Acceptance Officer** — Stage 4 internal acceptance
- **Reports Viewer** — read-only reporting access

### 3.5 Entity flow (high level)

```
Hardware Unit
 └── Stage 1A record (arrival/config/QC)
 └── Stage 1B record (firmware/config QC)
 └── Stage 2 record (logistics/shipping)  — can repeat for RMA replacement
 └── Stage 3 record (site installation)
 └── Stage 4 record (client acceptance)
 └── Stage 5 record (defective/damage/wrong item) — 0..n, triggers a
       return-and-replace loop back into Stage 1A/2
```

A `hardware_units` table is the anchor row (barcode, serial, current
stage/status). All stage tables FK to it.

### 3.6 Frontend: Kanban board view

The primary frontend view is a **Kanban board**, one card per hardware
unit, grouped into columns that map to the lifecycle:

```
Pre-Deployment → Configuration → Shipment → Installation → Commissioning → Completed
```

Column-to-stage mapping:

| Column         | Backed by                          | Card moves to next column when...                     |
|----------------|-------------------------------------|--------------------------------------------------------|
| Pre-Deployment | Stage 1‑A (arrival, encode, QC)     | Stage 1‑A signed off (encoded + configured + QC ticked) |
| Configuration  | Stage 1‑B (firmware, config QC)     | Stage 1‑B signed off                                    |
| Shipment       | Stage 2 (logistics)                 | Checked in at site / delivered                          |
| Installation   | Stage 3 (site installation)         | Fit & focus completed, device contactable               |
| Commissioning  | Stage 4 (client acceptance)         | Both BSP + client sign-off recorded                     |
| Completed      | Terminal state                      | —                                                        |

Notes:

- A card's column is **derived from `hardware_units.current_stage`**,
  not dragged-and-dropped freely — moving a card to the next column is
  a side effect of completing that stage's required sign-offs via a
  form/modal, not a manual drag action. This keeps the board consistent
  with the audit trail and prevents skipping required approvals.
  (Optional: allow drag purely as a shortcut that opens the relevant
  "complete this stage" form, rather than committing the move directly.)
- **Stage 5 (defective/damage/wrong item)** is not its own column — a
  unit in this state is flagged with a badge/red border on its current
  card (or pulled into a filterable "Exceptions" side panel) since it's
  a branch off the main flow, not a forward stage.
- Each card shows: barcode, device name/model, site name (once known),
  current owner/assignee for that stage, and a status badge (on track /
  blocked / exception).
- Clicking a card opens the full record with tabs for each stage's data
  and the audit trail for that unit.
- Board should support filtering (by site, device model, deployment
  team) and a swimlane-by-site option for logistics/installation teams
  tracking multiple units per site.

### 3.7 Detailed intake & store custody flow (per shared-process diagram)

The client-supplied swimlane diagram (Logistics / PM-PC / I.T / Site)
adds detail that refines Stages 1‑A, 1‑B, and 2 — it does not change
the Kanban columns, but it does add checkpoints and a second exception
path that the data model needs to support:

```
Logistics: Start → Hardware Received at SBS
PM/PC:     → All Items Correct? ──NO──→ Contact CH10 (supplier query)
                     │YES
                     ▼
           Devices Allocated to Branch → Devices Checked Out From Store
I.T:                 → Device Configuration → Configuration QA Passed? ──NO──┐
                     │YES                                                    │(loop back to
                     ▼                                                       │ Device Configuration)
PM/PC:     Devices Checked In To Store
Logistics: → Devices Checked Out From Store → Dispatched To Site
Site:      → Received On Site → Confirmed As Correct? ──NO──→ Contact PM/PC
                     │YES
                     ▼
           Install Process → End
```

Mapping onto existing stages/roles:

- **Hardware Received at SBS** + **All Items Correct?** is a receiving
  check that happens *before* Stage 1‑A encoding — verifies the
  physical delivery against the supplier order/waybill. A "NO" here is
  a receiving-level exception (wrong/missing item from supplier),
  logged and escalated to the supplier contact ("CH10"), distinct from
  Stage 5 (which covers defects found later, post-configuration or
  post-installation). Model as `stage0_receiving`
  (`hardware_unit_id, received_by, received_date, po_or_waybill_ref,
  items_correct (bool), discrepancy_notes, escalated_to, escalated_at`).
- **Devices Allocated to Branch**: an early "intended destination"
  assignment (site/branch) recorded before configuration starts —
  this is a lightweight field on `hardware_units`
  (`allocated_branch`), separate from the full `site_name` /
  `site_location` detail captured later in Stage 2 logistics.
- **Devices Checked Out/In From Store** (both legs — once before I.T
  configuration, once after QC, once again before dispatch): this is
  a **store custody log**, not a one-off field, since a unit passes
  through the store multiple times. Model as `store_custody_log`
  (`hardware_unit_id, action (checked_out|checked_in), by, at, purpose
  (for_configuration|for_dispatch), notes`) rather than trying to
  cram three timestamps onto one table.
- **Device Configuration** / **Configuration QA Passed?** = Stage 1‑B
  as already modeled, including the retry loop (QA fail sends it back
  to configuration — already implicit in `stage1b_configuration` since
  it isn't signed off until QC passes).
- **Dispatched to Site** = Stage 2 logistics (as modeled).
- **Received on Site** / **Confirmed as Correct?** = a receiving check
  at the *site* end, before install — a second exception path distinct
  from both the SBS receiving check and Stage 5. A "NO" here escalates
  to PM/PC (internal), not the supplier. Model as fields on
  `stage3_installation`: `received_on_site_at, confirmed_correct (bool),
  discrepancy_notes, escalated_to_pmpc_at`.
- **Install Process** = the rest of Stage 3 as already modeled.

Additional role: **PM/PC** (Project Manager / Program Coordinator) —
add to RBAC roles list (3.4): owns branch allocation, store check-in/
out custody, and is the escalation point for site-side discrepancies.

Both exception paths (`items_correct = false` at SBS receiving, and
`confirmed_correct = false` at site) should raise the same
`is_exception` flag / red-badge treatment on the Kanban card described
in 3.6, so blocked units are visible on the board without needing a
Stage 5 record — Stage 5 remains reserved for defects/damage found
*after* the item has been accepted as correct at least once.

### 3.8 Cross-platform deployment & responsive design

- **Backend**: plain Go binary, no OS-specific dependencies (avoid
  cgo-only libraries — use pure-Go PostgreSQL driver, pure-Go PDF/
  barcode libraries) so `GOOS=windows` and `GOOS=linux` builds both
  come from the same codebase via `go build`/cross-compilation. Ship
  as a Windows service (or just an `.exe` run under NSSM/Task
  Scheduler) and as a systemd service on Ubuntu.
- **Database**: PostgreSQL runs natively on both Windows and Ubuntu;
  no platform-specific SQL features — stick to standard PostgreSQL/
  JSONB so the same schema/migrations work on either host.
- **Packaging**: `docker-compose` as the primary deployment path
  (works identically on Windows w/ Docker Desktop and Ubuntu w/
  Docker Engine) — avoids having to hand-tune two separate native
  install procedures. Native (non-Docker) install docs as a fallback
  for environments without Docker.
- **Frontend responsiveness**: mobile-first layout using flexbox/CSS
  grid with breakpoints for phone / tablet / desktop. Practical
  implications for this specific UI:
  - The **Kanban board** (3.6) collapses to a vertical, single-column
    "stage list" with a column switcher on phone widths, a 2‑3 column
    scrollable board on tablet, and the full 6-column board on
    desktop.
  - Data-entry forms (Stage 1‑A's many fields, `meta_data` dynamic
    fields) stack to single-column layouts on small screens rather
    than multi-column grids.
  - Barcode/report PDFs are generated server-side either way, so print
    output is unaffected by screen size — only the on-screen forms/
    board need responsive treatment.
  - Signature capture (Stage 4) must work with touch input (finger/
    stylus on tablet/phone) as well as mouse on desktop.

## 4. Data Model Sketch

### `hardware_units`
`id, barcode (unique), status, current_stage, board_column, is_exception (bool),
allocated_branch, created_at, updated_at, meta_data JSONB`

`board_column` is a derived/denormalized enum
(`pre_deployment | configuration | shipment | installation | commissioning | completed`)
kept in sync by the stage sign-off services, so the Kanban board can
query it directly without recomputing from all stage tables on every
read. `is_exception` is set when there's an open Stage 5 record, a
failed receiving check (`stage0_receiving.items_correct = false`), or
a failed site confirmation (`stage3_installation.confirmed_correct = false`)
against the unit — see 3.7.

### `stage0_receiving` (SBS goods-in verification, precedes Stage 1‑A)
`hardware_unit_id, received_by, received_date, po_or_waybill_ref,
items_correct (bool), discrepancy_notes, escalated_to, escalated_at,
meta_data JSONB`

### `stage1a_configuration` (Pre-deployment Config & QC)
Core columns:
`hardware_unit_id, device_make, device_model, device_name_dns,
client_ip_address, serial_number, mac_address,
dns_server_1, dns_server_2, ntp_server, frequency_hz,
default_username, default_password (encrypted),
encoded_by, configured_by, qc_by, encoded_at, configured_at, qc_at,
barcode_printed_at, meta_data JSONB`

`meta_data` holds: axis object analytics installed/enabled, axis video
motion detection installed/enabled, axis scene metadata on/off,
certificate installed, and any future Axis/vendor feature flags.

### `stage1b_configuration`
`hardware_unit_id, configured_by, configured_date, firmware_updated (bool),
firmware_version, configuration_qc_by, qc_date, signed_off_at, meta_data JSONB`

### `store_custody_log` (multi-leg check-out/check-in tracking)
`id, hardware_unit_id, action (checked_out|checked_in), by, at,
purpose (for_configuration|for_dispatch), notes`

### `stage2_logistics`
`hardware_unit_id, site_name, site_location, site_ip, site_subnet,
site_gateway, deployment_date, deployment_team, team_leader,
issued_to, date_checked_out, date_checked_in,
shipped_via (air/sea/land), shipping_provider, waybill_number,
packing_list_printed_at, meta_data JSONB`

### `stage3_installation`
`hardware_unit_id, received_on_site_at, confirmed_correct (bool),
discrepancy_notes, escalated_to_pmpc_at,
date_installed, installed_by, inspected_by,
network_attached (bool), device_contactable (bool), ping_checked_at,
fit_focus_by, fit_focus_completed_at, installed_location,
installed_height_m, meta_data JSONB`

### `stage4_acceptance`
`hardware_unit_id, method (e_signature | manual_upload),
bsp_acceptance_by, bsp_signature, bsp_acceptance_date,
client_signature_link_token, client_signed_at, client_signature_data,
uploaded_document_path, comments, final_acceptance_notes, meta_data JSONB`

### `stage5_defective`
`hardware_unit_id, declared_by, declared_date, defect_type
(defective|damaged|wrong_item), description, report_generated_at,
emailed_to_supplier_at, supplier_email, replacement_status
(pending|shipped_back|replacement_received), replacement_hardware_unit_id,
meta_data JSONB`

### `meta_data_field_definitions`
`id, stage, field_key, label, data_type, is_required, sort_order, active`

### `audit_trails`
`id, entity_type, entity_id, action, performed_by, performed_at,
old_value JSONB, new_value JSONB, ip_address, notes`

### `users` / `roles` / `user_roles`
Standard RBAC join tables, plus `refresh_tokens` for JWT rotation.

## 5. Phased Delivery Plan

### Phase 0 — Foundations (Week 1-2)
- Repo scaffolding, Go module setup, PostgreSQL setup, migration tool
  (e.g. `golang-migrate` or `goose`)
- `users`, `roles`, `user_roles`, JWT auth (login, refresh, logout)
- Domain-restricted login (`@sbs.com.pg` only) + super user seed script
- RBAC middleware
- Audit trail service (shared package, wired into a base
  create/update handler pattern)
- Frontend shell on port 8083 (routing, login page, protected routes)
- Kanban board shell (6 columns, cards driven by `hardware_units.board_column`)

### Phase 1 — Receiving, Stage 1‑A & 1‑B (Week 3-5)
- `hardware_units` + `stage0_receiving` (goods-in check, branch
  allocation, "Contact CH10" exception path)
- `stage1a_configuration` + `meta_data_field_definitions`
- Dynamic meta-data form renderer (frontend) + admin UI to manage field
  definitions
- Barcode sticker generation (MAC + IP + barcode) — printable PDF/label
- Sign-off flow: encoded by / configured by / QC by, each gated by role
- `stage1b_configuration` + firmware fields + QC sign-off
- `store_custody_log` check-out/check-in tracking around configuration
- Enforce: Stage 2 cannot start until receiving, 1A, and 1B are all
  signed off
- Audit trail entries for every field change and every sign-off

### Phase 2 — Logistics (Week 6-7)
- `stage2_logistics` CRUD
- Packing list report (PDF, per shipment/destination site)
- Store check-out (`for_dispatch`) + checkout/check-in tracking (issued
  to, dates)
- Status transition: unit moves to "in transit" / "delivered"

### Phase 3 — Site Installation (Week 8-9)
- `stage3_installation` CRUD, including "Received on Site" +
  "Confirmed as Correct?" check with "Contact PM/PC" exception path
- Ping/contactability check — either manual tick or backend-triggered
  ICMP check against `site_ip` (needs network access from server or a
  local agent)
- Fit & focus sign-off

### Phase 4 — Client Acceptance (Week 10-12)
- `stage4_acceptance`
- E-signature flow:
  - Generate signed, time-limited JWT link for client
  - Public (unauthenticated-by-login, authenticated-by-token) signing
    page — client draws/uploads signature, submits
  - Manual path: BSP staff uploads scanned signed document
- BSP internal acceptance sign-off with comments/notes
- Locks the record once both signatures are present

### Phase 5 — Defective/Damage/Wrong Item (Week 13-14)
- `stage5_defective`
- Report generation + email-to-supplier (SMTP integration)
- Replacement loop: link new hardware unit back through Stage 1A → 2
- Status tracking: pending → shipped back → replacement received

### Phase 6 — Reporting Module (Week 15-16)
- Summary of Hardware (filterable: by stage, status, site, date range,
  device model, etc.) — printable/exportable (PDF/CSV)
- Summary of Hardware by Site
- Packing List Report per site (reprint capability)
- Defective/Damage/Wrong Item Report
- All reports pull from a read-optimized view/materialized view joining
  `hardware_units` + all stage tables

### Phase 7 — Hardening & Launch (Week 17-18)
- Full audit trail review/export screen (admin)
- Rate limiting, JWT refresh rotation, password policy
- Backup/restore strategy for PostgreSQL
- Load test barcode/report generation
- Responsive QA pass: verify Kanban board, forms, and signature capture
  on desktop, tablet, and phone breakpoints
- Cross-platform verification: build/run on both Windows and Ubuntu
  (native binary + docker-compose paths)
- UAT with actual field/logistics/QC users
- Deployment (docker-compose: Go API + Postgres + frontend on 8083,
  validated on both Windows and Ubuntu hosts)

## 6. Open Questions / Decisions Needed

- Ping/contactability check (Stage 3): does the API server have network
  reach to client sites, or does this need a local on-site agent
  reporting back?
- E-signature: build a minimal in-house signature capture, or integrate
  a provider? (In-house is simpler given JWT-link requirement already
  described.)
- File storage target: local disk vs S3-compatible object storage for
  scanned documents/signatures/photos.
- Multi-tenancy: is this single-organization (BSP only) or will it need
  to support multiple client organizations with data isolation?
- Notification needs: email/SMS alerts on stage transitions (e.g. QC
  pending, client sign-off pending)?

## 7. Non-Functional Requirements

- Every mutating action must write an audit trail entry (no exceptions,
  including report prints and barcode reprints).
- Default device credentials (`root`/`pass`) stored encrypted at rest,
  never returned in plaintext via API/logs.
- All dynamic `meta_data` fields must be filterable/searchable via
  PostgreSQL JSONB indexing (`GIN` index) where used in reports.
- Migrations should only be needed for new core/reportable columns —
  new optional attributes go through `meta_data_field_definitions`.
- Backend must run unmodified on both Windows and Ubuntu (no
  OS-specific/cgo-only dependencies).
- Frontend must be usable on desktop, tablet, and mobile screen sizes
  without horizontal scrolling or broken layouts (see 3.8).
