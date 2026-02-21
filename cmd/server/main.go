package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	stringadapter "github.com/casbin/casbin/v2/persist/string-adapter"
	"github.com/gorilla/sessions"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/verifone/veristoretools3/internal/activation"
	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/auth"
	"github.com/verifone/veristoretools3/internal/config"
	"github.com/verifone/veristoretools3/internal/csi"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/queue"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/site"
	syncpkg "github.com/verifone/veristoretools3/internal/sync"
	"github.com/verifone/veristoretools3/internal/terminal"
	"github.com/verifone/veristoretools3/internal/tms"
	"github.com/verifone/veristoretools3/internal/user"
)

func main() {
	// -----------------------------------------------------------------------
	// 1. Load configuration
	// -----------------------------------------------------------------------
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// -----------------------------------------------------------------------
	// 2. Initialize zerolog
	// -----------------------------------------------------------------------
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.App.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Info().Str("app", cfg.App.Name).Str("version", cfg.App.Version).Msg("starting application")

	// -----------------------------------------------------------------------
	// 3. Connect to MySQL via GORM
	// -----------------------------------------------------------------------
	db, err := shared.NewDatabase(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	// -----------------------------------------------------------------------
	// 4. Initialize gorilla/sessions cookie store
	// -----------------------------------------------------------------------
	sessionStore := sessions.NewCookieStore([]byte(cfg.App.SessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   cfg.App.SessionTimeout,
		HttpOnly: true,
		Secure:   !cfg.App.Debug,
		SameSite: http.SameSiteLaxMode,
	}
	sessionName := cfg.App.SessionName

	// -----------------------------------------------------------------------
	// 5. Initialize Casbin enforcer (embedded model and policy)
	// -----------------------------------------------------------------------
	casbinModel, err := model.NewModelFromString(auth.CasbinModel)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse embedded Casbin model")
	}
	policyAdapter := stringadapter.NewAdapter(auth.CasbinPolicy)
	enforcer, err := casbin.NewEnforcer(casbinModel, policyAdapter)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize Casbin enforcer")
	}

	// -----------------------------------------------------------------------
	// 6. Create TMS API client (TLS skip verify from config)
	// -----------------------------------------------------------------------
	tmsClient := tms.NewClient(cfg.TMS.BaseURL, cfg.TMS.APIBaseURL, db, cfg.TMS.SkipTLSVerify, cfg.TMS.AccessKey, cfg.TMS.AccessSecret)

	// -----------------------------------------------------------------------
	// 7. Create all repositories
	// -----------------------------------------------------------------------
	userRepo := user.NewRepository(db)
	terminalRepo := terminal.NewRepository(db)
	tmsRepo := tms.NewRepository(db)
	syncRepo := syncpkg.NewRepository(db)
	csiRepo := csi.NewRepository(db)
	adminRepo := admin.NewRepository(db)
	actRepo := activation.NewRepository(db)

	// -----------------------------------------------------------------------
	// 8. Create all services
	// -----------------------------------------------------------------------
	userService := user.NewService(userRepo, cfg.App.PasswordSalt)
	authService := auth.NewService(userRepo, cfg.App.PasswordSalt)
	terminalService := terminal.NewService(terminalRepo, db)
	tmsService := tms.NewService(tmsClient, db, cfg.TMS.ResellerList)
	syncService := syncpkg.NewService(syncRepo, db)
	csiService := csi.NewService(db, tmsService)

	// -----------------------------------------------------------------------
	// 9. Build Redis config and Asynq client (needed by handlers)
	// -----------------------------------------------------------------------
	redisCfg := queue.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	asynqClient := queue.NewClient(redisCfg)
	defer asynqClient.Close()

	// -----------------------------------------------------------------------
	// 10. Create all handlers
	// -----------------------------------------------------------------------
	appName := cfg.App.Name
	appVersion := cfg.App.Version

	authHandler := auth.NewHandler(authService, sessionStore, sessionName, appName, appVersion)
	authHandler.SetTmsSessionClearer(func(username string) error {
		return tmsService.ClearUserSession(username)
	})
	siteHandler := site.NewHandler(terminalRepo, syncRepo, csiRepo, adminRepo, sessionStore, sessionName, appName, appVersion)
	userHandler := user.NewHandler(userRepo, userService, sessionStore, sessionName, appName, appVersion)
	termHandler := terminal.NewHandler(terminalService, sessionStore, sessionName, appName, appVersion)
	paramHandler := terminal.NewParamHandler(terminalRepo, sessionStore, sessionName, appName, appVersion)
	tplParamHandler := admin.NewTemplateParamHandler(adminRepo, sessionStore, sessionName, appName, appVersion)
	tmsHandler := tms.NewHandler(tmsService, sessionStore, sessionName, appName, appVersion, adminRepo, asynqClient, cfg.PackageName)
	tmsLoginHandler := tms.NewLoginHandler(tmsRepo, tmsService, sessionStore, sessionName, appName, appVersion)
	verifyHandler := csi.NewHandler(csiService, sessionStore, sessionName, appName, appVersion)
	reportHandler := csi.NewReportHandler(csiRepo, sessionStore, sessionName, appName, appVersion)
	syncHandler := syncpkg.NewHandler(syncService, sessionStore, sessionName, appName, appVersion)
	schedulerHandler := syncpkg.NewSchedulerHandler(tmsRepo, sessionStore, sessionName, appName, appVersion)
	adminHandler := admin.NewHandler(adminRepo, sessionStore, sessionName, appName, appVersion)
	actHandler := activation.NewHandler(actRepo, sessionStore, sessionName, appName, appVersion)
	apiHandler := activation.NewAPIHandler(actRepo, db)

	// -----------------------------------------------------------------------
	// 10. Create Echo instance
	// -----------------------------------------------------------------------
	e := echo.New()
	e.HideBanner = true

	// -----------------------------------------------------------------------
	// 11. Register global middleware: Recovery, Logger
	// -----------------------------------------------------------------------
	e.Use(mw.Recovery())
	e.Use(mw.RequestLogger())

	// -----------------------------------------------------------------------
	// 12. Serve static files
	// -----------------------------------------------------------------------
	e.Static("/static", "static")

	// -----------------------------------------------------------------------
	// 13. Public routes (no auth required)
	// -----------------------------------------------------------------------
	e.GET("/user/login", authHandler.LoginPage)
	e.POST("/user/login", authHandler.Login)

	// -----------------------------------------------------------------------
	// 14. Protected routes (session auth + RBAC)
	// -----------------------------------------------------------------------
	protected := e.Group("", mw.SessionAuth(sessionStore, sessionName), mw.RBAC(enforcer))

	// Dashboard
	protected.GET("/", siteHandler.Dashboard)
	protected.GET("/user/logout", authHandler.Logout)

	// User
	protected.GET("/user/index", userHandler.Index)
	protected.GET("/user/view", userHandler.View)
	protected.GET("/user/create", userHandler.Create)
	protected.POST("/user/create", userHandler.Create)
	protected.POST("/user/delete", userHandler.Delete)
	protected.POST("/user/activate", userHandler.Activate)
	protected.GET("/user/change-password", userHandler.ChangePassword)
	protected.POST("/user/change-password", userHandler.ChangePassword)
	protected.GET("/user/get-app-type", userHandler.GetAppType)

	// Terminal (local)
	protected.GET("/terminal/index", termHandler.Index)
	protected.GET("/terminal/view", termHandler.View)
	protected.GET("/terminal/create", termHandler.Create)
	protected.POST("/terminal/create", termHandler.Create)
	protected.GET("/terminal/update", termHandler.Update)
	protected.POST("/terminal/update", termHandler.Update)
	protected.POST("/terminal/delete", termHandler.Delete)

	// Terminal Parameter
	protected.GET("/terminalparameter/index", paramHandler.Index)
	protected.GET("/terminalparameter/view", paramHandler.View)
	protected.GET("/terminalparameter/create", paramHandler.Create)
	protected.POST("/terminalparameter/create", paramHandler.Create)
	protected.GET("/terminalparameter/update", paramHandler.Update)
	protected.POST("/terminalparameter/update", paramHandler.Update)
	protected.POST("/terminalparameter/delete", paramHandler.Delete)

	// Template Parameter
	protected.GET("/templateparameter/index", tplParamHandler.Index)
	protected.GET("/templateparameter/view", tplParamHandler.View)
	protected.GET("/templateparameter/create", tplParamHandler.Create)
	protected.POST("/templateparameter/create", tplParamHandler.Create)
	protected.GET("/templateparameter/update", tplParamHandler.Update)
	protected.POST("/templateparameter/update", tplParamHandler.Update)
	protected.POST("/templateparameter/delete", tplParamHandler.Delete)

	// Veristore (TMS) - Terminal
	protected.GET("/veristore/terminal", tmsHandler.Terminal)
	protected.GET("/veristore/add", tmsHandler.Add)
	protected.POST("/veristore/add", tmsHandler.Add)
	protected.GET("/veristore/edit", tmsHandler.Edit)
	protected.POST("/veristore/edit", tmsHandler.Edit)
	protected.GET("/veristore/edit/param", tmsHandler.EditParam)
	protected.GET("/veristore/copy", tmsHandler.Copy)
	protected.POST("/veristore/copy", tmsHandler.Copy)
	protected.POST("/veristore/delete", tmsHandler.Delete)
	protected.POST("/veristore/replacement", tmsHandler.Replacement)
	protected.GET("/veristore/check", tmsHandler.Check)
	protected.POST("/veristore/check", tmsHandler.Check)
	protected.GET("/veristore/check/pdf", tmsHandler.CheckPDF)
	protected.GET("/veristore/report", tmsHandler.Report)
	protected.POST("/veristore/report", tmsHandler.Report)
	protected.GET("/veristore/export", tmsHandler.Export)
	protected.POST("/veristore/export", tmsHandler.Export)
	protected.GET("/veristore/exportresult", tmsHandler.ExportResult)
	protected.GET("/veristore/exportreset", tmsHandler.ExportReset)
	protected.GET("/veristore/import", tmsHandler.Import)
	protected.POST("/veristore/import", tmsHandler.Import)
	protected.GET("/veristore/import-format", tmsHandler.ImportFormat)
	protected.GET("/veristore/import-result", tmsHandler.ImportResult)
	protected.GET("/veristore/import-merchant", tmsHandler.ImportMerchant)
	protected.POST("/veristore/import-merchant", tmsHandler.ImportMerchant)
	protected.POST("/veristore/change-merchant", tmsHandler.ChangeMerchant)

	// Veristore - Merchant
	protected.GET("/veristore/merchant", tmsHandler.Merchant)
	protected.GET("/veristore/merchant/add", tmsHandler.AddMerchant)
	protected.POST("/veristore/merchant/add", tmsHandler.AddMerchant)
	protected.GET("/veristore/merchant/edit", tmsHandler.EditMerchant)
	protected.POST("/veristore/merchant/edit", tmsHandler.EditMerchant)
	protected.POST("/veristore/merchant/delete", tmsHandler.DeleteMerchant)

	// Veristore - Group
	protected.GET("/veristore/group", tmsHandler.Group)
	protected.GET("/veristore/group/add", tmsHandler.AddGroup)
	protected.POST("/veristore/group/add", tmsHandler.AddGroup)
	protected.GET("/veristore/group/edit", tmsHandler.EditGroup)
	protected.POST("/veristore/group/edit", tmsHandler.EditGroup)
	protected.POST("/veristore/group/delete", tmsHandler.DeleteGroup)
	protected.GET("/veristore/group/terminal", tmsHandler.AddGroupTerminal)

	// Veristore - AJAX / dropdowns
	protected.GET("/veristore/operator", tmsHandler.GetOperator)
	protected.GET("/veristore/verify-code", tmsHandler.GetVerifyCode)
	protected.GET("/veristore/model", tmsHandler.GetModel)
	protected.GET("/veristore/state", tmsHandler.GetState)
	protected.GET("/veristore/city", tmsHandler.GetCity)
	protected.GET("/veristore/district", tmsHandler.GetDistrict)

	// Veristore - Login
	protected.GET("/veristore/login", tmsHandler.Login)
	protected.POST("/veristore/login", tmsHandler.Login)

	// Verification
	protected.GET("/verification/index", verifyHandler.Index)
	protected.POST("/verification/index", verifyHandler.Index)
	protected.GET("/verification/gettechnician", verifyHandler.GetTechnician)

	// Verification Report
	protected.GET("/verificationreport/index", reportHandler.Index)
	protected.GET("/verificationreport/view", reportHandler.View)
	protected.GET("/verificationreport/create", reportHandler.Create)
	protected.POST("/verificationreport/create", reportHandler.Create)
	protected.GET("/verificationreport/update", reportHandler.Update)
	protected.POST("/verificationreport/update", reportHandler.Update)
	protected.POST("/verificationreport/delete", reportHandler.Delete)

	// Sync Terminal
	protected.GET("/sync-terminal/index", syncHandler.Index)
	protected.GET("/sync-terminal/view", syncHandler.View)
	protected.POST("/sync-terminal/create", syncHandler.Create)
	protected.POST("/sync-terminal/delete", syncHandler.Delete)
	protected.GET("/sync-terminal/download", syncHandler.Download)
	protected.POST("/sync-terminal/reset", syncHandler.Reset)

	// Activity Log
	protected.GET("/activitylog/index", adminHandler.ActivityLogIndex)
	protected.GET("/activitylog/view", adminHandler.ActivityLogView)

	// Technician
	protected.GET("/technician/index", adminHandler.TechnicianIndex)
	protected.GET("/technician/view", adminHandler.TechnicianView)
	protected.GET("/technician/create", adminHandler.TechnicianCreate)
	protected.POST("/technician/create", adminHandler.TechnicianCreate)
	protected.GET("/technician/update", adminHandler.TechnicianUpdate)
	protected.POST("/technician/update", adminHandler.TechnicianUpdate)
	protected.POST("/technician/delete", adminHandler.TechnicianDelete)

	// FAQ
	protected.GET("/faq/index", adminHandler.FaqIndex)
	protected.GET("/faq/download", adminHandler.FaqDownload)

	// Backup
	protected.GET("/backup/index", adminHandler.BackupIndex)
	protected.GET("/backup/download", adminHandler.BackupDownload)

	// App Activation
	protected.GET("/appactivation/index", actHandler.ActivationIndex)
	protected.GET("/appactivation/view", actHandler.ActivationView)
	protected.GET("/appactivation/create", actHandler.ActivationCreate)
	protected.POST("/appactivation/create", actHandler.ActivationCreate)
	protected.GET("/appactivation/update", actHandler.ActivationUpdate)
	protected.POST("/appactivation/update", actHandler.ActivationUpdate)
	protected.POST("/appactivation/delete", actHandler.ActivationDelete)

	// App Credential
	protected.GET("/appcredential/index", actHandler.CredentialIndex)
	protected.GET("/appcredential/view", actHandler.CredentialView)
	protected.GET("/appcredential/create", actHandler.CredentialCreate)
	protected.POST("/appcredential/create", actHandler.CredentialCreate)
	protected.GET("/appcredential/update", actHandler.CredentialUpdate)
	protected.POST("/appcredential/update", actHandler.CredentialUpdate)
	protected.POST("/appcredential/delete", actHandler.CredentialDelete)

	// TMS Login
	protected.GET("/tmslogin/index", tmsLoginHandler.Index)
	protected.POST("/tmslogin/create", tmsLoginHandler.Create)
	protected.POST("/tmslogin/delete", tmsLoginHandler.Delete)
	protected.GET("/tmslogin/getoperator", tmsLoginHandler.GetOperator)
	protected.GET("/tmslogin/getverifycode", tmsLoginHandler.GetVerifyCode)

	// Scheduler
	protected.GET("/scheduler/index", schedulerHandler.Index)
	protected.POST("/scheduler/update", schedulerHandler.Update)

	// -----------------------------------------------------------------------
	// 15. API routes (basic auth)
	// -----------------------------------------------------------------------
	api := e.Group("/feature/api", mw.BasicAuth(cfg.API.BasicAuthUser, cfg.API.BasicAuthPass))
	api.POST("/activation-code", apiHandler.ActivationCode)

	// -----------------------------------------------------------------------
	// 16. Instantiate queue task handlers
	// -----------------------------------------------------------------------
	importTermHandler := queue.NewImportTerminalHandler(tmsService, tmsClient, adminRepo, db)
	exportTermHandler := queue.NewExportTerminalHandler(tmsService, tmsClient, adminRepo, db, cfg.Export.OutputDir)
	importMerchHandler := queue.NewImportMerchantHandler(tmsService, tmsClient, adminRepo, db)
	syncParamHandler := queue.NewSyncParameterHandler(tmsService, tmsClient, terminalRepo, adminRepo, syncRepo, db, cfg.TMS.SyncBatchSize)
	tmsPingHandler := queue.NewTMSPingHandler(tmsService, tmsRepo)
	schedulerCheckHandler := queue.NewSchedulerCheckHandler(tmsRepo, asynqClient)

	// -----------------------------------------------------------------------
	// 18. Start Asynq worker in a goroutine
	// -----------------------------------------------------------------------
	asynqWorker := queue.NewWorker(redisCfg)
	asynqMux := queue.NewMux(map[string]asynq.Handler{
		queue.TaskImportTerminal: importTermHandler,
		queue.TaskExportTerminal: exportTermHandler,
		queue.TaskImportMerchant: importMerchHandler,
		queue.TaskSyncParameter:  syncParamHandler,
		queue.TaskExportAll:      exportTermHandler, // reuse export handler for export-all
		queue.TaskTMSPing:        tmsPingHandler,
		queue.TaskSchedulerCheck: schedulerCheckHandler,
	})

	go func() {
		log.Info().Str("redis", cfg.Redis.Addr).Msg("starting Asynq worker")
		if err := asynqWorker.Run(asynqMux); err != nil {
			log.Error().Err(err).Msg("asynq worker error")
		}
	}()

	// -----------------------------------------------------------------------
	// 19. Start Asynq scheduler in a goroutine
	// -----------------------------------------------------------------------
	asynqScheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		nil,
	)

	// TMS ping every 15 minutes
	if _, err := asynqScheduler.Register("@every 15m", asynq.NewTask(queue.TaskTMSPing, nil)); err != nil {
		log.Warn().Err(err).Msg("failed to register TMS ping schedule")
	}

	// Scheduler check every 1 minute
	if _, err := asynqScheduler.Register("@every 1m", asynq.NewTask(queue.TaskSchedulerCheck, nil)); err != nil {
		log.Warn().Err(err).Msg("failed to register scheduler check schedule")
	}

	go func() {
		log.Info().Msg("starting Asynq scheduler")
		if err := asynqScheduler.Run(); err != nil {
			log.Error().Err(err).Msg("asynq scheduler error")
		}
	}()

	// -----------------------------------------------------------------------
	// 20. Start Echo web server
	// -----------------------------------------------------------------------
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	go func() {
		log.Info().Str("addr", addr).Msg("starting HTTP server")
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// -----------------------------------------------------------------------
	// 21. Handle graceful shutdown on SIGINT/SIGTERM
	// -----------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop Asynq scheduler and worker.
	asynqScheduler.Shutdown()
	asynqWorker.Shutdown()

	// Shutdown Echo server.
	if err := e.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("echo shutdown error")
	}

	// Close database connection.
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}

	log.Info().Msg("server stopped")
}
