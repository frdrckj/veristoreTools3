# 4. Deployment Diagram

Docker Compose layout on Dell PowerEdge R750xs.

```mermaid
graph TB
    subgraph Server["Dell PowerEdge R750xs<br/>Intel Xeon | 64 GB DDR5 | 2x 600GB SSD<br/>IP: 192.168.0.120"]
        subgraph DockerEngine["Docker Engine"]
            subgraph Network["Docker Bridge Network: veristoretools3_default"]

                subgraph AppContainer["Container: app<br/>Image: alpine:3.20<br/>Restart: unless-stopped"]
                    Binary["Go Binary: /app/server<br/>~30 MB compiled"]
                    Static["Static Assets: /app/static<br/>CSS, JS, Images"]
                    Config["Config: /app/config.yaml<br/>Mounted from host"]
                    Migrate["Migration: /app/migrate"]
                end

                subgraph MySQLContainer["Container: mysql<br/>Image: mysql:8.0<br/>Restart: unless-stopped"]
                    MySQLProc["mysqld Process"]
                    MySQLData[("Volume: mysql_data<br/>/var/lib/mysql")]
                    MySQLHealth["Healthcheck:<br/>mysqladmin ping<br/>5s interval, 10 retries"]
                end

                subgraph RedisContainer["Container: redis<br/>Image: redis:7-alpine<br/>Restart: unless-stopped"]
                    RedisProc["redis-server Process"]
                    RedisData[("Volume: redis_data<br/>/data")]
                    RedisHealth["Healthcheck:<br/>redis-cli ping<br/>5s interval, 5 retries"]
                end
            end

            subgraph TMSDocker["TMS VeriStore Docker (Separate Compose)"]
                TMSApp["TMS Application<br/>Java/Spring Boot"]
                TMSDB[("TMS Database")]
            end
        end
    end

    subgraph Ports["Exposed Ports"]
        P8080["Host :8080"]
        P3307["Host :3307"]
        P6380["Host :6380"]
    end

    P8080 -->|"→ :8080"| AppContainer
    P3307 -->|"→ :3306"| MySQLContainer
    P6380 -->|"→ :6379"| RedisContainer

    AppContainer -->|"GORM TCP :3306"| MySQLContainer
    AppContainer -->|"Asynq TCP :6379"| RedisContainer
    AppContainer -->|"HTTPS"| TMSDocker

    subgraph StartupOrder["Container Startup Order"]
        direction LR
        S1["1. MySQL starts"] --> S2["2. Redis starts"]
        S2 --> S3["3. Both healthy ✓"]
        S3 --> S4["4. App starts"]
    end

    subgraph BuildStage["Dockerfile Multi-Stage Build"]
        direction LR
        B1["golang:1.23-alpine<br/>go mod download<br/>go build -ldflags='-s -w'"] --> B2["alpine:3.20<br/>COPY server, migrate<br/>COPY static, migrations<br/>EXPOSE 8080<br/>CMD ./server"]
    end

    classDef container fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
    classDef volume fill:#fff8e1,stroke:#f9a825,stroke-width:2px
    classDef port fill:#e8f5e9,stroke:#2e7d32,stroke-width:1px

    class AppContainer,MySQLContainer,RedisContainer container
    class MySQLData,RedisData volume
    class P8080,P3307,P6380 port
```

## Environment Configuration

| Container | Environment Variable | Value |
|-----------|---------------------|-------|
| app | TZ | Asia/Jakarta |
| mysql | TZ | Asia/Jakarta |
| mysql | MYSQL_ROOT_PASSWORD | veristoretools3 |
| mysql | MYSQL_DATABASE | veristoretools3 |

## Volume Mounts

| Container | Host Path | Container Path | Mode |
|-----------|-----------|----------------|------|
| app | ./config.docker.yaml | /app/config.yaml | read-only |
| mysql | mysql_data (named) | /var/lib/mysql | read-write |
| redis | redis_data (named) | /data | read-write |
