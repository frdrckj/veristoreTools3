# VeriStore Tools 3 - Deployment Guide

## Prerequisites

- Docker & Docker Compose installed
- Access to the production server

## Quick Start

```bash
# 1. Build and start all services
docker compose up -d --build

# 2. Run database migrations (first time only)
docker compose exec app ./migrate

# 3. Open in browser
http://<server-ip>:8080
```

## Services

| Service | Internal Port | External Port | Description |
|---------|--------------|---------------|-------------|
| app     | 8080         | 8080          | Go application |
| mysql   | 3306         | 3307          | MySQL 8.0 database |
| redis   | 6379         | 6380          | Redis 7 (job queue) |

## Configuration

Edit `config.docker.yaml` before deploying. Key settings to change for production:

```yaml
app:
  debug: false
  session_secret: "CHANGE-THIS-TO-A-RANDOM-STRING"

database:
  password: "CHANGE-THIS-TO-A-STRONG-PASSWORD"
```

Make sure `docker-compose.yml` MySQL password matches:

```yaml
MYSQL_ROOT_PASSWORD: "CHANGE-THIS-TO-A-STRONG-PASSWORD"
```

## Migrating Data from V2

### Option A: From local V2 database

```bash
# Export from V2
mysqldump -u root veristoretools2 > v2_backup.sql

# Import into V3 Docker MySQL
mysql -u root -p<password> -h 127.0.0.1 -P 3307 veristoretools3 < v2_backup.sql
```

### Option B: From remote V2 server

```bash
# Export from V2 server
ssh user@v2-server "mysqldump -u root veristoretools2" > v2_backup.sql

# Import into V3
mysql -u root -p<password> -h 127.0.0.1 -P 3307 veristoretools3 < v2_backup.sql
```

### Verify Migration

```bash
docker compose exec app ./migrate verify
```

Expected output:

```
TABLE                 V2 COUNT   V3 COUNT   MATCH
-----                 --------   --------   -----
user                  15         15         OK
terminal              25144      25144      OK
...
All table row counts match.
```

## Common Commands

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart app only
docker compose restart app

# View app logs
docker compose logs -f app

# View all logs
docker compose logs -f

# Rebuild after code changes
docker compose up -d --build app

# Access MySQL shell
docker compose exec mysql mysql -u root -p veristoretools3

# Access Redis shell
docker compose exec redis redis-cli
```

## Backup

```bash
# Database backup
docker compose exec mysql mysqldump -u root -p veristoretools3 > backup_$(date +%Y%m%d).sql

# Restore
docker compose exec -T mysql mysql -u root -p veristoretools3 < backup_20260221.sql
```

## Updating

```bash
# Pull latest code
git pull

# Rebuild and restart
docker compose up -d --build app

# Run new migrations if any
docker compose exec app ./migrate
```
