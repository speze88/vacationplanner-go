# Urlaubsplaner

Webbasierte Urlaubsplanung mit interaktivem Jahreskalender, automatischer Feiertagserkennung und Mehrbenutzersupport.

## Features

- **Jahreskalender** mit 12-Monats-Übersicht und Tagesansicht
- **Automatische Feiertagserkennung** für alle 16 Bundesländer (inkl. bewegliche Feiertage)
- **Abwesenheitstypen**: Urlaub (UR), halber Tag (UR/2), Sonderurlaub (SUR), unbezahlter Urlaub (UUR)
- **Urlaubskontingent** pro Jahr konfigurierbar, automatische Restberechnung
- **Halbe Feiertage** (z.B. Silvester) werden bei der Kontingentberechnung berücksichtigt
- **Multi-Select**: Mehrere Tage per Shift+Klick markieren und gleichzeitig bearbeiten
- **ODS-Import**: Bestehende Daten aus LibreOffice Calc importieren
- **Mehrbenutzersupport** mit Admin-Benutzerverwaltung
- **Dark Theme**, responsive Design
- **Docker-Deployment** mit SQLite-Datenbank

## Schnellstart

### Voraussetzungen

- Docker / Podman mit Compose

### Starten

```bash
git clone <repo-url> && cd urlaubsplaner
mkdir -p data
docker compose up --build
```

Die App ist unter **http://localhost:8080** erreichbar.

### Erster Login

Standardmäßig wird ein Admin-Account angelegt:

| Feld | Wert |
|------|------|
| Benutzer | `admin` |
| Passwort | `changeme` |

Die Zugangsdaten können in `docker-compose.yml` angepasst werden:

```yaml
environment:
  - URLAUBSPLANER_ADMIN_USER=admin
  - URLAUBSPLANER_ADMIN_PASS=mein-sicheres-passwort
```

## Anleitung

### Abwesenheiten eintragen

1. Auf einen Tag klicken — ein Popover zeigt die verfügbaren Abwesenheitstypen
2. Typ auswählen: Urlaub, halber Tag, Sonderurlaub oder unbezahlter Urlaub
3. Zum Entfernen: nochmal klicken und "Entfernen" wählen

### Mehrere Tage bearbeiten

1. **Shift+Klick** auf einen Tag startet die Mehrfachauswahl
2. **Shift+Klick** auf einen zweiten Tag wählt den gesamten Bereich aus (Wochenenden/Feiertage werden übersprungen)
3. Einzelne Tage per Klick zur Auswahl hinzufügen/entfernen
4. Über die Aktionsleiste am unteren Rand den gewünschten Typ zuweisen oder entfernen

### Kontingent anpassen

Im Header das Feld "Kontingent (Tage)" ändern. Das Kontingent wird pro Jahr gespeichert.

### Bundesland ändern

Im Header das gewünschte Bundesland auswählen — die Feiertage werden automatisch angepasst.

### ODS-Import

1. Im Header auf **"ODS Import"** klicken
2. Die `.ods`-Datei per Drag & Drop oder Klick auswählen
3. Die erkannten Daten werden als Vorschau angezeigt (Jahre, Anzahl Einträge, Typen)
4. Mit **"Importieren"** bestätigen

Erwartetes ODS-Format: Ein Sheet pro Jahr (Name = Jahreszahl), 12 Monate in Spaltenpaaren (Tag-Label + Abwesenheitstyp).

### Benutzerverwaltung (nur Admin)

1. Im Header auf **"Benutzer"** klicken
2. **Neuer Benutzer**: Formular aufklappen, Daten eingeben, "Anlegen"
3. **Passwort ändern**: "Passwort"-Button neben dem Benutzer
4. **Benutzer löschen**: "Löschen"-Button (alle Daten des Benutzers werden mitgelöscht)

## Konfiguration

Umgebungsvariablen:

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `URLAUBSPLANER_PORT` | `8080` | Server-Port |
| `URLAUBSPLANER_DB_PATH` | `urlaubsplaner.db` | Pfad zur SQLite-Datenbank |
| `URLAUBSPLANER_ADMIN_USER` | `admin` | Admin-Benutzername (nur bei Erststart) |
| `URLAUBSPLANER_ADMIN_PASS` | `changeme` | Admin-Passwort (nur bei Erststart) |

## Techstack

- **Backend**: Go, SQLite, bcrypt-Auth, Session-Cookies
- **Frontend**: Vanilla HTML/CSS/JavaScript (Single Page App, kein Framework)
- **Deployment**: Docker (Multi-Stage Build)

## Projektstruktur

```
├── server.go           # Go-Backend (API, Auth, DB)
├── index.html          # Frontend (UI, Kalenderlogik, Feiertagsberechnung)
├── go.mod / go.sum     # Go-Abhängigkeiten
├── Dockerfile          # Multi-Stage Docker Build
├── docker-compose.yml  # Compose-Konfiguration
└── data/               # SQLite-Datenbank (Volume)
```
