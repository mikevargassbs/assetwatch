# SBS AssetWatch — How to Use the System

This guide walks you through everything you need to know to use AssetWatch, the system SBS uses to track CCTV hardware from the moment it arrives in our warehouse to the moment it's installed and signed off at a client site.

You don't need to read this front to back — jump to the section that matches what you're trying to do.

## Contents

1. [Signing In](#1-signing-in)
2. [What You'll See When You Log In](#2-what-youll-see-when-you-log-in)
3. [What Can I Do? (By Job Role)](#3-what-can-i-do-by-job-role)
4. [How a Unit Moves Through the System](#4-how-a-unit-moves-through-the-system)
5. [The Board — Your Main Screen](#5-the-board--your-main-screen)
6. [Adding a New Unit](#6-adding-a-new-unit)
7. [Filling In a Unit's Details, Step by Step](#7-filling-in-a-units-details-step-by-step)
8. [Shipping Units — Delivery Dockets](#8-shipping-units--delivery-dockets)
9. [If Something's Wrong With a Unit](#9-if-somethings-wrong-with-a-unit)
10. [Getting a Client or Manager to Sign Off Remotely](#10-getting-a-client-or-manager-to-sign-off-remotely)
11. [Reports](#11-reports)
12. [The Exceptions List](#12-the-exceptions-list)
13. [For Admins Only](#13-for-admins-only)
14. [Common Questions & Troubleshooting](#14-common-questions--troubleshooting)

---

## 1. Signing In

1. Open AssetWatch in your browser and go to the login page.
2. Enter your `@sbs.com.pg` email and your password, then click **Sign in**.
3. If your email or password isn't accepted, double check for typos. If you're sure they're correct, contact an admin — your account may not have been set up yet, or may have been deactivated.

**Forgot your password?**
1. Click **Forgot password?** on the login screen.
2. Enter your email and click **Send reset link**.
3. Check your inbox for a reset email (you'll always see the same confirmation message, whether or not an account exists — that's normal, it's just a security precaution).
4. Click the link, type your new password twice (at least 8 characters), and click **Reset password**.
5. If the page says the link is invalid or expired, go back to step 1 and request a new one — reset links only work for a limited time.

---

## 2. What You'll See When You Log In

After signing in, you'll land on the **Board** — the main working screen. Down the left side is a menu; the items you see depend on what your account is allowed to do:

- **Kanban Board** — everyone sees this. It's where you track and move units.
- **Exceptions** — everyone sees this. Units with problems (defective, damaged, wrong item) live here.
- **Logistics** — everyone sees this. Shipments to sites.
- **Reports** — everyone sees this. Printable summaries.
- **Audit History** and **Admin** — only visible if you're an administrator.

At the top of the screen you'll always see your email address and which role(s) you've been given.

---

## 3. What Can I Do? (By Job Role)

Your account can be assigned one or more roles. Whatever you've been assigned determines which buttons and forms you'll see around the app — if something described in this guide doesn't appear on your screen, it's most likely because your account doesn't have that role. If you think you're missing something you need, ask an admin.

- **Encoder** — You add new units into the system and fill out the initial configuration form ("Pre-Deployment Config"), then give it your sign-off.
- **Configurator** — You handle device configuration and firmware setup, and sign off once it's done.
- **QC (Quality Control)** — You double-check configuration and firmware work, and pass or fail it.
- **PM/PC (Project Manager / Coordinator)** — You add new units, record deliveries arriving at the warehouse, and log when units go into storage.
- **Logistics** — You build delivery dockets, dispatch shipments, mark them received, and track them in transit.
- **Field Technician** — You handle everything on-site: confirming a unit arrived correctly, installing it, taking photos, and signing off once it's mounted and working.
- **BSP Acceptance Officer** — You handle final sign-off: your own acceptance, plus collecting the client's and/or head office's signature.
- **Admin** — You can do everything above, plus manage user accounts, system settings, and site/reference data.

No matter your role, if you spot a problem with a unit — it arrived broken, damaged, or is the wrong item — you can report it. See [Section 9](#9-if-somethings-wrong-with-a-unit).

---

## 4. How a Unit Moves Through the System

Think of every camera/unit as moving through six stages, left to right, like items on a factory line:

**Pre-Deployment → Configuration → Shipment → Installation → Commissioning → Completed**

You can't skip a stage — each one has a checklist that must be finished before the unit is allowed to move to the next:

| To move from... | ...to | You need |
|---|---|---|
| Pre-Deployment | Configuration | The Encoder's sign-off |
| Configuration | Shipment | Firmware set up and QC's sign-off |
| Shipment | Installation | The shipment dispatched and marked received |
| Installation | Commissioning | Installed + Inspected + Fit & Focus sign-offs |
| Commissioning | Completed | BSP Acceptance, plus the client or head office signature |

If you try to move a unit before it's ready, the system will tell you exactly what's still missing.

Moving a unit *backward* a stage is only possible for admins, and it undoes any client/manager sign-offs already collected — so it's not something to do casually.

---

## 5. The Board — Your Main Screen

The Board shows every active unit as a card, sorted into columns by stage.

- **To move a unit forward**, drag its card into the next column. If it's not ready yet, you'll get a message explaining what's missing.
- **Switch to Table view** if you'd rather see everything as rows in a spreadsheet-style table — you can choose which columns to show (Name, Barcode, Serial, Make, Model, Site, Stage, Status, and sign-off status) and search or sort.
- **Search** for a unit by typing its name, barcode, serial number, make, or model into the search box.
- Each card shows three small dots indicating whether it's been Encoded, Configured, and QC'd — a quick visual check of progress.
- **Click any card** to open that unit's full details (see [Section 7](#7-filling-in-a-units-details-step-by-step)).

---

## 6. Adding a New Unit

*(Encoder, PM/PC, or Admin)*

1. On the Board, click **+ New Unit**.
2. Fill in:
   - **Serial Number** (required)
   - **Device Make** and **Device Model** — start typing and pick from suggestions, or type a new one
   - **Part Number**
   - **Barcode** — you can leave this blank and the system will generate one for you
   - **Allocated Branch / Site** — where this unit is destined for
3. You can leave the alias/name blank — it will be generated automatically from the site, model, and serial number.
4. Click **Create**.

Don't worry about entering network settings or credentials yet — that happens later during Pre-Deployment Configuration.

---

## 7. Filling In a Unit's Details, Step by Step

Click on a unit's card (or its barcode anywhere else in the app) to open its details page. You'll see it organized into tabs across the top — new tabs appear automatically as the unit progresses, so don't worry if you don't see every tab described below right away.

### Receiving (optional — PM/PC)

If you want to log that a batch of new stock arrived at the warehouse: enter the **PO/Waybill reference**, tick **All items correct** (or leave it unticked and explain what's wrong in the notes), then click **Record Receiving**.

### Pre-Deployment Config & QC (Encoder / Configurator / QC)

This is where the unit gets its real identity and network setup.

1. Fill in the device details: make, model, serial number, part number, alias, and which site it belongs to.
2. Fill in the network settings: device name, IP address, DNS servers, NTP server. Many of these will suggest sensible values automatically based on the site you picked — you usually just need to check they look right.
3. Fill in the default login username and password for the device itself (not your AssetWatch login).
4. If your admin has set up any extra fields for this site or device type, fill those in too.
5. Click **Save configuration**.
6. Once everything looks right, click **Encoded** to sign off that you've completed the entry (Encoder role).
7. Once encoded, a Configurator clicks **Configured**, then QC reviews it and marks **Pass** or **Fail**. A Fail sends it back for the Configurator to fix.
8. Once fully signed off, you can click **Print barcode sticker** to print a label for the physical unit.

### Firmware Configuration (Configurator / QC)

1. Tick **Firmware updated** and enter the firmware version number.
2. Click **Save firmware configuration**.
3. Configurator signs off, then QC passes or fails it, same as above.
4. If the firmware update didn't go smoothly, click **Report Update Issue** instead of signing off — this raises a defect report automatically explaining what happened.

### Logistics (view only, on the unit page)

Once a unit is added to a shipment, this tab shows you where it's headed, the waybill number, who dispatched/received it, and a running log of tracking updates. To actually manage shipments, see [Section 8](#8-shipping-units--delivery-dockets).

### Installation (Field Technician)

1. When the unit physically arrives at the site, click to confirm **Confirmed as correct on receipt** — or, if something's off, leave it unticked and describe the problem. This automatically flags it for the Project Manager/Coordinator to look into.
2. Fill in where and how it was installed: site details, the team who installed it, exact mounting location and height, and whether it's networked and reachable.
3. Take at least one photo of the installation and upload it (up to 3 photos allowed).
4. Sign off three things in order: **Installed**, **Inspected**, **Fit & Focus** (camera aimed and focused correctly). You'll need at least one photo uploaded before you can do the last two.

### Acceptance (BSP Acceptance Officer)

This is the final sign-off stage, in three parts:

1. **Your own sign-off** — enter your name, sign on the screen with your mouse/finger, add any comments, and click **Record BSP acceptance**.
2. **Client (Branch Manager) sign-off** — once you've signed off, you can either:
   - Generate a link and send it to the client yourself, or
   - Type in their email and let the system send it for you, or
   - If they've already signed something on paper, upload the scanned document instead.
3. **Head Office sign-off** (only needed if you're not using the Branch Manager option) — same choices as above.

Once your sign-off plus one of the other two is complete, the unit is ready to move to Completed.

### Attributes

A general notes/extra-info tab for anything your admin has set up as a custom field that isn't tied to one specific stage.

---

## 8. Shipping Units — Delivery Dockets

A **delivery docket** is a shipment — a group of units traveling together to one site.

*(Logistics or Admin)*

1. Go to **Logistics** in the menu and click **New Docket**.
2. Choose the destination site and submit — you'll be taken straight into the new docket.
3. **Add items**: scan or type in the barcode or serial number of each unit going in this shipment, and click **Add item**. (Only units that have reached the Shipment stage can be added.)
4. When you're ready to send it out, go to the **Sign-off** tab, fill in the waybill number, how it's being shipped, and who's dispatching it, then click **Mark Dispatched**.
5. While it's in transit, use the **Tracking History** tab to log quick updates like **In Transit** or **Arrived at Site**, or add your own custom notes.
6. When it arrives, enter who received it and click **Mark Received** — this automatically moves every unit in the shipment forward to the Installation stage.

You can print a waybill for any docket from its details page.

---

## 9. If Something's Wrong With a Unit

If a unit turns out to be defective, damaged, or the wrong item entirely, don't just leave it stuck — report it.

1. Open the unit and click **Declare Defect**.
2. Choose what kind of problem it is and describe it.
3. This automatically pulls the unit off the main Board and moves it to the [Exceptions list](#12-the-exceptions-list) until it's resolved.

From there, work through returning it to the supplier:

1. **Print a return tag** and/or **email the report to the supplier**.
2. Once you've shipped it back, click **Mark shipped back to supplier** and enter the tracking details.
3. Click **Mark delivered to supplier**, then **Mark received by supplier** once they confirm.
4. When you get a replacement unit, click **Record replacement received** and enter its serial number. This closes out the old defective unit and creates a brand new one that starts fresh from Pre-Deployment.

---

## 10. Getting a Client or Manager to Sign Off Remotely

If you sent a client or head office a signing link (from the Acceptance tab — see [Section 7](#7-filling-in-a-units-details-step-by-step)), here's what they'll experience — no account or login needed:

1. They open the link you sent them.
2. They see the unit's barcode and what they're being asked to sign off on.
3. They type their name and sign with their mouse, finger, or stylus.
4. They click **Submit Acceptance**, and see a confirmation message once it's done.

If the link has already been used or has expired, they'll see a message telling them so — in that case, generate a fresh link and send it again.

---

## 11. Reports

Go to **Reports** in the menu. You can filter by stage, site, device model, or date range, then choose one of:

- **Summary of Hardware** — a full list of units and their current status. Filter by site to get a report of everything at one location.
- **Defective / Damage / Wrong Item Report** — a list of everything currently flagged as a problem.
- **Packing List Report** — type in a site name to print a packing list for everything headed there.

Each report can be exported as a CSV file (for spreadsheets) or printed/exported as a PDF.

---

## 12. The Exceptions List

Go to **Exceptions** in the menu to see every unit currently flagged as defective, damaged, or wrong-item. These are kept separate from the main Board since they're not progressing through the normal stages. Click any unit to jump straight to its Defect tab and continue handling the return process.

---

## 13. For Admins Only

If you're an administrator, you also have access to the **Admin** panel, with six sections:

- **Users** — create accounts, tick/untick roles for each person, edit names, and deactivate accounts that shouldn't log in anymore.
- **Meta-Data Fields** — add or remove the custom extra fields that appear on forms throughout the app.
- **Site Locations** — manage the list of sites everyone picks from (name, region, network details).
- **Barcode Label** — set the physical size of the printed barcode stickers and which details print on them, with a live preview.
- **Retired Units** — units that were deleted show up here. You can restore them, or permanently delete them if they're truly not needed anymore (this can't be undone).
- **Data Management** — export a full backup of all data, or, if absolutely necessary, permanently wipe all transaction data. This is irreversible, so always export a backup first, and you'll be asked to type a confirmation phrase before it's allowed.

You also have access to the **Audit History** page, a searchable log of every action anyone has taken in the system — useful for tracking down who did what and when.

---

## 14. Common Questions & Troubleshooting

**I can't see a button or tab that this guide mentions.**
It's almost always because your account doesn't have the role needed for that action. Check [Section 3](#3-what-can-i-do-by-job-role) or ask an admin to confirm your roles.

**I tried to drag a unit to the next column and it wouldn't move.**
The system will tell you what's still missing (a sign-off, a shipment step, etc.) — check [Section 4](#4-how-a-unit-moves-through-the-system) for the full checklist per stage.

**I made a mistake and need to move a unit backward.**
Only admins can do this, and it will clear any client/manager signatures already collected on that unit, so it needs to be redone. Ask an admin if you need this done.

**A unit is broken/damaged/wrong — what do I do?**
See [Section 9](#9-if-somethings-wrong-with-a-unit).

**I deleted a unit by mistake.**
Ask an admin — deleted units aren't erased, they're "retired" and can be restored from the Admin panel's Retired Units section.

**Who do I contact if I'm stuck?**
Reach out to your system administrator.
