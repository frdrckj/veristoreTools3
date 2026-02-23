# VeriStore Tools 3 — Deployment Guide

## Overview

The app is built on your Mac and deployed as Docker images to the Linux server.
The server does **not** need internet access, Go, or the source code.

---

## Prerequisites

| Machine | Requirements |
|---------|-------------|
| Mac (build) | Docker Desktop |
| Server (production) | Docker Engine + Docker Compose |

---

## First-Time Deployment

### Step 1: Build & Package (on Mac)

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

### Step 2: Transfer to Server

Copy the folder to the server via USB, SCP, or any method available:

```bash
scp -r deploy-package/ user@server:/opt/veristoretools3/
```

### Step 3: Configure (on Server)

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
  base_url: "https://app.veristore.net"
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

### Step 4: Start (on Server)

```bash
./setup.sh
```

The app will be available at `http://server-ip:8080`.

### Step 5: Migrate Data from V2

```bash
# On V2 server: export the database
mysqldump -u root -p veristoretools2 > v2_backup.sql

# Transfer v2_backup.sql to V3 server, then:
docker compose exec -T mysql mysql -u root -p veristoretools3 < v2_backup.sql

# Run V3 migrations to add new columns/tables
docker compose exec app ./migrate
```

---

## Updating the App

When you make code changes and need to deploy an update:

### On your Mac:

```bash
cd veristoreTools3

# Rebuild
./deploy.sh
```

### Transfer only the app image (smaller & faster):

MySQL and Redis images never change, so you can skip them:

```bash
docker build --platform linux/amd64 -t veristoretools3-app:latest .

# On Mac — save just the app image (~20 MB vs ~600 MB)
docker save veristoretools3-app:latest -o app-update.tar
gzip app-update.tar


rsync -avz --progress app-update.tar.gz frearm01@10.120.8.116:/opt/veristoretools3/

```

Transfer only `app-update.tar.gz` to the server.

### On the Server:

```bash
cd /opt/veristoretools3

# Load the updated image
docker load < app-update.tar.gz

# Restart only the app container (database & Redis data are preserved)
docker compose restart app
```

### If there are new database migrations:

```bash
# After restarting, run migrate
docker compose exec app ./migrate
```

---

## Common Commands

```bash
cd /opt/veristoretools3

# Check status of all services
docker compose ps

# View app logs (live, follow)
docker compose logs -f app

# View all service logs
docker compose logs

# Restart app only (keeps DB & Redis running)
docker compose restart app

# Stop everything (data is preserved)
docker compose down

# Start everything
docker compose up -d

# Stop everything AND delete all data (database, redis)
docker compose down -v                    # WARNING: destroys all data
```

---

## Database

### Access MySQL Shell

```bash
docker compose exec mysql mysql -u root -p veristoretools3
```

### Backup

```bash
docker compose exec mysql mysqldump -u root -p veristoretools3 > backup_$(date +%Y%m%d).sql
```

### Restore

```bash
docker compose exec -T mysql mysql -u root -p veristoretools3 < backup_20260222.sql
```

---

## Services

| Service | Internal Port | External Port | Description |
|---------|--------------|---------------|-------------|
| app | 8080 | 8080 | VeriStore Tools 3 |
| mysql | 3306 | 3307 | MySQL 8.0 database |
| redis | 6379 | 6380 | Redis 7 (job queue) |

---

## Troubleshooting

### App won't start

```bash
docker compose logs app
```

Common causes:
- `connection refused` to mysql/redis — services not ready yet, wait and retry
- Config errors — check `config.docker.yaml` YAML syntax

### Port 8080 already in use

Change the port in `docker-compose.yml`:

```yaml
app:
  ports:
    - "9090:8080"    # access via port 9090 instead
```

### Disk space

```bash
docker system df          # check Docker disk usage
docker image prune        # clean up unused images
```

---

## Architecture

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
