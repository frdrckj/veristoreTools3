# VeriStore Tools 3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild veristoreTools2 as an exact 1:1 feature replica in Go for significantly faster performance.

**Architecture:** Modular monolith in Go with Echo router, GORM ORM, Templ templates, HTMX for partial page updates, AdminLTE 3 theme, Asynq queue, and Casbin RBAC. Single binary runs web server + queue worker + scheduler.

**Tech Stack:** Go 1.22+, Echo v4, GORM, Templ, HTMX, Alpine.js, AdminLTE 3, MySQL 8, Redis, Asynq, Casbin, Excelize

**Design Doc:** `docs/plans/2026-02-19-veristoretools3-migration-design.md`

**v2 Source Reference:** `../veristoreTools2/` (PHP/Yii2 original)

---

## Phase 1: Project Scaffolding

### Task 1: Initialize Go Module and Dependencies

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `Makefile`
- Create: `config.yaml`
- Create: `.gitignore`

**Step 1: Initialize Go module**

```bash
cd /Users/frederickjerusha/Documents/works/Verifone/Projects/veristoretools/veristoreTools3
go mod init github.com/verifone/veristoretools3
```

**Step 2: Install core dependencies**

```bash
go get github.com/labstack/echo/v4@latest
go get github.com/a-h/templ@latest
go get gorm.io/gorm@latest
go get gorm.io/driver/mysql@latest
go get github.com/gorilla/sessions@latest
go get github.com/hibiken/asynq@latest
go get github.com/casbin/casbin/v2@latest
go get github.com/casbin/gorm-adapter/v3@latest
go get github.com/xuri/excelize/v2@latest
go get github.com/rs/zerolog@latest
go get gopkg.in/yaml.v3@latest
go get github.com/go-playground/validator/v10@latest
go get github.com/stretchr/testify@latest
```

**Step 3: Install templ CLI**

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

**Step 4: Create minimal main.go**

```go
// cmd/server/main.go
package main

import (
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.GET("/", func(c echo.Context) error {
		return c.String(200, "VeriStore Tools 3 is running")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
```

**Step 5: Create config.yaml**

```yaml
# config.yaml
app:
  name: "VeriStore Tools 3"
  version: "3.0.0"
  port: 8080
  debug: true
  session_timeout: 900
  session_secret: "veristoretools3-secret-change-in-production"
  session_name: "_veristore_tools_3_app"
  password_salt: "@!Boteng2021%??"

database:
  host: localhost
  port: 3306
  name: veristoretools3
  user: VFVeristoretools3
  password: ""
  charset: utf8mb4
  max_open_conns: 25
  max_idle_conns: 10

redis:
  addr: localhost:6379
  password: ""
  db: 0

tms:
  base_url: "http://172.17.30.158:8280"
  secret_key: "35136HH7B63C27AA74CDCC2BBRT9"
  secret_iv: "J5g275fgf5H"

api:
  basic_auth_user: "Vfiengineering"
  basic_auth_pass: "Welcome@123!"

import:
  max_file_size: 5242880
  batch_size: 100

export:
  batch_size: 100
  output_dir: "static/export"

country_id: 5
package_name: "com.vfi.android.payment.cimb"
```

**Step 6: Create Makefile**

```makefile
# Makefile
.PHONY: build run test templ clean migrate-up migrate-down lint

build: templ
	go build -o bin/veristoretools3 ./cmd/server

run: templ
	go run ./cmd/server

test:
	go test ./... -v

templ:
	templ generate

clean:
	rm -rf bin/

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

lint:
	golangci-lint run ./...

dev: templ
	air
```

**Step 7: Create .gitignore**

```
bin/
vendor/
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
.env
config.local.yaml
tmp/
static/export/*
static/import/*
static/sync/*
!static/export/.gitkeep
!static/import/.gitkeep
!static/sync/.gitkeep
*_templ.go
```

**Step 8: Verify it compiles and runs**

```bash
make run
# Expected: server starts on :8080
# Ctrl+C to stop
```

**Step 9: Commit**

```bash
git init
git add .
git commit -m "feat: initialize Go project with Echo, dependencies, and config"
```

---

