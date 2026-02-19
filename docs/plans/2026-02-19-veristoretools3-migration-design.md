# VeriStore Tools 3 - Migration Design Document

**Date:** 2026-02-19
**Status:** Approved
**Migration:** veristoreTools2 (PHP/Yii2) -> veristoreTools3 (Go)

---

## 1. Overview

Rebuild veristoreTools2 as an exact 1:1 feature replica in Go for significantly faster performance. Same UI (AdminLTE), same features, same database schema, same TMS API integration. No features added or removed.

### Key Decisions

| Decision | Choice |
|----------|--------|
| Feature scope | Exact 1:1 replica of veristoreTools2 |
| Backend | Go + Echo router |
| Frontend | HTMX + Templ + AdminLTE 3 + Alpine.js |
| Database | MySQL 8 (new `veristoretools3` database, migrate data from v2) |
| ORM | GORM |
| Queue | Asynq (Redis-backed) |
| RBAC | Casbin |
| Deployment | Same server as v2, systemd service |
| Project location | `/veristoretools/veristoreTools3/` |
| Process management | Systemd (single binary: web + worker + scheduler) |

---

## 2. Architecture: Modular Monolith

Single Go binary organized by domain modules.

```
veristoreTools3/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point: web server + queue worker + scheduler
│
├── internal/
│   ├── config/config.go            # App config (YAML)
│   ├── middleware/                  # auth, rbac, activitylog, recovery
│   ├── auth/                       # Login, logout, password, session, Casbin RBAC
│   ├── user/                       # User CRUD (handler, service, model, repository)
│   ├── terminal/                   # Terminal CRUD + TerminalParameter
│   ├── csi/                        # CSI Verification module
│   ├── tms/                        # TMS Profiling module + TMS API client + encryption
│   ├── sync/                       # Sync terminal + scheduler
│   ├── queue/                      # Asynq task definitions + workers
│   ├── admin/                      # Activity log, technician, FAQ, backup
│   ├── activation/                 # App activation + credential + REST API
│   └── shared/                     # database, response, pagination, excel, pdf, flash
│
├── templates/                      # Templ files (.templ)
│   ├── layouts/                    # main, login, header, sidebar, content
│   ├── site/                       # Dashboard
│   ├── user/                       # User views
│   ├── terminal/                   # Terminal views
│   ├── veristore/                  # TMS views (terminal, merchant, group, import, export)
│   ├── verification/               # CSI verification views
│   ├── sync/                       # Sync views
│   ├── admin/                      # Admin views
│   └── components/                 # Reusable: table, form, alert, modal, pagination
│
├── static/                         # Served by Echo
│   ├── adminlte/                   # AdminLTE 3 dist
│   ├── css/site.css                # Custom styles
│   ├── js/htmx.min.js             # HTMX
│   ├── js/alpine.min.js           # Alpine.js
│   ├── js/app.js                  # Custom JS (loading, confirmations)
│   └── img/                       # Logos, icons
│
├── migrations/                     # SQL migration files + runner
├── config.yaml                     # Application configuration
├── go.mod / go.sum
├── Makefile
└── README.md
```

---

## 3. Data Layer

### Database

- New database: `veristoretools3` (MySQL 8)
- Same schema as v2 (identical column names for easy data migration)
- 20 tables: user, terminal, terminal_parameter, template_parameter, verification_report, sync_terminal, tms_login, tms_report, app_activation, app_credential, activity_log, technician, queue_log, export, export_result, tid_note, session, hash, auth_item, auth_assignment, auth_rule

### GORM Models

Each module defines its own models with `gorm:"column:..."` tags matching v2 column names exactly.

### Repository Pattern

Each module has a repository struct wrapping `*gorm.DB` with typed query methods:
- `FindByID`, `FindByCSI`, `Search(params) ([]T, int64, error)`, `Create`, `Update`, `Delete`

### Migration Strategy

1. Create `veristoretools3` database
2. Run Go migration files (identical schema)
3. `mysqldump` from `veristoretools2` -> import into `veristoretools3`
4. Verify row counts match

---

## 4. Routing & Handlers

### 130+ Endpoints

All v2 routes mapped 1:1 to Echo handlers:

- **Public:** `/user/login`, `/user/logout`
- **Protected (session auth):** all web routes via `middleware.RequireAuth()`
- **API (basic auth):** `/feature/api/activation-code` via `middleware.BasicAuth()`

### Handler Pattern

Handlers detect HTMX requests (`HX-Request` header) to return either:
- Full page HTML (normal navigation)
- Partial HTML fragment (HTMX swap for search/pagination/forms)

### Route Groups

- `/` - Site (dashboard)
- `/user/*` - User management
- `/terminal/*` - Local terminal CRUD
- `/veristore/*` - TMS integration (30+ routes)
- `/verification/*` - CSI verification
- `/sync-terminal/*` - Data sync
- `/activitylog/*` - Activity logs
- `/technician/*` - Technician CRUD
- `/templateparameter/*` - Template parameter CRUD
- `/appactivation/*` - App activation CRUD
- `/appcredential/*` - App credential CRUD
- `/scheduler/*` - Scheduler config
- `/faq/*` - FAQ
- `/backup/*` - Backup
- `/feature/api/*` - REST API

---

## 5. Authentication & Security

### Authentication

- Session-based (gorilla/sessions with MySQL backend)
- Session name: `_veristore_tools_3_app`
- Session timeout: 900 seconds (15 minutes)
- Password: SHA256 + salt (`@!Boteng2021%??`) for v2 data compatibility; bcrypt for new users

