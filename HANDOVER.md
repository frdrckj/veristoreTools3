# VeriStore Tools 3 - Handover Document

**Version:** 3.0.0
**Last Updated:** 2026-04-02
**Author:** Frederick A. Jerusha

---

## 1. Architecture

### Tech Stack

| Component | Technology | Version |
|---|---|---|
| Language | Go | 1.23.4 |
| Web Framework | Echo | v4.13.3 |
| Template Engine | Templ | v0.3.977 |
| Database | MySQL | 8.0 |
| ORM | GORM | v1.31.1 |
| Task Queue | Asynq (Redis-backed) | v0.25.1 |
| Cache/Queue Backend | Redis | 7 (Alpine) |
| Session Store | Gorilla Sessions (Cookie) | v1.4.0 |
| RBAC | Casbin | v2.135.0 |
| Logging | Zerolog | v1.34.0 |
| Excel Processing | Excelize | v2.9.0 |
| CSS/UI | AdminLTE 3 + Bootstrap 4 | - |
| JS Libraries | jQuery, Select2, DataTables | - |

### Project Structure

```
veristoreTools3/
├── cmd/
│   ├── server/             # Main application entry point
│   └── migrate/            # Database migration tool
├── internal/
│   ├── activation/         # App activation management
│   ├── admin/              # Activity log, technician, template params, backup, FAQ
│   ├── auth/               # Login, Casbin RBAC model & policy
│   ├── config/             # YAML config loader
│   ├── csi/                # CSI verification (Verifikasi CSI)
│   ├── middleware/          # Session auth, activity log, RBAC middleware
│   ├── queue/              # Background jobs (import, export, sync, reports)
│   ├── shared/             # Flash messages, render helpers, pagination
│   ├── site/               # Dashboard
│   ├── sync/               # Sync terminal & scheduler
│   ├── terminal/           # Local terminal & parameter CRUD
│   ├── tms/                # TMS API client, handlers, encryption
│   └── user/               # User CRUD, login, password change
├── templates/              # Templ (.templ) template files
│   ├── layouts/            # Base layout, sidebar, page data
│   ├── verification/       # Verifikasi CSI
│   ├── verificationreport/ # Laporan Verifikasi
│   ├── veristore/          # TMS terminal, merchant, group
│   ├── admin/              # Activity log, technician, FAQ, backup
│   ├── site/               # Dashboard
│   └── user/               # Login, user management
├── migrations/             # SQL migration files (001-025)
├── scripts/
│   └── migrate_data.sh     # Data migration from v2 to v3
├── static/                 # CSS, JS, images, AdminLTE assets
│   ├── export/             # Generated export Excel files
│   └── import/             # Uploaded import files
├── deploy-package/         # Production deployment package
├── config.yaml             # Local development config
├── config.docker.yaml      # Docker/production config
├── docker-compose.yml      # Development docker compose
├── docker-compose.prod.yml # Production docker compose
├── Dockerfile              # Multi-stage build (builder + alpine runtime)
├── Makefile                # Build targets
└── go.mod / go.sum
```

### Request Flow

```
Browser Request
    → Echo Router
    → Middleware Chain (Recovery → Logger → ActivityLog → SessionAuth → RBAC)
    → Handler (internal/{module}/handler.go)
    → Service Layer (business logic)
    → Repository Layer (GORM database queries)
    → Templ Template Rendering
    → HTML Response
```

### Background Job Flow

```
Handler enqueues task → Redis (Asynq) → Worker picks up task → Handler processes
                                                              → Logs to queue_log table
                                                              → Writes result file/DB
```

### TMS API Integration

The application communicates with the Veristore TMS for terminal, merchant, and group operations.

- **TMS Web:** https://app.veristore.net / http://10.90.30.158:8280
- **TMS API:** https://tps.veristore.net
- **Encryption:** AES with configured secret_key and secret_iv
- **Authentication:** access_key + access_secret
- **TLS:** Verification skipped (self-signed cert)

### Database Schema

**Engine:** MySQL 8.0, charset utf8mb4

**Tables (24 total):**

