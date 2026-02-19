# VeriStore Tools 3

VeriStore Tools 3 is a full rewrite of the veristoreTools2 PHP/Yii2 application in Go.
It provides terminal management, TMS integration, CSI verification, sync operations,
activation code generation, and administrative tools for Verifone field operations.

## Tech Stack

| Layer           | Technology                                       |
|-----------------|--------------------------------------------------|
| Language        | Go 1.22+                                         |
| Web Framework   | Echo v4                                          |
| Templates       | templ (type-safe Go templates)                   |
| Database        | MySQL 8 via GORM                                 |
| Session Store   | gorilla/sessions (cookie-based)                  |
| Authorization   | Casbin (RBAC with model/policy files)            |
| Background Jobs | Asynq (Redis-backed task queue and scheduler)    |
| Logging         | zerolog                                          |
| Frontend        | AdminLTE 3, HTMX, Alpine.js                     |
| Crypto          | AES-256-CBC, Triple DES ECB, SHA-256 (PHP-compat)|

## Prerequisites

- **Go** 1.22 or later
- **MySQL** 8.0 or later
- **Redis** 6.0 or later
- **templ** CLI (`go install github.com/a-h/templ/cmd/templ@latest`)

## Quick Start

```bash
# 1. Clone the repository
git clone <repo-url> && cd veristoreTools3

# 2. Copy and edit configuration
cp config.yaml config.yaml   # Edit database, Redis, and API credentials

# 3. Create the database
mysql -u root -e "CREATE DATABASE IF NOT EXISTS veristoretools3 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# 4. Run database migrations
go run ./cmd/migrate

# 5. Generate templ templates
templ generate

# 6. Build and run
go build -o bin/veristoretools3 ./cmd/server
./bin/veristoretools3
```

The application starts on `http://localhost:8080` by default.

## Build Commands

All build targets are defined in the `Makefile`:

```bash
make build       # Build server and migrate binaries to bin/
make run         # Generate templates and run the server
make dev         # Same as 'make run' (development mode)
make test        # Run all tests with verbose output
make templ       # Generate templ templates
make lint        # Run golangci-lint
make migrate     # Run database migrations
make clean       # Remove build artifacts
make verify      # Run data migration verification (v2 vs v3 row counts)
```

## Configuration

All configuration is in `config.yaml`:

```yaml
app:
  name: "VeriStore Tools 3"
  version: "3.0.0"
  port: 8080
  debug: true
  session_timeout: 900          # seconds
  session_secret: "change-me"   # 32+ characters recommended
  session_name: "_veristore_tools_3_app"
  password_salt: "your-salt"    # Must match v2 for password compatibility

database:
  host: localhost
  port: 3306
  name: veristoretools3
  user: root
  password: ""
  charset: utf8mb4
  max_open_conns: 25
  max_idle_conns: 10

redis:
  addr: localhost:6379
  password: ""
  db: 0

tms:
  base_url: "http://tms-host:8280"
  secret_key: "..."
  secret_iv: "..."

api:
  basic_auth_user: "..."
  basic_auth_pass: "..."

import:
  max_file_size: 5242880
  batch_size: 100

export:
  batch_size: 100
  output_dir: "static/export"

country_id: 5
package_name: "com.vfi.android.payment.cimb"
```

## Data Migration from v2

To migrate data from the existing veristoretools2 MySQL database:

```bash
# 1. Ensure v3 schema is applied
go run ./cmd/migrate

# 2. Run the migration script
./scripts/migrate_data.sh [mysql_user] [mysql_password]

# 3. Or verify manually
go run ./cmd/migrate verify
```

The verify command compares row counts across all 18 tables between the v2
and v3 databases and reports any mismatches.

## Deployment (systemd)

```bash
# Build and install
sudo ./deploy/install.sh

# Manual service management
sudo systemctl status veristoretools3
sudo systemctl restart veristoretools3
sudo journalctl -u veristoretools3 -f
```

The systemd unit file is at `deploy/veristoretools3.service`.
The application is installed to `/opt/veristoretools3/`.

## Development

### Generate Templates

The project uses [templ](https://templ.guide/) for type-safe HTML templates.
After modifying any `.templ` file, regenerate the Go code:

```bash
templ generate
# or
make templ
```

### Run Tests

```bash
go test ./...
# or
make test
```

### Lint

```bash
golangci-lint run ./...
# or
make lint
```

### API Endpoint

The activation code REST API is available at:

```
POST /feature/api/activation-code
Authorization: Basic <base64(user:pass)>
Content-Type: application/json

{
  "csi": "12345678",
  "tid": "TID001",
  "mid": "MID001",
  "model": "X990",
  "version": "1.0.0"
}
```

## Project Structure

```
veristoreTools3/
├── cmd/
│   ├── server/              # Main application entry point
│   │   └── main.go
│   └── migrate/             # Database migration and verification tool
│       ├── main.go
│       └── verify.go
├── config.yaml              # Application configuration
├── deploy/
│   ├── install.sh           # Production installation script
│   └── veristoretools3.service  # systemd unit file
├── internal/
│   ├── activation/          # App activation & credential CRUD + REST API
│   ├── admin/               # Activity log, technician, template params, FAQ, backup
│   ├── auth/                # Authentication (login/logout, session, Casbin RBAC)
│   ├── config/              # YAML configuration loader
│   ├── csi/                 # CSI verification and reporting
│   ├── middleware/          # Echo middleware (session auth, RBAC, basic auth, logging)
│   ├── queue/               # Asynq background jobs (sync, import, export, TMS ping)
│   ├── shared/              # Shared utilities (crypto, DB, flash, pagination, response)
│   ├── site/                # Dashboard handler
│   ├── sync/                # Sync terminal operations and scheduler
│   ├── terminal/            # Local terminal CRUD and parameter management
│   ├── tms/                 # TMS API client, encryption, Veristore handlers (30+)
│   └── user/                # User management CRUD
├── migrations/              # SQL migration files (001-018)
├── scripts/
│   └── migrate_data.sh      # Data migration script (v2 to v3)
├── static/                  # Static assets (AdminLTE, CSS, JS, images)
├── templates/               # templ templates organized by module
│   ├── admin/
│   ├── appactivation/
│   ├── appcredential/
│   ├── components/          # Shared UI components (alerts, pagination, tables)
│   ├── layouts/             # Base layout, header, sidebar, login layout
│   ├── scheduler/
│   ├── site/
│   ├── sync/
│   ├── templateparameter/
│   ├── terminal/
│   ├── terminalparameter/
│   ├── tmslogin/
│   ├── user/
│   ├── verification/
│   ├── verificationreport/
│   └── veristore/
├── Makefile
├── go.mod
├── go.sum
└── tools.go                 # Tool dependencies (templ)
```

## Cryptographic Compatibility

VeriStore Tools 3 maintains full cryptographic compatibility with v2:

- **AES-256-CBC** encryption for TMS session data uses the same key derivation
  (SHA-256 of secret key/IV) and double base64 encoding as PHP `openssl_encrypt`.
- **Triple DES ECB** activation code generation produces identical 6-character
  hex codes as the PHP `calcPassword()` function.
- **SHA-256 password hashing** uses the same `password + salt` concatenation as
  PHP `hash('sha256', ...)`.

## License

Proprietary - Verifone, Inc. All rights reserved.