### RBAC (Casbin)

5 roles (same as v2):
- CSI ADMIN, CSI OPERATOR
- TMS ADMIN, TMS SUPERVISOR, TMS OPERATOR

Policies loaded from database (migrated from v2 auth_item/auth_assignment tables).

### Encryption (Backward Compatible)

| Function | Algorithm | Same as v2 |
|----------|-----------|-----------|
| TMS API auth | HMAC-SHA256 | Yes |
| TMS session | AES-256-CBC | Yes, same keys |
| Activation codes | Triple DES (ECB) | Yes, same algorithm |
| Password storage | SHA256 + salt | Yes, same salt |
| API auth | HTTP Basic Auth | Yes, same credentials |

### Middleware Chain

Web: `Recovery -> Logger -> SessionAuth -> RBAC -> Handler`
API: `Recovery -> CORS -> BasicAuth -> Handler`

---

## 6. Queue System

### Asynq (Redis-backed)

Replaces Yii2 DB Queue. Runs in same binary as web server.

### Tasks

| v2 Component | v3 Asynq Task | Retry | Timeout |
|---|---|---|---|
| ImportTerminal | `import:terminal` | 3 | 1 hour |
| ExportTerminal | `export:terminal` | 3 | 1 hour |
| ImportMerchant | `import:merchant` | 3 | 1 hour |
| SyncTerminalParameter | `sync:parameter` | 3 | 1 hour |
| ExportAllTerminalsJob | `export:all_terminals` | 3 | 1 hour |

### Scheduler (Built-in)

Replaces v2 console commands:
- Every 15 min: `tms:ping` (check TMS token validity)
- Every 1 min: `tms:scheduler_check` (check and execute scheduled syncs)

### Progress Tracking

Same `queue_log` table for job progress monitoring in UI.

---

## 7. Frontend

### Stack

- **Templ:** Type-safe Go HTML templates (replaces Yii2 PHP views)
- **HTMX:** Partial page updates (replaces Yii2 Pjax) - 14KB
- **Alpine.js:** Lightweight interactivity for dropdowns/modals (replaces jQuery) - 15KB
- **AdminLTE 3:** Same dashboard theme as v2
- **Bootstrap 4:** Same grid/components
- **FontAwesome:** Same icons

### Widget Mapping

| v2 Widget | v3 Replacement |
|---|---|
| GridView | Templ table + HTMX pagination/sort |
| ActiveForm | `<form>` + `hx-post` |
| Pjax | `hx-target` + `hx-swap` |
| Select2 | Select2 JS (standalone) |
| DatePicker | Flatpickr |
| Kartik Dialog | SweetAlert2 |
| Kartik Spinner | CSS spinner + Alpine.js |
| Modal | Bootstrap modal + Alpine.js |
| Alert | Templ alert component |
| Breadcrumbs | Templ breadcrumb component |
| LinkPager | Templ pagination component |

### Template Structure

Layouts: main (authenticated), login, header, sidebar, content
Components: reusable table, form, alert, modal, pagination templates
Views: one directory per module matching v2 structure

---

## 8. TMS API Integration

### TMS Client

Go HTTP client (`internal/tms/client.go`) replacing TmsHelper.php:

- `Login()`, `GetVerifyCode()`, `GetResellerList()`
- `GetTerminalList()`, `GetTerminalDetail()`, `AddTerminal()`, `EditTerminal()`, `CopyTerminal()`, `DeleteTerminal()`, `ReplaceTerminal()`
- `GetTerminalParameter()`, `SyncTerminalParameters()`
- `GetMerchantList()`, `AddMerchant()`, `EditMerchant()`, `DeleteMerchant()`
- `GetGroupList()`, `AddGroup()`, `EditGroup()`, `DeleteGroup()`

### Token Renewal

Same pattern: detect "toke更新" in response -> re-login -> retry request.

### External Systems

- TMS API: `http://172.17.30.158:8280` (veristore.net)
- TPS API: `tps.veristore.net`

---

## 9. Testing

- **Unit tests:** testify + gomock per module
- **HTTP tests:** Echo test utilities + httptest
- **Crypto tests:** Byte-for-byte comparison with v2 PHP output
- **Integration tests:** Separate `veristoretools3_test` database
- **Tools:** `go test ./...`, golangci-lint

---

## 10. Deployment

### Single Binary

`go build -o bin/veristoretools3 ./cmd/server`

Runs: web server (port 8080) + queue worker + scheduler

### Systemd Service

```ini
[Unit]
Description=VeriStore Tools 3
After=network.target mysql.service redis.service

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/veristoretools3
ExecStart=/opt/veristoretools3/bin/veristoretools3
Restart=always
RestartSec=5
Environment=CONFIG_PATH=/opt/veristoretools3/config.yaml

[Install]
WantedBy=multi-user.target
```

### Configuration

All config in `config.yaml`: app settings, database, Redis, TMS API, API credentials, import/export limits.

---

## 11. Go Dependencies

```
github.com/labstack/echo/v4          # HTTP router
github.com/a-h/templ                 # HTML templates
github.com/gorilla/sessions          # Session management
gorm.io/gorm                         # ORM
gorm.io/driver/mysql                 # MySQL driver
github.com/hibiken/asynq             # Task queue (Redis)
github.com/casbin/casbin/v2          # RBAC
github.com/xuri/excelize/v2          # Excel read/write
github.com/stretchr/testify          # Testing
github.com/rs/zerolog                # Structured logging
gopkg.in/yaml.v3                     # Config parsing
github.com/go-playground/validator   # Struct validation
```