| Table | Purpose |
|---|---|
| user | Application users and credentials |
| terminal | Local terminal records (synced from TMS) |
| terminal_parameter | Terminal parameters (host, merchant, TID, MID) |
| sync_terminal | Sync job records |
| tms_login | TMS operator login sessions |
| tms_report | TMS generated reports (longblob file storage) |
| app_activation | App activation records |
| activity_log | User activity audit trail |
| verification_report | CSI verification reports |
| app_credential | App credentials |
| technician | Technician records |
| template_parameter | Parameter templates |
| export | Export jobs (longblob for Excel file storage) |
| export_result | Export result metadata |
| tid_note | TID notes |
| queue_log | Background job execution log |
| session | Session data (legacy, not actively used) |
| hash | Change-tracking hashes |
| queue | Queue job records |
| casbin_rule | RBAC policies (Casbin auto-managed) |
| faq | FAQ content |
| import | Import job records |

**Important field naming quirk** (inherited from v2):

| Column Name | Actual Meaning |
|---|---|
| `vfi_rpt_term_serial_num` | **CSI** (not serial number) |
| `vfi_rpt_term_device_id` | **Serial Number** (not device ID) |

### Authentication & Authorization

- **Sessions:** Cookie-based via `gorilla/sessions`, 15-minute inactivity timeout
- **Password:** Hashed with configurable salt
- **API Auth:** Basic HTTP Auth for `/feature/api/*` endpoints

---

## 2. Configuration

Configuration is loaded from `config.yaml` (local dev) or `config.docker.yaml` (Docker/production).

### Settings

| Setting | Dev Value | Docker/Prod Value |
|---|---|---|
| Port | 8080 | 8080 |
| Debug | true | **false** |
| Session Timeout | 900s (15 min) | 900s |
| DB Host | localhost | mysql (container name) |
| DB Port | 3306 | 3306 |
| DB Name | veristoretools3 | veristoretools3 |
| DB Max Open Conns | 25 | 25 |
| DB Max Idle Conns | 10 | 10 |
| Redis Address | localhost:6379 | redis:6379 |
| TMS Base URL | https://app.veristore.net | http://10.90.30.158:8280 |
| TMS API URL | https://tps.veristore.net | https://tps.veristore.net |
| Import Max File Size | 5MB | 5MB |
| Export Output Dir | static/export | static/export |

### Sensitive Values

These values are in the config file and should be secured in production:

| Key | Purpose |
|---|---|
| `session_secret` | Session encryption key |
| `password_salt` | User password hashing salt |
| `tms.secret_key` | TMS API AES encryption key |
| `tms.secret_iv` | TMS API AES initialization vector |
| `tms.access_key` | TMS API authentication key |
| `tms.access_secret` | TMS API authentication secret |
| `api.basic_auth_user` | External API basic auth username |
| `api.basic_auth_pass` | External API basic auth password |

---

## 3. Build & Deploy

The app is built on a Mac or Windows PC and deployed as Docker images to the Linux server. The server does **not** need internet access, Go, or the source code.

### Prerequisites

| Machine | Requirements |
|---------|-------------|
| Mac (build) | Docker Desktop |
| Windows (build) | Docker Desktop for Windows (with WSL2 backend) |
| Server (production) | Docker Engine + Docker Compose |

### First-Time Deployment

**Step 1: Build & Package**

#### Building on Mac

```bash
cd veristoreTools3
./deploy.sh
```

This creates a `deploy-package/` folder:

```
deploy-package/
├── veristoretools3-images.tar.gz   (~600 MB) All Docker images
├── docker-compose.yml              Production compose file
├── config.docker.yaml              App configuration
└── setup.sh                        One-command server setup
```

#### Building on Windows

The `deploy.sh` script does not run natively on Windows. Use PowerShell (or Git Bash/WSL as an alternative) to run the equivalent Docker commands manually:

