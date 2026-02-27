# 5. Component Diagram

Internal package structure of VeriStore Tools V3.

```mermaid
graph TB
    subgraph EntryPoint["cmd/server/main.go"]
        Main["main()<br/>465 lines<br/>Init config, DB, sessions,<br/>Casbin, handlers, routes,<br/>workers, graceful shutdown"]
    end

    subgraph Middleware["internal/middleware"]
        Recovery["recovery.go<br/>Panic Recovery"]
        Logger["logger.go<br/>Zerolog Request Logger"]
        Auth["auth.go<br/>SessionAuth (gorilla)"]
        RBAC["rbac.go<br/>Casbin RBAC Enforcer"]
        ActLog["activitylog.go<br/>DB Activity Logger"]
    end

    subgraph Handlers["HTTP Handlers"]
        AuthH["internal/auth<br/>handler.go | service.go<br/>Login, Logout"]
        SiteH["internal/site<br/>handler.go<br/>Dashboard"]
        UserH["internal/user<br/>handler.go | service.go<br/>User CRUD"]
        TMSH["internal/tms<br/>handler.go (2724 lines)<br/>Terminal, Export, Delete,<br/>Merchant, Group, App"]
        TMSLogin["internal/tms<br/>login_handler.go<br/>TMS Login Bridge"]
        TermH["internal/terminal<br/>handler.go | param_handler.go<br/>Local Terminal CRUD"]
        AdminH["internal/admin<br/>handler.go<br/>Activity Log, Technician,<br/>FAQ, Backup, Template Params"]
        SyncH["internal/sync<br/>handler.go | service.go<br/>Parameter Sync"]
        CSIH["internal/csi<br/>handler.go | report_handler.go<br/>Verification Reports"]
        ActH["internal/activation<br/>handler.go | api_handler.go<br/>App Activation + Public API"]
    end

    subgraph Core["Core Services"]
        TMSClient["internal/tms/client.go<br/>3131 lines | 60+ methods<br/>TMS API Communication<br/>Session + Signed Auth"]
        TMSService["internal/tms/service.go<br/>Session Mgmt, Search,<br/>Delete, Bulk Operations"]
        TMSRepo["internal/tms/repository.go<br/>tms_login, tms_report"]
        Encrypt["internal/tms/encryption.go<br/>AES Encrypt/Decrypt"]
    end

    subgraph Background["internal/queue (Asynq)"]
        WorkerMgr["worker.go<br/>Worker + Scheduler Setup"]
        ExportTask["export_terminal.go<br/>10 concurrent workers<br/>Excel generation"]
        ImportTask["import_terminal.go<br/>Terminal Import"]
        ImportMerch["import_merchant.go<br/>Merchant Import"]
        ReportTask["report_terminal.go<br/>Report Generation"]
        SyncTask["sync_parameter.go<br/>Parameter Sync Job"]
        PingTask["tms_ping.go<br/>TMS Health Check (15m)"]
        SchedulerCheck["scheduler_check.go<br/>Queue Monitor (1m)"]
    end

    subgraph Templates["templates/"]
        Layout["templates/layout/<br/>Base layout, navbar, sidebar"]
        VSTmpl["templates/veristore/<br/>terminal, export, import,<br/>merchant, group, app"]
        AdminTmpl["templates/admin/<br/>activity_log, technician,<br/>user, faq"]
        SharedTmpl["templates/shared/<br/>pagination, flash, modal"]
    end

    subgraph Data["Data Layer"]
        Config["internal/config<br/>YAML config loading"]
        Shared["internal/shared<br/>Session flash, render helpers"]
        Models["Models (in each package)<br/>User, Terminal, Export,<br/>Import, ActivityLog, etc."]
        Repos["Repositories (in each package)<br/>GORM database operations"]
    end

    %% Dependencies
    Main --> Middleware
    Main --> Handlers
    Main --> Core
    Main --> Background
    Main --> Config

    Middleware --> Shared

    AuthH --> UserH
    AuthH --> TMSService
    TMSH --> TMSService
    TMSH --> TMSClient
    TMSLogin --> TMSClient
    TMSService --> TMSClient
    TMSService --> TMSRepo
    TMSClient --> Encrypt

    Handlers --> Templates
    Handlers --> Shared
    Handlers --> Models
    Handlers --> Repos

    Background --> TMSService
    Background --> TMSClient
    Background --> Repos

    classDef handler fill:#e8eaf6,stroke:#283593,stroke-width:2px
    classDef core fill:#fce4ec,stroke:#c62828,stroke-width:2px
    classDef bg fill:#e0f2f1,stroke:#00695c,stroke-width:2px
    classDef data fill:#fff3e0,stroke:#e65100,stroke-width:2px
    classDef mw fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px

    class AuthH,SiteH,UserH,TMSH,TMSLogin,TermH,AdminH,SyncH,CSIH,ActH handler
    class TMSClient,TMSService,TMSRepo,Encrypt core
    class ExportTask,ImportTask,ImportMerch,ReportTask,SyncTask,PingTask,SchedulerCheck,WorkerMgr bg
    class Config,Shared,Models,Repos data
    class Recovery,Logger,Auth,RBAC,ActLog mw
```

## Package Summary

| Package | Files | Purpose |
|---------|-------|---------|
| cmd/server | 1 | Application entry point, init & wiring |
| internal/config | 2 | YAML config loading & DSN builder |
| internal/middleware | 5 | Recovery, logging, auth, RBAC, activity log |
| internal/shared | ~3 | Flash messages, render helpers, DB utils |
| internal/auth | 5 | Login/logout, password verification |
| internal/user | 4 | User CRUD, password management |
| internal/tms | 10 | TMS API integration (core package) |
| internal/terminal | 6 | Local terminal CRUD |
| internal/admin | 4 | Activity log, technician, FAQ, backup |
| internal/sync | 4 | Parameter synchronization |
| internal/csi | 5 | Verification reports |
| internal/activation | 5 | App activation + public API |
| internal/queue | 9 | Background jobs (export, import, sync, ping) |
| internal/site | 1 | Dashboard |
