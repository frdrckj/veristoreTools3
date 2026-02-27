# 8. Infrastructure Diagram

Network topology, Docker bridge, ports, Redis/MySQL connections.

## 8.1 Network Topology

```mermaid
graph TB
    subgraph Internet["External Network"]
        TMSSession["TMS Session API<br/>app.veristore.net<br/>HTTPS :443"]
        TMSSigned["TMS Signed API<br/>tps.veristore.net<br/>HTTPS :443"]
    end

    subgraph Dell["Dell PowerEdge R750xs (192.168.0.120)"]
        subgraph HostNetwork["Host Network"]
            H8080["Host :8080"]
            H3307["Host :3307"]
            H6380["Host :6380"]
        end

        subgraph DockerBridge["Docker Bridge Network: veristoretools3_default<br/>Subnet: 172.x.0.0/16 (auto-assigned)"]

            subgraph App["app container"]
                EchoServer["Echo HTTP Server<br/>:8080"]
                AsynqWorker["Asynq Worker<br/>10 concurrent"]
                AsynqScheduler["Asynq Scheduler<br/>cron jobs"]
            end

            subgraph MySQL["mysql container"]
                MySQLServer["MySQL 8.0<br/>:3306<br/>DB: veristoretools3<br/>Max conn: 25 open / 10 idle"]
            end

            subgraph Redis["redis container"]
                RedisServer["Redis 7<br/>:6379<br/>DB: 0<br/>No auth (internal)"]
            end
        end
    end

    subgraph LAN["Office LAN"]
        Admin["Admin Browser<br/>http://192.168.0.120:8080"]
    end

    Admin -->|"HTTP"| H8080
    H8080 -->|"NAT :8080→:8080"| EchoServer
    H3307 -->|"NAT :3307→:3306"| MySQLServer
    H6380 -->|"NAT :6380→:6379"| RedisServer

    EchoServer -->|"TCP mysql:3306<br/>GORM MySQL Driver"| MySQLServer
    EchoServer -->|"TCP redis:6379<br/>Asynq Enqueue"| RedisServer
    AsynqWorker -->|"TCP redis:6379<br/>Asynq Dequeue"| RedisServer
    AsynqWorker -->|"TCP mysql:3306<br/>Update Progress"| MySQLServer
    AsynqScheduler -->|"TCP redis:6379<br/>Schedule Tasks"| RedisServer

    EchoServer -->|"HTTPS :443"| TMSSession
    EchoServer -->|"HTTPS :443"| TMSSigned
    AsynqWorker -->|"HTTPS :443"| TMSSession
    AsynqWorker -->|"HTTPS :443"| TMSSigned

    classDef appNode fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
    classDef dbNode fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef extNode fill:#fce4ec,stroke:#c62828,stroke-width:2px
    classDef lanNode fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px

    class App appNode
    class MySQL,Redis dbNode
    class TMSSession,TMSSigned extNode
    class Admin lanNode
```

## 8.2 Port Mapping

```mermaid
graph LR
    subgraph External["Host Ports (accessible from LAN)"]
        EP1[":8080 → V3 Web App"]
        EP2[":3307 → MySQL (admin access)"]
        EP3[":6380 → Redis (admin access)"]
    end

    subgraph Internal["Docker Internal Ports"]
        IP1["app:8080"]
        IP2["mysql:3306"]
        IP3["redis:6379"]
    end

    EP1 --> IP1
    EP2 --> IP2
    EP3 --> IP3
```

## 8.3 Docker DNS Resolution

All containers within `veristoretools3_default` bridge network communicate via Docker's embedded DNS:

| Service Name | Resolves To | Port | Protocol |
|-------------|-------------|------|----------|
| `mysql` | 172.x.0.2 (auto) | 3306 | TCP (MySQL protocol) |
| `redis` | 172.x.0.3 (auto) | 6379 | TCP (RESP protocol) |
| `app` | 172.x.0.4 (auto) | 8080 | TCP (HTTP) |

## 8.4 Connection Configuration

### MySQL (from config.yaml)

```
Host:          mysql       (Docker DNS)
Port:          3306        (internal)
Database:      veristoretools3
User:          root
Password:      veristoretools3
Charset:       utf8mb4
MaxOpenConns:  25
MaxIdleConns:  10
DSN: root:veristoretools3@tcp(mysql:3306)/veristoretools3?charset=utf8mb4&parseTime=True&loc=Local
```

### Redis (from config.yaml)

```
Address:       redis:6379  (Docker DNS)
Password:      (empty)
Database:      0
Usage:         Asynq job queue broker
```

### TMS API (from config.yaml)

```
BaseURL:       https://app.veristore.net   (Session API)
APIBaseURL:    https://tps.veristore.net   (Signed API)
AccessKey:     (configured per environment)
AccessSecret:  (configured per environment)
HTTP Timeout:  60 seconds
TLS Verify:    configurable (InsecureSkipVerify)
```

## 8.5 Data Persistence

```mermaid
graph TD
    subgraph Volumes["Docker Named Volumes"]
        MV["mysql_data<br/>/var/lib/docker/volumes/mysql_data/_data"]
        RV["redis_data<br/>/var/lib/docker/volumes/redis_data/_data"]
    end

    subgraph Containers["Container Mount Points"]
        MC["/var/lib/mysql<br/>All database files"]
        RC["/data<br/>Redis RDB snapshots"]
        AC["/app/config.yaml<br/>Bind mount from host"]
    end

    subgraph Host["Host Filesystem"]
        HConfig["./config.docker.yaml<br/>Application configuration"]
    end

    MV -->|"mount"| MC
    RV -->|"mount"| RC
    HConfig -->|"bind mount (read-only)"| AC
```

## 8.6 Scheduled Background Tasks

| Task | Schedule | Description |
|------|----------|-------------|
| `tms:ping` | Every 15 minutes | Health check TMS API session |
| `tms:scheduler_check` | Every 1 minute | Monitor queue health |
| `export:terminal` | On demand | Terminal export to Excel |
| `import:terminal` | On demand | Terminal import from Excel |
| `import:merchant` | On demand | Merchant data import |
| `sync:parameter` | On demand | Parameter synchronization |
| `report:terminal` | On demand | Report generation |

## 8.7 Security Boundaries

```mermaid
graph TB
    subgraph Public["Public Zone (LAN Accessible)"]
        Port8080[":8080 - Web UI<br/>Protected by SessionAuth + RBAC"]
    end

    subgraph Internal["Internal Zone (Docker Bridge Only)"]
        Port3306[":3306 MySQL<br/>Root access, no external auth"]
        Port6379[":6379 Redis<br/>No password"]
    end

    subgraph AdminAccess["Admin Debug Access (Host Only)"]
        Port3307[":3307 → MySQL<br/>For backup/admin tools"]
        Port6380[":6380 → Redis<br/>For monitoring"]
    end

    subgraph Outbound["Outbound (HTTPS)"]
        TMSAPI["TMS APIs :443<br/>HMAC-SHA256 + Session tokens"]
    end

    Port8080 -.->|"Cookie session"| Internal
    Internal -.->|"Exposed to host"| AdminAccess
    Internal -.->|"App outbound"| Outbound
```