```powershell
cd veristoreTools3

# Build the app image for Linux
docker build --platform linux/amd64 -t veristoretools3-app:latest .

# Pull base images for Linux
docker pull --platform linux/amd64 mysql:8.0
docker pull --platform linux/amd64 redis:7-alpine

# Save all images to a single tar file
docker save veristoretools3-app:latest mysql:8.0 redis:7-alpine -o veristoretools3-images.tar

# Create the deploy-package folder
mkdir -p deploy-package

# Compress the images (using tar if available, or 7-Zip)
tar -czf deploy-package\veristoretools3-images.tar.gz -C . veristoretools3-images.tar
# Alternative with 7-Zip:
# & "C:\Program Files\7-Zip\7z.exe" a -tgzip deploy-package\veristoretools3-images.tar.gz veristoretools3-images.tar

# Copy config and compose files into the deploy package
Copy-Item docker-compose.prod.yml deploy-package\docker-compose.yml
Copy-Item config.docker.yaml deploy-package\config.docker.yaml

# Clean up the uncompressed tar
Remove-Item veristoretools3-images.tar
```

This produces the same `deploy-package/` folder as the Mac script.

> **Tip:** If you have Git Bash or WSL installed, you can run `./deploy.sh` directly from those shells instead.

**Step 2: Transfer to Server**

Copy the folder to the server via USB, SCP, or any method available:

```bash
# Mac / Linux
scp -r deploy-package/ user@server:/opt/veristoretools3/
```

```powershell
# Windows (PowerShell) — rsync is not available by default, use scp instead
scp -r deploy-package/ user@server:/opt/veristoretools3/
```

**Step 3: Configure (on Server)**

Edit `config.docker.yaml` with production values:

```bash
cd /opt/veristoretools3
nano config.docker.yaml
```

Key settings to update:

```yaml
app:
  session_secret: "USE-A-STRONG-RANDOM-STRING"

database:
  host: mysql
  password: "CHANGE-ME"      # must match MYSQL_ROOT_PASSWORD in docker-compose.yml

tms:
  base_url: "http://10.90.30.158:8280"
  api_base_url: "https://tps.veristore.net"
  access_key: "your-key"
  access_secret: "your-secret"
```

If you change the MySQL password, also update it in `docker-compose.yml`:

```yaml
mysql:
  environment:
    MYSQL_ROOT_PASSWORD: CHANGE-ME
```

**Step 4: Start (on Server)**

```bash
./setup.sh
```

The app will be available at `http://server-ip:8080`.

**Step 5: Migrate Data from V2**

```bash
# On V2 server: export the database
mysqldump -u root -p veristoretools2 > v2_backup.sql

# Transfer v2_backup.sql to V3 server, then:
docker compose exec -T mysql mysql -u root -p veristoretools3 < v2_backup.sql

# Run V3 migrations to add new columns/tables
docker compose exec app ./migrate
```

### Updating the App

When you make code changes and need to deploy an update:

#### Updating from Mac

```bash
cd veristoreTools3

# Rebuild just the app image (skip MySQL/Redis — they never change)
docker build --platform linux/amd64 -t veristoretools3-app:latest .

# Save just the app image (~20 MB vs ~600 MB for full package)
docker save veristoretools3-app:latest -o app-update.tar
gzip app-update.tar

# Transfer to server
rsync -avz --progress app-update.tar.gz user@server:/opt/veristoretools3/
```

#### Updating from Windows

```powershell
cd veristoreTools3

# Rebuild just the app image
docker build --platform linux/amd64 -t veristoretools3-app:latest .

# Save just the app image
docker save veristoretools3-app:latest -o app-update.tar

# Compress (using tar if available, or 7-Zip)
tar -czf app-update.tar.gz -C . app-update.tar
# Alternative with 7-Zip:
# & "C:\Program Files\7-Zip\7z.exe" a -tgzip app-update.tar.gz app-update.tar

# Transfer to server (use scp since rsync is not available on Windows by default)
scp app-update.tar.gz user@server:/opt/veristoretools3/

# Clean up
Remove-Item app-update.tar
```

**On the Server:**

```bash
cd /opt/veristoretools3

# Load the updated image
docker load < app-update.tar.gz

# Restart the app container
docker compose up -d app
```

If there are new database migrations:

```bash
docker compose exec app ./migrate
```

### Common Commands