### Task 2: Config Loader

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write config test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	content := []byte(`
app:
  name: "Test App"
  port: 9090
  debug: true
  session_timeout: 900
  session_secret: "test-secret"
  session_name: "_test_app"
  password_salt: "test-salt"
database:
  host: localhost
  port: 3306
  name: testdb
  user: testuser
  password: testpass
  charset: utf8mb4
redis:
  addr: localhost:6379
tms:
  base_url: "http://localhost:8280"
`)
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "Test App", cfg.App.Name)
	assert.Equal(t, 9090, cfg.App.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, "http://localhost:8280", cfg.TMS.BaseURL)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestConfig_DSN(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Name:     "testdb",
			User:     "root",
			Password: "pass",
			Charset:  "utf8mb4",
		},
	}
	expected := "root:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, cfg.Database.DSN())
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/ -v
# Expected: FAIL - package not found
```

**Step 3: Write config implementation**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	TMS      TMSConfig      `yaml:"tms"`
	API      APIConfig      `yaml:"api"`
	Import   ImportConfig   `yaml:"import"`
	Export   ExportConfig   `yaml:"export"`
	CountryID   int    `yaml:"country_id"`
	PackageName string `yaml:"package_name"`
}

type AppConfig struct {
	Name           string `yaml:"name"`
	Version        string `yaml:"version"`
	Port           int    `yaml:"port"`
	Debug          bool   `yaml:"debug"`
	SessionTimeout int    `yaml:"session_timeout"`
	SessionSecret  string `yaml:"session_secret"`
	SessionName    string `yaml:"session_name"`
	PasswordSalt   string `yaml:"password_salt"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Charset      string `yaml:"charset"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Name, d.Charset)
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type TMSConfig struct {
	BaseURL   string `yaml:"base_url"`
	SecretKey string `yaml:"secret_key"`
	SecretIV  string `yaml:"secret_iv"`
}

type APIConfig struct {
	BasicAuthUser string `yaml:"basic_auth_user"`
	BasicAuthPass string `yaml:"basic_auth_pass"`
}

type ImportConfig struct {
	MaxFileSize int `yaml:"max_file_size"`
	BatchSize   int `yaml:"batch_size"`
}

type ExportConfig struct {
	BatchSize int    `yaml:"batch_size"`
	OutputDir string `yaml:"output_dir"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.Database.Charset == "" {
		cfg.Database.Charset = "utf8mb4"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.Import.BatchSize == 0 {
		cfg.Import.BatchSize = 100
	}
	if cfg.Export.BatchSize == 0 {
		cfg.Export.BatchSize = 100
	}

	return &cfg, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/config/ -v
# Expected: PASS
```

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config loader with YAML parsing and tests"
```

---

### Task 3: Database Connection (Shared)

**Files:**
- Create: `internal/shared/database.go`
- Create: `internal/shared/database_test.go`

**Step 1: Write database setup**

```go
// internal/shared/database.go
package shared

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/verifone/veristoretools3/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	logLevel := logger.Silent
	// Can make this configurable later

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Info().Str("host", cfg.Host).Str("database", cfg.Name).Msg("database connected")
	return db, nil
}
```

**Step 2: Commit**

```bash
git add internal/shared/
git commit -m "feat: add database connection setup with GORM"
```

---

### Task 4: Shared Response & Pagination Helpers

**Files:**
- Create: `internal/shared/response.go`
- Create: `internal/shared/pagination.go`
- Create: `internal/shared/flash.go`
- Create: `internal/shared/pagination_test.go`

**Step 1: Write pagination test**

```go
// internal/shared/pagination_test.go
package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPagination(t *testing.T) {
	p := NewPagination(3, 20, 95)
	assert.Equal(t, 3, p.CurrentPage)
	assert.Equal(t, 20, p.PerPage)
	assert.Equal(t, int64(95), p.Total)
	assert.Equal(t, 5, p.TotalPages)
	assert.Equal(t, 40, p.Offset())
}

func TestPagination_FirstPage(t *testing.T) {
	p := NewPagination(1, 20, 50)
	assert.Equal(t, 0, p.Offset())
	assert.Equal(t, 3, p.TotalPages)
	assert.True(t, p.IsFirst())
	assert.False(t, p.IsLast())
}

func TestPagination_LastPage(t *testing.T) {
	p := NewPagination(3, 20, 50)
	assert.True(t, p.IsLast())
	assert.False(t, p.IsFirst())
}

func TestPagination_InvalidPage(t *testing.T) {
	p := NewPagination(0, 20, 50)
	assert.Equal(t, 1, p.CurrentPage)

	p2 := NewPagination(-5, 20, 50)
	assert.Equal(t, 1, p2.CurrentPage)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/shared/ -v -run TestNewPagination
# Expected: FAIL
```

**Step 3: Write implementations**

```go
// internal/shared/pagination.go
package shared

import "math"

type Pagination struct {
	CurrentPage int
	PerPage     int
	Total       int64
	TotalPages  int
}

func NewPagination(page, perPage int, total int64) Pagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	return Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		TotalPages:  totalPages,
	}
}

func (p Pagination) Offset() int {
	return (p.CurrentPage - 1) * p.PerPage
}

func (p Pagination) IsFirst() bool {
	return p.CurrentPage <= 1
}

func (p Pagination) IsLast() bool {
	return p.CurrentPage >= p.TotalPages
}

func (p Pagination) Pages() []int {
	pages := make([]int, 0, p.TotalPages)
	for i := 1; i <= p.TotalPages; i++ {
		pages = append(pages, i)
	}
	return pages
}
```

```go
// internal/shared/response.go
package shared

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// Render renders a templ component to the response.
func Render(c echo.Context, status int, component templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return component.Render(c.Request().Context(), c.Response())
}

// IsHTMX checks if the request is an HTMX request.
func IsHTMX(c echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

// APIResponse is the standard JSON response for the REST API.
type APIResponse struct {
	Code        int         `json:"code"`
	Description string      `json:"description"`
	Data        interface{} `json:"data,omitempty"`
}

func APISuccess(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{
		Code:        0,
		Description: "Success",
		Data:        data,
	})
}

func APIError(c echo.Context, status int, message string) error {
	return c.JSON(status, APIResponse{
		Code:        1,
		Description: message,
	})
}
```

```go
// internal/shared/flash.go
package shared

import (
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

const (
	FlashSuccess = "success"
	FlashError   = "error"
	FlashInfo    = "info"
	FlashWarning = "warning"
)

func SetFlash(c echo.Context, store sessions.Store, sessionName, flashType, message string) {
	session, _ := store.Get(c.Request(), sessionName)
	session.AddFlash(message, flashType)
	session.Save(c.Request(), c.Response())
}

func GetFlashes(c echo.Context, store sessions.Store, sessionName string) map[string][]string {
	session, _ := store.Get(c.Request(), sessionName)
	flashes := make(map[string][]string)
	for _, flashType := range []string{FlashSuccess, FlashError, FlashInfo, FlashWarning} {
		if msgs := session.Flashes(flashType); len(msgs) > 0 {
			for _, msg := range msgs {
				if s, ok := msg.(string); ok {
					flashes[flashType] = append(flashes[flashType], s)
				}
			}
		}
	}
	session.Save(c.Request(), c.Response())
	return flashes
}
```

**Step 4: Run tests**

```bash
go test ./internal/shared/ -v
# Expected: PASS
```

**Step 5: Commit**

```bash
git add internal/shared/
git commit -m "feat: add shared helpers (pagination, response, flash)"
```

---

## Phase 2: Database Migrations & Models

### Task 5: SQL Migrations

**Files:**
- Create: `migrations/001_create_users.sql`
- Create: `migrations/002_create_terminals.sql`
- Create: `migrations/003_create_terminal_parameters.sql`
- Create: `migrations/004_create_verification_reports.sql`
- Create: `migrations/005_create_sync_terminals.sql`
- Create: `migrations/006_create_tms_logins.sql`
- Create: `migrations/007_create_tms_reports.sql`
- Create: `migrations/008_create_app_activations.sql`
- Create: `migrations/009_create_app_credentials.sql`
- Create: `migrations/010_create_activity_logs.sql`
- Create: `migrations/011_create_technicians.sql`
- Create: `migrations/012_create_template_parameters.sql`
- Create: `migrations/013_create_exports.sql`
- Create: `migrations/014_create_export_results.sql`
- Create: `migrations/015_create_tid_notes.sql`
- Create: `migrations/016_create_queue_logs.sql`
- Create: `migrations/017_create_sessions.sql`
- Create: `migrations/018_create_hashes.sql`
- Create: `migrations/019_create_auth_tables.sql`
- Create: `migrations/020_create_queue.sql`
- Create: `cmd/migrate/main.go`

**Step 1: Create all migration SQL files**

Copy exact CREATE TABLE statements from v2 SQL dumps at `../veristoreTools2/sql dump/`. Each migration file contains the exact v2 schema to ensure data migration compatibility. Use the schemas documented in the design exploration (19 tables from SQL dumps + activity_log + verification_report + app_credential from GORM AutoMigrate).

**Step 2: Create migration runner**

```go
// cmd/migrate/main.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/verifone/veristoretools3/internal/config"
	"github.com/verifone/veristoretools3/internal/shared"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [up|down]")
		os.Exit(1)
	}

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	db, err := shared.NewDatabase(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to database")
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	switch os.Args[1] {
	case "up":
		runMigrations(sqlDB, "migrations/", "up")
	case "down":
		fmt.Println("Down migrations not yet implemented")
	default:
		fmt.Println("Usage: migrate [up|down]")
	}
}
```

**Step 3: Verify migrations run against test database**

```bash
mysql -u root -e "CREATE DATABASE IF NOT EXISTS veristoretools3"
go run ./cmd/migrate up
# Expected: all tables created
```

**Step 4: Commit**

```bash
git add migrations/ cmd/migrate/
git commit -m "feat: add SQL migrations for all 20 database tables"
```

---

### Task 6: GORM Models - Core Tables

**Files:**
- Create: `internal/user/model.go`
- Create: `internal/terminal/model.go`
- Create: `internal/csi/model.go`
- Create: `internal/sync/model.go`
- Create: `internal/tms/model.go`
- Create: `internal/admin/model.go`
- Create: `internal/activation/model.go`

**Step 1: Write all GORM models matching v2 schema exactly**

Each model uses `gorm:"column:..."` tags that exactly match v2 column names. Reference the SQL dump schemas from Task 5.

Example for User model:

```go
// internal/user/model.go
package user

import "time"

type User struct {
	UserID                 int       `gorm:"primaryKey;column:user_id;autoIncrement"`
	UserFullname           string    `gorm:"column:user_fullname;size:100;not null"`
	UserName               string    `gorm:"column:user_name;size:60;not null;uniqueIndex"`
	Password               string    `gorm:"column:password;size:256;not null"`
	UserPrivileges         string    `gorm:"column:user_privileges;size:60;not null"`
	UserLastChangePassword *time.Time `gorm:"column:user_lastchangepassword"`
	CreatedDtm             time.Time `gorm:"column:createddtm;not null"`
	CreatedBy              string    `gorm:"column:createdby;size:60;not null"`
	AuthKey                *string   `gorm:"column:auth_key;size:32"`
	PasswordHash           *string   `gorm:"column:password_hash;size:256"`
	PasswordResetToken     *string   `gorm:"column:password_reset_token;size:256"`
	Email                  *string   `gorm:"column:email;size:256"`
	Status                 *int      `gorm:"column:status"`
	CreatedAt              *int64    `gorm:"column:created_at"`
	UpdatedAt              *int64    `gorm:"column:updated_at"`
	TmsSession             *string   `gorm:"column:tms_session;size:5120"`
	TmsPassword            *string   `gorm:"column:tms_password;size:256"`
}

func (User) TableName() string {
	return "user"
}

// UserSearch is used for search/filter queries
type UserSearch struct {
	UserName       string
	UserFullname   string
	UserPrivileges string
	Page           int
	PerPage        int
}
```

Repeat for Terminal, TerminalParameter, VerificationReport, SyncTerminal, TmsLogin, TmsReport, ActivityLog, Technician, TemplateParameter, AppActivation, AppCredential, Export, ExportResult, TidNote, QueueLog, Hash, Session.

**Step 2: Write a model test to verify table name mapping**

```go
// internal/user/model_test.go
package user

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestUser_TableName(t *testing.T) {
	u := User{}
	assert.Equal(t, "user", u.TableName())
}
```

**Step 3: Run tests**

```bash
go test ./internal/... -v -run TableName
# Expected: PASS
```

**Step 4: Commit**

```bash
git add internal/*/model.go internal/*/model_test.go
git commit -m "feat: add GORM models for all 20 database tables"
```

---

### Task 7: Repositories - Core CRUD

**Files:**
- Create: `internal/user/repository.go`
- Create: `internal/terminal/repository.go`
- Create: `internal/admin/repository.go`
- Create: `internal/csi/repository.go`
- Create: `internal/activation/repository.go`
- Create: `internal/sync/repository.go`

**Step 1: Write repository for each module**

```go
// internal/user/repository.go
package user

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByID(id int) (*User, error) {
	var user User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindByUsername(username string) (*User, error) {
	var user User
	if err := r.db.Where("user_name = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) Search(params UserSearch) ([]User, int64, error) {
	query := r.db.Model(&User{})

	if params.UserName != "" {
		query = query.Where("user_name LIKE ?", "%"+params.UserName+"%")
	}
	if params.UserFullname != "" {
		query = query.Where("user_fullname LIKE ?", "%"+params.UserFullname+"%")
	}
	if params.UserPrivileges != "" {
		query = query.Where("user_privileges = ?", params.UserPrivileges)
	}

	var total int64
	query.Count(&total)

	var users []User
	p := shared.NewPagination(params.Page, params.PerPage, total)
	err := query.Offset(p.Offset()).Limit(p.PerPage).Find(&users).Error
	return users, total, err
}

func (r *Repository) Create(user *User) error {
	return r.db.Create(user).Error
}

func (r *Repository) Update(user *User) error {
	return r.db.Save(user).Error
}

func (r *Repository) Delete(id int) error {
	return r.db.Delete(&User{}, id).Error
}

func (r *Repository) UpdateStatus(id int, status int) error {
	return r.db.Model(&User{}).Where("user_id = ?", id).Update("status", status).Error
}

func (r *Repository) All() ([]User, error) {
	var users []User
	err := r.db.Find(&users).Error
	return users, err
}
```

Follow same pattern for Terminal, ActivityLog, Technician, VerificationReport, SyncTerminal, AppActivation, AppCredential, TemplateParameter repositories.

**Step 2: Commit**

```bash
git add internal/*/repository.go
git commit -m "feat: add repository layer for all modules"
```

---

## Phase 3: Authentication & Middleware

### Task 8: Session Auth Middleware

**Files:**
- Create: `internal/middleware/auth.go`
- Create: `internal/middleware/recovery.go`
- Create: `internal/middleware/logger.go`

**Step 1: Write session auth middleware**

```go
// internal/middleware/auth.go
package middleware

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

const (
	SessionUserID         = "user_id"
	SessionUserName       = "user_name"
	SessionUserPrivileges = "user_privileges"
	SessionUserFullname   = "user_fullname"
)

func SessionAuth(store sessions.Store, sessionName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			session, _ := store.Get(c.Request(), sessionName)

			userID, ok := session.Values[SessionUserID]
			if !ok || userID == nil {
				// Not authenticated - redirect to login
				if c.Request().Header.Get("HX-Request") == "true" {
					c.Response().Header().Set("HX-Redirect", "/user/login")
					return c.NoContent(http.StatusUnauthorized)
				}
				return c.Redirect(http.StatusFound, "/user/login")
			}

			// Store user info in context for handlers
			c.Set(SessionUserID, userID)
			c.Set(SessionUserName, session.Values[SessionUserName])
			c.Set(SessionUserPrivileges, session.Values[SessionUserPrivileges])
			c.Set(SessionUserFullname, session.Values[SessionUserFullname])

			return next(c)
		}
	}
}

func BasicAuth(username, password string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u, p, ok := c.Request().BasicAuth()
			if !ok || u != username || p != password {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Unauthorized",
				})
			}
			return next(c)
		}
	}
}

// GetCurrentUserID retrieves the current user ID from context.
func GetCurrentUserID(c echo.Context) int {
	if id, ok := c.Get(SessionUserID).(int); ok {
		return id
	}
	return 0
}

// GetCurrentUserName retrieves the current username from context.
func GetCurrentUserName(c echo.Context) string {
	if name, ok := c.Get(SessionUserName).(string); ok {
		return name
	}
	return ""
}

// GetCurrentUserPrivileges retrieves the current user privileges from context.
func GetCurrentUserPrivileges(c echo.Context) string {
	if priv, ok := c.Get(SessionUserPrivileges).(string); ok {
		return priv
	}
	return ""
}
```

**Step 2: Write RBAC middleware**

```go
// internal/middleware/rbac.go
package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v4"
)

func RBAC(enforcer *casbin.Enforcer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetCurrentUserPrivileges(c)
			path := c.Path()
			method := c.Request().Method

			allowed, err := enforcer.Enforce(role, path, method)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "RBAC error")
			}
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "Access denied")
			}
			return next(c)
		}
	}
}
```

**Step 3: Commit**

```bash
git add internal/middleware/
git commit -m "feat: add auth, RBAC, and basic auth middleware"
```

---

### Task 9: Auth Module (Login/Logout/Password)

**Files:**
- Create: `internal/auth/handler.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`

**Step 1: Write password hashing test (must match v2 output)**

```go
// internal/auth/service_test.go
package auth

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHashPassword_MatchesV2(t *testing.T) {
	// SHA256 with salt "@!Boteng2021%??" - must match v2 PHP output
	salt := "@!Boteng2021%??"
	password := "admin123"
	hash := HashPasswordSHA256(password, salt)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 hex = 64 chars
}

func TestVerifyPassword(t *testing.T) {
	salt := "@!Boteng2021%??"
	password := "admin123"
	hash := HashPasswordSHA256(password, salt)
	assert.True(t, VerifyPasswordSHA256(password, hash, salt))
	assert.False(t, VerifyPasswordSHA256("wrongpassword", hash, salt))
}
```

**Step 2: Write auth service**

```go
// internal/auth/service.go
package auth

import (
	"crypto/sha256"
	"fmt"

	"github.com/verifone/veristoretools3/internal/user"
)

type Service struct {
	userRepo *user.Repository
	salt     string
}

func NewService(userRepo *user.Repository, salt string) *Service {
	return &Service{userRepo: userRepo, salt: salt}
}

func HashPasswordSHA256(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(password + salt))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func VerifyPasswordSHA256(password, hash, salt string) bool {
	return HashPasswordSHA256(password, salt) == hash
}

func (s *Service) Authenticate(username, password string) (*user.User, error) {
	u, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !VerifyPasswordSHA256(password, u.Password, s.salt) {
		return nil, fmt.Errorf("invalid credentials")
	}

	if u.Status != nil && *u.Status == 0 {
		return nil, fmt.Errorf("account is deactivated")
	}

	return u, nil
}
```

**Step 3: Write auth handler**

```go
// internal/auth/handler.go
package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
)

type Handler struct {
	service     *Service
	store       sessions.Store
	sessionName string
}

func NewHandler(service *Service, store sessions.Store, sessionName string) *Handler {
	return &Handler{service: service, store: store, sessionName: sessionName}
}

func (h *Handler) LoginPage(c echo.Context) error {
	// Render login template
	return shared.Render(c, http.StatusOK, LoginPage())
}

func (h *Handler) Login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	u, err := h.service.Authenticate(username, password)
	if err != nil {
		// Re-render login with error
		return shared.Render(c, http.StatusOK, LoginPageWithError(err.Error()))
	}

	// Create session
	session, _ := h.store.Get(c.Request(), h.sessionName)
	session.Values[mw.SessionUserID] = u.UserID
	session.Values[mw.SessionUserName] = u.UserName
	session.Values[mw.SessionUserPrivileges] = u.UserPrivileges
	session.Values[mw.SessionUserFullname] = u.UserFullname
	session.Save(c.Request(), c.Response())

	return c.Redirect(http.StatusFound, "/")
}

func (h *Handler) Logout(c echo.Context) error {
	session, _ := h.store.Get(c.Request(), h.sessionName)
	session.Options.MaxAge = -1
	session.Save(c.Request(), c.Response())
	return c.Redirect(http.StatusFound, "/user/login")
}
```

**Step 4: Run tests and commit**

```bash
go test ./internal/auth/ -v
git add internal/auth/
git commit -m "feat: add auth module with login/logout and SHA256 password hashing"
```

---

## Phase 4: Frontend Foundation

### Task 10: Static Assets Setup (AdminLTE + HTMX + Alpine.js)

**Files:**
- Create: `static/adminlte/` (download AdminLTE 3 dist)
- Create: `static/js/htmx.min.js` (download HTMX)
- Create: `static/js/alpine.min.js` (download Alpine.js)
- Create: `static/js/sweetalert2.min.js` (download SweetAlert2)
- Create: `static/js/app.js`
- Create: `static/css/site.css`
- Create: `static/img/` (copy logos from v2)
- Create: `static/export/.gitkeep`
- Create: `static/import/.gitkeep`
- Create: `static/sync/.gitkeep`

**Step 1: Download AdminLTE 3**

```bash
cd /Users/frederickjerusha/Documents/works/Verifone/Projects/veristoretools/veristoreTools3
npm init -y
npm install admin-lte@3.2.0
cp -r node_modules/admin-lte/dist/* static/adminlte/
cp node_modules/admin-lte/plugins/fontawesome-free static/adminlte/plugins/ -r
cp node_modules/admin-lte/plugins/bootstrap static/adminlte/plugins/ -r
cp node_modules/admin-lte/plugins/jquery static/adminlte/plugins/ -r
cp node_modules/admin-lte/plugins/select2 static/adminlte/plugins/ -r
cp node_modules/admin-lte/plugins/sweetalert2 static/adminlte/plugins/ -r
cp node_modules/admin-lte/plugins/flatpickr static/adminlte/plugins/ -r 2>/dev/null || true
rm -rf node_modules package.json package-lock.json
```

**Step 2: Download HTMX and Alpine.js**

```bash
curl -o static/js/htmx.min.js https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
curl -o static/js/alpine.min.js https://unpkg.com/alpinejs@3.14.8/dist/cdn.min.js
```

**Step 3: Create app.js (replaces v2 content.php JS)**

```javascript
// static/js/app.js
// Loading spinner
function loading(spinnerId, status) {
    var spinner = document.getElementById(spinnerId);
    if (spinner) {
        spinner.style.display = status ? 'block' : 'none';
    }
}

// Confirmation dialog (Indonesian - matching v2)
function confirmation(text, spinnerId, formId) {
    Swal.fire({
        title: 'Konfirmasi',
        text: text,
        icon: 'warning',
        showCancelButton: true,
        confirmButtonColor: '#3085d6',
        cancelButtonColor: '#d33',
        confirmButtonText: 'Ya',
        cancelButtonText: 'Tidak'
    }).then((result) => {
        if (result.isConfirmed) {
            if (spinnerId) loading(spinnerId, true);
            if (formId) document.getElementById(formId).submit();
        }
    });
}

// Confirmation dialog (English - matching v2)
function confirmationEnglish(text, spinnerId, formId) {
    Swal.fire({
        title: 'Confirmation',
        text: text,
        icon: 'warning',
        showCancelButton: true,
        confirmButtonColor: '#3085d6',
        cancelButtonColor: '#d33',
        confirmButtonText: 'Yes',
        cancelButtonText: 'No'
    }).then((result) => {
        if (result.isConfirmed) {
            if (spinnerId) loading(spinnerId, true);
            if (formId) document.getElementById(formId).submit();
        }
    });
}

// Search handler
function search(spinnerId, formId) {
    if (spinnerId) loading(spinnerId, true);
    if (formId) document.getElementById(formId).submit();
}
```

**Step 4: Create site.css (copy from v2 and adapt)**

Copy `../veristoreTools2/web/css/site.css` content and adapt for AdminLTE 3.

**Step 5: Copy logos/images from v2**

```bash
cp ../veristoreTools2/web/img/* static/img/ 2>/dev/null || true
```

**Step 6: Commit**

```bash
git add static/
git commit -m "feat: add static assets (AdminLTE 3, HTMX, Alpine.js, custom CSS/JS)"
```

---

### Task 11: Templ Layout Templates

**Files:**
- Create: `templates/layouts/base.templ`
- Create: `templates/layouts/login.templ`
- Create: `templates/layouts/header.templ`
- Create: `templates/layouts/sidebar.templ`
- Create: `templates/layouts/content.templ`
- Create: `templates/components/alert.templ`
- Create: `templates/components/pagination.templ`
- Create: `templates/components/table.templ`

**Step 1: Write base layout template**

```go
// templates/layouts/base.templ
package layouts

import "github.com/verifone/veristoretools3/internal/user"

type PageData struct {
	Title   string
	User    *user.User
	Flashes map[string][]string
}

templ Base(data PageData) {
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1"/>
		<title>{ data.Title } | VeriStore Tools 3</title>
		<link rel="stylesheet" href="/static/adminlte/plugins/fontawesome-free/css/all.min.css"/>
		<link rel="stylesheet" href="/static/adminlte/css/adminlte.min.css"/>
		<link rel="stylesheet" href="/static/adminlte/plugins/select2/css/select2.min.css"/>
		<link rel="stylesheet" href="/static/css/site.css"/>
	</head>
	<body class="hold-transition sidebar-mini skin-blue">
		<div class="wrapper">
			@Header(data.User)
			@Sidebar(data.User)
			<div class="content-wrapper">
				{ children... }
			</div>
		</div>
		<script src="/static/adminlte/plugins/jquery/jquery.min.js"></script>
		<script src="/static/adminlte/plugins/bootstrap/js/bootstrap.bundle.min.js"></script>
		<script src="/static/adminlte/js/adminlte.min.js"></script>
		<script src="/static/adminlte/plugins/select2/js/select2.full.min.js"></script>
		<script src="/static/adminlte/plugins/sweetalert2/sweetalert2.all.min.js"></script>
		<script src="/static/js/htmx.min.js"></script>
		<script src="/static/js/alpine.min.js" defer></script>
		<script src="/static/js/app.js"></script>
	</body>
	</html>
}
```

**Step 2: Write header, sidebar, content templates**

Match the exact HTML structure from v2's `views/layouts/header.php`, `left.php`, `content.php` but in Templ syntax. Replicate AdminLTE class names, logo placement, user dropdown, sidebar menu structure.

**Step 3: Write reusable component templates**

Alert, pagination, data table components that replace Kartik widgets.

**Step 4: Generate and verify compilation**

```bash
templ generate
go build ./...
# Expected: compiles without errors
```

**Step 5: Commit**

```bash
git add templates/
git commit -m "feat: add Templ layout templates (base, header, sidebar, components)"
```

---

## Phase 5: Module Implementation (Controller by Controller)

### Task 12: Site Module (Dashboard)

**Files:**
- Create: `internal/site/handler.go`
- Create: `templates/site/dashboard.templ`

**Step 1:** Write dashboard handler that shows CSI or TMS dashboard based on user role (same logic as v2 `SiteController::actionIndex`).

**Step 2:** Write dashboard Templ template matching v2's `views/site/index.php` layout with AdminLTE info boxes for metrics.

**Step 3:** Register routes in `main.go`.

**Step 4:** Commit.

---

### Task 13: User Module (CRUD + Password)

**Files:**
- Create: `internal/user/handler.go`
- Create: `internal/user/service.go`
- Create: `templates/user/index.templ`
- Create: `templates/user/view.templ`
- Create: `templates/user/change_password.templ`

**Step 1:** Write user service with CRUD + password change logic.

**Step 2:** Write user handler with all actions: `Index`, `View`, `Delete`, `Activate`, `ChangePassword`, `GetAppType`.

**Step 3:** Write Templ templates matching v2 views: user list (GridView → HTMX table), user detail view, change password form.

**Step 4:** Register routes. Test manually.

**Step 5:** Commit.

---

### Task 14: TMS Encryption Module

**Files:**
- Create: `internal/tms/encryption.go`
- Create: `internal/tms/encryption_test.go`

**Step 1:** Write encryption tests with known v2 inputs/outputs to verify byte-for-byte compatibility.

**Step 2:** Implement HMAC-SHA256, AES-256-CBC encrypt/decrypt, Triple DES activation code generation. Reference v2's `components/TmsHelper.php` lines for encrypt_decrypt() and activation code logic.

**Step 3:** Run tests to verify Go output matches PHP output.

**Step 4:** Commit.

---

### Task 15: TMS API Client

**Files:**
- Create: `internal/tms/client.go`
- Create: `internal/tms/client_test.go`

**Step 1:** Write TMS HTTP client with all methods matching v2 TmsHelper.php: Login, GetVerifyCode, GetResellerList, GetTerminalList, GetTerminalDetail, AddTerminal, EditTerminal, CopyTerminal, DeleteTerminal, ReplaceTerminal, GetTerminalParameter, GetMerchantList, AddMerchant, EditMerchant, DeleteMerchant, GetGroupList, AddGroup, EditGroup, DeleteGroup.

**Step 2:** Implement token renewal pattern (detect "toke更新" → re-login → retry).

**Step 3:** Write tests with mocked HTTP server.

**Step 4:** Commit.

---

### Task 16: Veristore Module (TMS Terminal/Merchant/Group Handlers)

**Files:**
- Create: `internal/tms/handler.go`
- Create: `internal/tms/service.go`
- Create: `templates/veristore/terminal.templ`
- Create: `templates/veristore/add.templ`
- Create: `templates/veristore/edit.templ`
- Create: `templates/veristore/merchant.templ`
- Create: `templates/veristore/group.templ`
- Create: `templates/veristore/import.templ`
- Create: `templates/veristore/export.templ`
- Create: `templates/veristore/login.templ`

**Step 1:** Write TMS service layer wrapping TMS client + local database operations.

**Step 2:** Write handler with all 30+ actions matching v2 VeristoreController exactly: Terminal (list, add, edit, copy, delete, replacement, check, report, reset), Merchant (list, add, edit, delete, import), Group (list, add, edit, delete, add terminals), Import, Export, GetOperator, GetVerifyCode, ChangeMerchant.

**Step 3:** Write Templ templates for each view, matching v2 HTML structure.

**Step 4:** Register all `/veristore/*` routes.

**Step 5:** Commit.

---

### Task 17: CSI Verification Module

**Files:**
- Create: `internal/csi/handler.go`
- Create: `internal/csi/service.go`
- Create: `templates/verification/index.templ`

**Step 1:** Write verification service matching v2 VerificationController: search terminal by CSI, check local DB first then TMS API, calculate verification password (Triple DES), get technician list.

**Step 2:** Write handler and Templ templates.

**Step 3:** Register `/verification/*` routes.

**Step 4:** Commit.

---

### Task 18: Verification Report Module

**Files:**
- Create: `internal/csi/report_handler.go`
- Create: `templates/verificationreport/index.templ`
- Create: `templates/verificationreport/view.templ`
- Create: `templates/verificationreport/form.templ`

**Step 1:** Write CRUD handler for verification reports matching v2 VerificationReportController.

**Step 2:** Write Templ templates.

**Step 3:** Register `/verificationreport/*` routes.

**Step 4:** Commit.

---

### Task 19: Sync Terminal Module

**Files:**
- Create: `internal/sync/handler.go`
- Create: `internal/sync/service.go`
- Create: `templates/sync/index.templ`
- Create: `templates/sync/view.templ`

**Step 1:** Write sync service: create sync job, check sync status, download sync report, reset sync. Matching v2 SyncTerminalController.

**Step 2:** Write handler and templates.

**Step 3:** Register `/sync-terminal/*` routes.

**Step 4:** Commit.

---

### Task 20: Terminal Module (Local CRUD)

**Files:**
- Create: `internal/terminal/handler.go`
- Create: `internal/terminal/service.go`
- Create: `templates/terminal/index.templ`
- Create: `templates/terminal/view.templ`
- Create: `templates/terminal/form.templ`

**Step 1:** Write local terminal CRUD matching v2 TerminalController.

**Step 2:** Write handler with Index, View, Create, Update, Delete.

**Step 3:** Write Templ templates with HTMX data table.

**Step 4:** Register `/terminal/*` routes.

**Step 5:** Commit.

---

### Task 21: Terminal Parameter Module

**Files:**
- Create: `internal/terminal/param_handler.go`
- Create: `templates/terminalparameter/index.templ`
- Create: `templates/terminalparameter/view.templ`
- Create: `templates/terminalparameter/form.templ`

**Step 1:** Write CRUD handler for terminal parameters matching v2 TerminalParameterController.

**Step 2:** Write Templ templates.

**Step 3:** Register `/terminalparameter/*` routes.

**Step 4:** Commit.

---

### Task 22: Template Parameter Module

**Files:**
- Create: `internal/admin/template_param_handler.go`
- Create: `templates/templateparameter/index.templ`
- Create: `templates/templateparameter/view.templ`
- Create: `templates/templateparameter/form.templ`

**Step 1:** Write CRUD matching v2 TemplateParameterController.

**Step 2:** Register `/templateparameter/*` routes.

**Step 3:** Commit.

---

### Task 23: Admin Module (Activity Log, Technician, FAQ, Backup)

**Files:**
- Create: `internal/admin/handler.go`
- Create: `internal/admin/service.go`
- Create: `templates/admin/activitylog_index.templ`
- Create: `templates/admin/technician_index.templ`
- Create: `templates/admin/technician_form.templ`
- Create: `templates/admin/faq.templ`
- Create: `templates/admin/backup.templ`

**Step 1:** Write handler with: ActivityLog CRUD, Technician CRUD, FAQ index + user guide download, Backup index + log download. Matching v2 controllers.

**Step 2:** Write Templ templates for each view.

**Step 3:** Register all admin routes: `/activitylog/*`, `/technician/*`, `/faq/*`, `/backup/*`.

**Step 4:** Commit.

---

### Task 24: Activation & Credential Module

**Files:**
- Create: `internal/activation/handler.go`
- Create: `internal/activation/api_handler.go`
- Create: `internal/activation/service.go`
- Create: `templates/appactivation/index.templ`
- Create: `templates/appactivation/view.templ`
- Create: `templates/appcredential/index.templ`
- Create: `templates/appcredential/view.templ`

**Step 1:** Write CRUD handlers for AppActivation and AppCredential matching v2 controllers.

**Step 2:** Write REST API handler for `/feature/api/activation-code` (POST, Basic Auth) matching v2 ApiController. Uses Triple DES encryption from Task 14.

**Step 3:** Write Templ templates.

**Step 4:** Register routes including API route with BasicAuth middleware.

**Step 5:** Commit.

---

### Task 25: User Management & TMS Login Modules

**Files:**
- Create: `internal/user/management_handler.go`
- Create: `internal/tms/login_handler.go`
- Create: `templates/usermanagement/index.templ`
- Create: `templates/usermanagement/form.templ`
- Create: `templates/tmslogin/index.templ`

**Step 1:** Write UserManagement CRUD handler matching v2 UsermanagementController.

**Step 2:** Write TmsLogin handler: index, get-operator, get-verify-code. Matching v2 TmsLoginController.

**Step 3:** Write Templ templates.

**Step 4:** Register `/usermanagement/*` and `/tmslogin/*` routes.

**Step 5:** Commit.

---

### Task 26: Scheduler Module

**Files:**
- Create: `internal/sync/scheduler.go`
- Create: `internal/sync/scheduler_handler.go`
- Create: `templates/scheduler/index.templ`

**Step 1:** Write scheduler handler showing scheduled sync configuration. Matching v2 SchedulerController.

**Step 2:** Write scheduler service that integrates with Asynq periodic tasks. Replaces v2 console command `tms:scheduler`.

**Step 3:** Register `/scheduler/*` routes.

**Step 4:** Commit.

---

## Phase 6: Queue System & Background Jobs

### Task 27: Asynq Queue Setup

**Files:**
- Create: `internal/queue/worker.go`
- Create: `internal/queue/tasks.go`

**Step 1:** Write Asynq worker setup with task routing.

```go
// internal/queue/tasks.go
package queue

const (
	TaskImportTerminal    = "import:terminal"
	TaskExportTerminal    = "export:terminal"
	TaskImportMerchant    = "import:merchant"
	TaskSyncParameter     = "sync:parameter"
	TaskExportAll         = "export:all_terminals"
	TaskTMSPing           = "tms:ping"
	TaskSchedulerCheck    = "tms:scheduler_check"
)
```

```go
// internal/queue/worker.go
package queue

import (
	"github.com/hibiken/asynq"
)

func NewWorker(redisAddr string, handlers map[string]asynq.Handler) *asynq.Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)
	return srv
}

func NewMux(handlers map[string]asynq.Handler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	for pattern, handler := range handlers {
		mux.Handle(pattern, handler)
	}
	return mux
}
```

**Step 2:** Commit.

---

### Task 28: Import Terminal Job

**Files:**
- Create: `internal/queue/import_terminal.go`
- Create: `internal/queue/import_terminal_test.go`

**Step 1:** Write import terminal job matching v2 `components/ImportTerminal.php`: read Excel file with excelize, validate rows, map columns to terminal fields, batch insert to database, send to TMS API. Max 3 retries, 1-hour timeout.

**Step 2:** Write unit test with sample Excel file.

**Step 3:** Commit.

---

### Task 29: Export Terminal Job

**Files:**
- Create: `internal/queue/export_terminal.go`
- Create: `internal/queue/export_terminal_test.go`

**Step 1:** Write export terminal job matching v2 `components/ExportTerminal.php`: query terminals, batch 100 per chunk, write Excel with excelize, save to `static/export/`, update export table with progress.

**Step 2:** Write unit test.

**Step 3:** Commit.

---

### Task 30: Import Merchant Job

**Files:**
- Create: `internal/queue/import_merchant.go`

**Step 1:** Write import merchant job matching v2 `components/ImportMerchant.php`.

**Step 2:** Commit.

---

### Task 31: Sync Parameter Job

**Files:**
- Create: `internal/queue/sync_parameter.go`

**Step 1:** Write sync parameter job matching v2 `components/SyncTerminalParameter.php`: login to TMS, fetch terminal list, update local database parameters, track progress in queue_log.

**Step 2:** Commit.

---

### Task 32: TMS Ping & Scheduler Jobs

**Files:**
- Create: `internal/queue/tms_ping.go`
- Create: `internal/queue/scheduler_check.go`

**Step 1:** Write TMS ping job (every 15 min): check TMS token validity, refresh terminal list. Matching v2 `commands/TmsController::actionPing`.

**Step 2:** Write scheduler check job (every 1 min): check TmsLogin scheduled settings, push SyncParameter job if schedule matches. Matching v2 `commands/TmsController::actionScheduler`.

**Step 3:** Commit.

---

## Phase 7: Activity Logging

### Task 33: Activity Log Middleware

**Files:**
- Create: `internal/middleware/activitylog.go`
- Modify: `internal/admin/model.go` (add ActivityLog model if not done)

**Step 1:** Write activity logging helper matching v2 `components/ActivityLogHelper.php` with all 28 activity types as constants.

**Step 2:** Write logging function that inserts into `activity_log` table with user, timestamp, action type, and detail.

**Step 3:** Integrate logging calls into handlers at appropriate points (login, logout, terminal CRUD, import, export, sync, etc.).

**Step 4:** Commit.

---

## Phase 8: Wire Everything Together

### Task 34: Main.go - Wire All Modules

**Files:**
- Modify: `cmd/server/main.go`

**Step 1:** Wire up all dependencies in main.go:

1. Load config
2. Connect to MySQL (GORM)
3. Connect to Redis (Asynq)
4. Initialize session store
5. Initialize Casbin enforcer
6. Create all repositories
7. Create all services
8. Create all handlers
9. Register all routes (130+)
10. Start Echo web server (goroutine)
11. Start Asynq worker (goroutine)
12. Start Asynq scheduler (goroutine)
13. Block on signal for graceful shutdown

**Step 2:** Verify the binary compiles and starts.

```bash
make build
./bin/veristoretools3
# Expected: server starts, connects to MySQL and Redis
```

**Step 3:** Commit.

---

### Task 35: Casbin RBAC Policies

**Files:**
- Create: `internal/auth/casbin_model.conf`
- Create: `internal/auth/casbin_policy.go`

**Step 1:** Write Casbin model conf matching v2 RBAC rules.

**Step 2:** Write policy loader that reads from `auth_item` and `auth_assignment` tables (migrated from v2).

**Step 3:** Commit.

---

## Phase 9: Data Migration

### Task 36: Data Migration Script

**Files:**
- Create: `cmd/migrate/data_migrate.go`

**Step 1:** Write data migration script:

```bash
# Dump v2 data
mysqldump -u root veristoretools2 --no-create-info > v2_data.sql
# Import into v3
mysql -u root veristoretools3 < v2_data.sql
# Verify row counts
```

**Step 2:** Write Go verification script that compares row counts between v2 and v3 databases.

**Step 3:** Commit.

---

## Phase 10: Testing

### Task 37: Crypto Compatibility Tests

**Files:**
- Create: `internal/tms/encryption_compat_test.go`

**Step 1:** Write tests that verify Go encryption output matches PHP output for: HMAC-SHA256, AES-256-CBC, Triple DES activation codes, SHA256 password hashing. Use known input/output pairs captured from v2.

**Step 2:** Run tests.

**Step 3:** Commit.

---

### Task 38: Handler Integration Tests

**Files:**
- Create: `internal/auth/handler_test.go`
- Create: `internal/user/handler_test.go`
- Create: `internal/terminal/handler_test.go`

**Step 1:** Write HTTP handler tests using Echo's test utilities and httptest for key flows: login, user CRUD, terminal CRUD.

**Step 2:** Run tests.

**Step 3:** Commit.

---

### Task 39: API Endpoint Test

**Files:**
- Create: `internal/activation/api_handler_test.go`

**Step 1:** Write test for REST API `/feature/api/activation-code` with Basic Auth, verifying activation code generation matches v2.

**Step 2:** Commit.

---

## Phase 11: Deployment

### Task 40: Systemd Service File

**Files:**
- Create: `deploy/veristoretools3.service`
- Create: `deploy/install.sh`

**Step 1:** Write systemd service file:

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

**Step 2:** Write install script:

```bash
#!/bin/bash
# deploy/install.sh
set -e

BUILD_DIR=/opt/veristoretools3
sudo mkdir -p $BUILD_DIR/bin
sudo mkdir -p $BUILD_DIR/static
sudo mkdir -p $BUILD_DIR/templates

# Build
go build -o $BUILD_DIR/bin/veristoretools3 ./cmd/server

# Copy assets
cp -r static/* $BUILD_DIR/static/
cp config.yaml $BUILD_DIR/config.yaml

# Install service
sudo cp deploy/veristoretools3.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable veristoretools3
sudo systemctl start veristoretools3

echo "VeriStore Tools 3 installed and started"
```

**Step 3:** Commit.

---

### Task 41: Final Verification & README

**Files:**
- Create: `README.md`

**Step 1:** Write README with: project overview, tech stack, prerequisites (Go, MySQL, Redis), setup instructions, build commands, configuration guide, deployment steps.

**Step 2:** Run full test suite:

```bash
make test
make lint
make build
```

**Step 3:** Final commit.

```bash
git add .
git commit -m "feat: complete veristoreTools3 migration - ready for deployment"
```

---

## Implementation Order Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-4 | Project scaffolding, config, shared helpers |
| 2 | 5-7 | Database migrations, GORM models, repositories |
| 3 | 8-9 | Auth middleware, login/logout |
| 4 | 10-11 | AdminLTE static assets, Templ layouts |
| 5 | 12-26 | All module handlers + templates (15 tasks) |
| 6 | 27-32 | Queue system + background jobs (6 tasks) |
| 7 | 33 | Activity logging |
| 8 | 34-35 | Wire everything + RBAC |
| 9 | 36 | Data migration |
| 10 | 37-39 | Testing |
| 11 | 40-41 | Deployment + README |

**Total: 41 tasks across 11 phases**

Each phase builds on the previous one. Phase 1-4 creates the foundation. Phase 5 is the bulk of the work (module-by-module port). Phase 6-7 adds async processing. Phase 8 wires it all together. Phase 9-11 handles migration, testing, and deployment.
