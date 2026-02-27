# 7. API Flow / Integration Diagram

TMS API endpoints, session management, and old vs new API paths.

## 7.1 Dual API Architecture

```mermaid
graph TB
    subgraph V3App["VeriStore Tools V3"]
        TMSClient["tms/client.go<br/>3131 lines | 60+ methods"]

        subgraph AuthMethods["Authentication"]
            HMACSign["generateSignature()<br/>HMAC-SHA256<br/>accessKey + timestamp<br/>sorted key=value pairs"]
            SessionToken["doPost(session, path, body)<br/>Authorization: session header<br/>Auto token renewal"]
        end
    end

    subgraph NewAPI["Signed API (tps.veristore.net)"]
        direction TB
        N1["POST /v1/tps/terminal/list<br/>List terminals (paginated)"]
        N2["GET /v1/tps/terminal/{serialNum}<br/>Terminal detail by SN"]
        N3["GET /v1/tps/terminal/id/{id}<br/>Terminal detail by ID"]
        N4["GET /v1/tps/terminal/applist/{id}<br/>Terminal app list"]
        N5["POST /v1/tps/terminal/delete/{id}<br/>Delete terminal"]
    end

    subgraph OldAPI["Session API (app.veristore.net)"]
        direction TB
        O1["POST /market/login<br/>Login (get session token)"]
        O2["POST /market/manage/terminal/page<br/>Search by Merchant/Group/TID/MID"]
        O3["POST /market/manage/terminalAppParameter/view<br/>Get terminal parameters"]
        O4["POST /market/manage/merchant/list<br/>Merchant management"]
        O5["POST /market/manage/group/list<br/>Group management"]
        O6["POST /market/manage/index/topSum<br/>Dashboard summary"]
        O7["POST /market/common/operationMark<br/>Operation mark"]
        O8["GET /market/common/getCountryList<br/>Location data (country/state/city)"]
        O9["GET /market/common/getVendorList<br/>Device vendors & models"]
        O10["POST /market/manage/app/list<br/>Application list"]
        O11["POST /market/common/checkToken<br/>Session validation"]
    end

    TMSClient -->|"HMAC-SHA256 Signature"| NewAPI
    TMSClient -->|"Session Token Header"| OldAPI

    classDef newapi fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px
    classDef oldapi fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef client fill:#e3f2fd,stroke:#1565c0,stroke-width:2px

    class N1,N2,N3,N4,N5 newapi
    class O1,O2,O3,O4,O5,O6,O7,O8,O9,O10,O11 oldapi
    class TMSClient client
```

## 7.2 Search Type → API Routing

```mermaid
graph LR
    subgraph Input["Search Type"]
        T0["Type 0: Serial Number"]
        T1["Type 1: Merchant"]
        T2["Type 2: Group"]
        T3["Type 3: TID"]
        T4["Type 4: CSI"]
        T5["Type 5: MID"]
    end

    subgraph Router["client.go::GetTerminalListSearch"]
        Decision{{"Query Type?"}}
    end

    subgraph APIs["API Endpoint"]
        NewPath["NEW: doSignedPost()<br/>POST /v1/tps/terminal/list<br/>Auth: HMAC-SHA256<br/>Stateless, faster"]
        OldPath["OLD: doPost(session)<br/>POST /market/manage/terminal/page<br/>Auth: Session token<br/>Supports merchant/group search"]
    end

    T0 --> Decision
    T1 --> Decision
    T2 --> Decision
    T3 --> Decision
    T4 --> Decision
    T5 --> Decision

    Decision -->|"SN (0), CSI (4)"| NewPath
    Decision -->|"Merchant (1), Group (2),<br/>TID (3), MID (5)"| OldPath
```

## 7.3 Session Management

```mermaid
graph TD
    subgraph Login["User Login to V3 App"]
        L1["POST /user/login"]
        L2["Authenticate against user table"]
        L3["Clear old TMS session<br/>user.tms_session = NULL"]
        L4["Encrypt & store TMS password<br/>user.tms_password = AES(plain)"]
        L1 --> L2 --> L3 --> L4
    end

    subgraph TMSLogin["TMS Login (Lazy)"]
        T1["First TMS API call needed"]
        T2["GetUserSession(username)<br/>Check user.tms_session"]
        T3{{"Session exists?"}}
        T4["Use existing session"]
        T5["Decrypt user.tms_password"]
        T6["POST /market/login<br/>Get new TMS session token"]
        T7["Store in user.tms_session"]
        T1 --> T2 --> T3
        T3 -->|"Yes"| T4
        T3 -->|"No"| T5 --> T6 --> T7
    end

    subgraph Renewal["Auto Token Renewal"]
        R1["API returns 'toke更新'<br/>(token renewal signal)"]
        R2["renewToken()"]
        R3["Extract new token from response"]
        R4["Update tms_login.tms_login_session<br/>or user.tms_session"]
        R5["Retry original request"]
        R1 --> R2 --> R3 --> R4 --> R5
    end

    subgraph Fallback["Global Session (Fallback)"]
        F1["GetSession()"]
        F2["SELECT * FROM tms_login<br/>WHERE tms_login_enable = '1'"]
        F3["Return tms_login_session"]
        F1 --> F2 --> F3
    end

    Login -.->|"Next TMS call"| TMSLogin
    T4 -.->|"If expired"| Renewal
    TMSLogin -.->|"If no user session"| Fallback
```

## 7.4 Complete TMS API Endpoint Map

### Signed API (tps.veristore.net)

| Method | Endpoint | Purpose | Used By |
|--------|----------|---------|---------|
| POST | `/v1/tps/terminal/list` | List/search terminals | Terminal page, Export, Delete |
| GET | `/v1/tps/terminal/{sn}` | Detail by serial number | Terminal detail |
| GET | `/v1/tps/terminal/id/{id}` | Detail by ID | Export worker |
| GET | `/v1/tps/terminal/applist/{id}` | Apps on terminal | Export worker |
| POST | `/v1/tps/terminal/delete/{id}` | Delete terminal | Delete handler |

### Session API (app.veristore.net)

| Method | Endpoint | Purpose | Used By |
|--------|----------|---------|---------|
| POST | `/market/login` | Authentication | TMS login |
| POST | `/market/common/checkToken` | Validate session | Session check |
| GET | `/market/common/getCaptcha` | CAPTCHA image | TMS login page |
| GET | `/market/common/getMarketsByUser` | Reseller list | TMS login |
| POST | `/market/manage/terminal/page` | Search (merchant/group/TID/MID) | Terminal search |
| POST | `/market/manage/terminalAppParameter/view` | Get parameters | Export, Edit |
| POST | `/market/common/operationMark` | Operation mark | Export |
| POST | `/market/manage/index/topSum` | Dashboard counts | Dashboard |
| POST | `/market/manage/index/newAppList` | Recent apps | Dashboard |
| POST | `/market/manage/merchant/list` | Merchant list | Merchant page |
| POST | `/market/manage/group/list` | Group list | Group page |
| POST | `/market/manage/app/list` | Application list | App page |
| GET | `/market/common/getCountryList` | Countries | Terminal add/edit |
| GET | `/market/common/getStateList` | States | Terminal add/edit |
| GET | `/market/common/getCityList` | Cities | Terminal add/edit |
| GET | `/market/common/getDistrictList` | Districts | Terminal add/edit |
| GET | `/market/common/getTimeZoneList` | Timezones | Terminal add/edit |
| GET | `/market/common/getVendorList` | Vendors | Terminal add |
| GET | `/market/common/getModelList` | Models | Terminal add |
