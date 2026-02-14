# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Urlaubsplaner — a German vacation planning web application. Go backend with vanilla JS/HTML/CSS frontend, SQLite database, deployed via Docker.

## Build & Run

```bash
# Build and run with Docker (primary method)
docker compose up --build

# Build Go binary directly (requires CGO for SQLite)
CGO_ENABLED=1 go build -o urlaubsplaner server.go

# Run locally
URLAUBSPLANER_ADMIN_USER=admin URLAUBSPLANER_ADMIN_PASS=changeme ./urlaubsplaner
```

No test suite, linter, or CI pipeline exists yet.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `URLAUBSPLANER_PORT` | `8080` | HTTP server port |
| `URLAUBSPLANER_DB_PATH` | `./data/urlaubsplaner.db` | SQLite database path |
| `URLAUBSPLANER_ADMIN_USER` | `admin` | Initial admin username |
| `URLAUBSPLANER_ADMIN_PASS` | `changeme` | Initial admin password |

## Architecture

**Two-file monolith:** `server.go` (backend) and `index.html` (frontend SPA). No subdirectories or modules.

### Backend (`server.go`)

Single-file Go HTTP server using `net/http.ServeMux`. Key sections:

- **Database** — SQLite with WAL mode, auto-creates tables (`users`, `quotas`, `absences`) on startup. Foreign keys enabled. Parameterized queries throughout.
- **Sessions** — In-memory `map[string]*Session` protected by `sync.RWMutex`. 32-byte random hex tokens stored in HttpOnly/SameSite=Strict cookies.
- **Auth middleware** — `requireAuth()` and `requireAdmin()` are higher-order functions wrapping handlers: `requireAdmin(requireAuth(handler))`.
- **JSON helpers** — `readJSON()`, `jsonResponse()`, `jsonError()` provide consistent request/response handling.
- **API routes:**
  - `/api/login`, `/api/logout`, `/api/me` — authentication
  - `/api/absences` — GET/PUT/DELETE vacation entries (bulk operations use transactions)
  - `/api/quotas`, `/api/quotas/{year}` — per-year vacation allowance
  - `/api/settings` — user profile (state, defaultQuota, displayName)
  - `/api/admin/users` — CRUD users, password reset (admin-only)
  - `/` — serves `index.html`

### Frontend (`index.html`)

Single-page app with embedded CSS (~930 lines) and JavaScript (~1080 lines). Dark theme. No framework.

- **Calendar grid** — 12 month columns, each day is clickable. Supports single-click popover edit and shift-click range selection.
- **Holiday calculation** — Full German holiday logic for all 16 Bundesländer, including Easter-based moveable holidays (Meeus algorithm) and state-specific rules.
- **Absence types** — `UR` (full day), `UR/2` (half day), `SUR` (special leave), `UUR` (unpaid leave).
- **ODS import** — Client-side parsing using JSZip (CDN). Expects sheets named by year, month-pair columns with day labels and type values.
- **State management** — Global JS variables (`absences`, `quotas`, `selection`, `currentUser`). Local state updated immediately, then synced to server via async API calls.
- **`api(method, path, body)`** — Central fetch wrapper handling JSON serialization and 401 auto-redirect to login.

### Data Model

- **users** — id, username, password_hash (bcrypt), display_name, state (Bundesland), default_quota, is_admin
- **quotas** — user_id, year, quota (year-specific vacation allowance)
- **absences** — user_id, date (YYYY-MM-DD), type (UR/UR\/2/SUR/UUR)

## Key Conventions

- All dates use `YYYY-MM-DD` string format
- UI language is German
- Passwords hashed with bcrypt (default cost)
- Absence type validation uses whitelist: `["UR", "UR/2", "SUR", "UUR"]`
- Go module name is `urlaubsplaner`
