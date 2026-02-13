# Plan: Admin-UI + ODS-Import

## 1. Admin-UI (index.html)

### CSS
- `.admin-btn` Style im Header (Icon-Button, nur sichtbar für Admins)
- `.modal-overlay` + `.modal-box` Styles (wiederverwendbar, ähnlich login-overlay)
- `.user-table` für die Benutzerliste
- `.form-row` für das Anlegen-Formular
- Responsive Anpassungen

### HTML
- Admin-Button im Header (neben Logout)
- Admin-Modal mit:
  - Benutzerliste (Tabelle: Username, Name, Bundesland, Admin-Badge, Aktionen)
  - "Neuer Benutzer"-Formular (aufklappbar oder inline)
  - Löschen-Button (mit confirm()-Dialog, eigener Account geschützt)
  - Passwort-Reset-Button (prompt() für neues Passwort)

### JavaScript
- `openAdminModal()` — lädt `/api/admin/users`, rendert Liste
- `createUser(form)` — POST `/api/admin/users`
- `deleteUser(id)` — DELETE `/api/admin/users/{id}` mit confirm()
- `resetPassword(id)` — PUT `/api/admin/users/{id}/password` mit prompt()
- Admin-Button nur anzeigen wenn `currentUser.isAdmin`

## 2. ODS-Import (index.html)

### Ansatz: Client-seitig
- ODS = ZIP-Datei → JSZip aus CDN laden (https://cdnjs.cloudflare.com/ajax/libs/jszip/3.10.1/jszip.min.js)
- `content.xml` aus dem ZIP extrahieren → DOMParser parsen
- Sheets erkennen (table:table name = Jahreszahl)
- Zellen auslesen: Spaltenstruktur `1 + m*2` = Tag-Label, `2 + m*2` = Abwesenheitstyp
- Wochenenden und ganztägige Feiertage herausfiltern (mit existierendem `getHolidays()`)
- Quotas aus Sheet extrahieren oder manuell eingeben lassen

### UI
- "ODS importieren"-Button im Header (oder als Aktion im Admin-Bereich, je nach Kontext)
- Import-Modal:
  - File-Input für .ods
  - Nach Auswahl: Vorschau (gefundene Jahre, Anzahl Einträge pro Jahr)
  - Import-Button → sendet via PUT /api/absences + PUT /api/quotas pro Jahr
  - Fortschrittsanzeige

### JavaScript
- `handleOdsImport(file)` — liest File, entpackt ZIP, parst XML
- `parseOdsContent(xml)` — extrahiert Abwesenheiten und Quotas
- `importData(parsed)` — sendet an API und aktualisiert lokalen State

## 3. Backend (server.go)
- `seedImportedData()` bereits entfernt ✓
- Keine weiteren Backend-Änderungen nötig (bestehende API reicht)

## Reihenfolge
1. Admin-UI (CSS + HTML + JS)
2. ODS-Import (JSZip CDN + Import-Modal + Parser)
3. Rebuild + Test