```bash
cd /opt/veristoretools3

docker compose ps                     # Check status
docker compose logs -f app            # View app logs (live)
docker compose restart app            # Restart app only
docker compose down                   # Stop everything (data preserved)
docker compose up -d                  # Start everything
docker compose down -v                # WARNING: destroys all data
```

### Database Access

```bash
# MySQL shell
docker compose exec mysql mysql -u root -p veristoretools3

# Backup
docker compose exec mysql mysqldump -u root -p veristoretools3 > backup_$(date +%Y%m%d).sql

# Restore
docker compose exec -T mysql mysql -u root -p veristoretools3 < backup_20260222.sql
```

### Services

| Service | Internal Port | External Port | Description |
|---------|--------------|---------------|-------------|
| app | 8080 | 8080 | VeriStore Tools 3 |
| mysql | 3306 | 3307 | MySQL 8.0 database |
| redis | 6379 | 6380 | Redis 7 (job queue) |

### Architecture

```
┌─────────────────────────────────────────────┐
│               Linux Server                  │
│                                             │
│  ┌───────────┐  ┌─────────┐  ┌───────────┐ │
│  │    App    │──│  MySQL  │  │   Redis   │ │
│  │   :8080   │  │  :3306  │  │   :6379   │ │
│  └───────────┘  └─────────┘  └───────────┘ │
│       │                                     │
│       ├── config.docker.yaml (mounted)      │
│       ├── mysql_data (Docker volume)        │
│       └── redis_data (Docker volume)        │
└───────┼─────────────────────────────────────┘
        │ HTTPS
        ▼
  TMS API Server
  (app.veristore.net / tps.veristore.net)
```

### Troubleshooting

| Problem | Solution |
|---------|---------|
| App won't start | `docker compose logs app` — check for config errors or MySQL/Redis not ready |
| Port 8080 in use | Change port in `docker-compose.yml`: `"9090:8080"` |
| Disk space | `docker system df` to check, `docker image prune` to clean |

---

## 4. Storage & Maintenance

### Logging

- **Library:** Zerolog (structured JSON)
- **Output:** stdout only -- no log files written to disk
- **Log Level:** Debug (dev), Info (production)
- **Logged:** HTTP requests, app lifecycle, background jobs, errors

```bash
# View logs locally
./veristoretools3

# View Docker logs
docker compose logs -f app
```

### Items That Grow Over Time

| Item | Location | Growth Rate | Risk |
|---|---|---|---|
| `activity_log` table | MySQL | Every user action | Low (small rows, but no retention policy) |
| `queue_log` table | MySQL | Every background job | Low (small rows, no cleanup) |
| `export.exp_data` (longblob) | MySQL | Every export | **Medium-High** (full Excel files stored as BLOBs) |
| `tms_report.tms_rpt_file` (longblob) | MySQL | Every report | **Medium** (report files stored as BLOBs) |
| `static/export/*.xlsx` | Filesystem | Every export | Low (easy to clean with cron) |
| `static/import/*` | Filesystem | Every import | Low (easy to clean with cron) |
| `dump.rdb` | Filesystem | Redis persistence | Low (Asynq has built-in retention) |

### Recommended Cleanup Actions

1. **Database longblobs** (highest priority): Implement a scheduled job or cron to delete export/report BLOBs older than N days
2. **Activity/queue logs**: Add a retention policy (e.g., delete records older than 90 days)
3. **Export files**: Set up a cron job to remove old files:
   ```bash
   # Delete export files older than 30 days
   find static/export/ -name "*.xlsx" -mtime +30 -delete
   ```
4. **Import files**: Same approach:
   ```bash
   # Delete import files older than 30 days
   find static/import/ -type f -mtime +30 -delete
   ```

### Production Server

- **Server Specs:** 24 cores, 62GB RAM, Dell server
- **TMS Server:** 172.17.30.158:8280

---

## 5. External Dependencies

| Service | URL | Purpose |
|---|---|---|
| Veristore TMS Web | http://10.90.30.158:8280 | Terminal management system UI |
| Veristore TMS API | https://tps.veristore.net | Terminal management REST API |
| MySQL 8.0 | localhost:3306 / mysql:3306 | Primary database |
| Redis 7 | localhost:6379 / redis:6379 | Background task queue |
