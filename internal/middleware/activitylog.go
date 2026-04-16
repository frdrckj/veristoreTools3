package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Activity log type constants matching v2 ActivityLogHelper.php naming.
const (
	LogLogin              = "LOGIN"
	LogLogout             = "LOGOUT"
	LogCreateUser         = "CREATE USER"
	LogUpdateUser         = "UPDATE USER"
	LogCreateEngineer     = "CREATE ENGINEER"
	LogUpdateEngineer     = "UPDATE ENGINEER"
	LogSyncData           = "SINKRONISASI DATA"
	LogVerifyTerminal     = "VERIFIKASI CSI"
	LogTmsLogin           = "TMS LOGIN"
	LogSchedulerSync      = "PENJADWALAN SINKRONISASI DATA"
	LogSchedulerSyncEdit  = "PERUBAHAN PENJADWALAN SINKRONISASI"
	LogVeristoreLogin     = "VERISTORE LOGIN"
	LogVeristoreAddCSI    = "VERISTORE ADD CSI"
	LogVeristoreCopyCSI   = "VERISTORE DUPLICATE CSI"
	LogVeristoreDeleteCSI = "VERISTORE DELETE CSI"
	LogVeristoreEditParam = "VERISTORE EDIT PARAMETER"
	LogVeristoreReport    = "VERISTORE CREATE REPORT"
	LogVeristoreImportCSI = "VERISTORE IMPORT CSI"
	LogVeristoreAddMerch  = "VERISTORE ADD MERCHANT"
	LogVeristoreEditMerch = "VERISTORE EDIT MERCHANT"
	LogVeristoreDelMerch  = "VERISTORE DELETE MERCHANT"
	LogVeristoreAddGroup  = "VERISTORE ADD GROUP"
	LogVeristoreEditGroup = "VERISTORE EDIT GROUP"
	LogVeristoreDelGroup  = "VERISTORE DELETE GROUP"
	LogVeristoreExport    = "VERISTORE EXPORT TERMINAL"
	LogVeristoreReplace   = "VERISTORE REPLACEMENT CSI"
	LogVeristoreImportMerch     = "VERISTORE IMPORT MERCHANT"
	LogVeristoreEditMerchTerm   = "VERISTORE EDIT MERCHANT TERMINAL"
	LogVeristoreAddTech         = "VERISTORE ADD TECHNICIAN"
	LogVeristoreEditTech        = "VERISTORE EDIT TECHNICIAN"
	LogVeristoreDelTech         = "VERISTORE DELETE TECHNICIAN"
	LogVeristoreRequestCSI      = "VERISTORE REQUEST CSI"
	LogVeristoreApproveCSI      = "VERISTORE APPROVE CSI"
	LogVeristoreRejectCSI       = "VERISTORE REJECT CSI"
	LogVerificationCreate       = "LAPORAN VERIFIKASI CREATE"
	LogVerificationEdit         = "LAPORAN VERIFIKASI EDIT"
	LogVerificationDelete       = "LAPORAN VERIFIKASI DELETE"
	LogVerificationExport       = "LAPORAN VERIFIKASI EXPORT"
)

// contextKeyDB is the key used to store *gorm.DB in the echo context.
const contextKeyDB = "actlog_db"

// activityLogEntry mirrors admin.ActivityLog to avoid an import cycle between
// middleware and admin packages.
type activityLogEntry struct {
	ActLogID     int       `gorm:"column:act_log_id;primaryKey;autoIncrement"`
	ActLogAction string    `gorm:"column:act_log_action;type:varchar(100);not null"`
	ActLogDetail *string   `gorm:"column:act_log_detail;type:text"`
	CreatedBy    string    `gorm:"column:created_by;type:varchar(100);not null"`
	CreatedDt    time.Time `gorm:"column:created_dt;not null"`
}

func (activityLogEntry) TableName() string {
	return "activity_log"
}

// ActivityLogDBMiddleware stores *gorm.DB in the echo context so handlers
// can call LogActivityFromContext without needing db injected into their struct.
func ActivityLogDBMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(contextKeyDB, db)
			return next(c)
		}
	}
}

// LogActivity records an activity log entry into the database.
func LogActivity(db *gorm.DB, logType, logDesc, logUser string) {
	entry := activityLogEntry{
		ActLogAction: logType,
		ActLogDetail: &logDesc,
		CreatedBy:    logUser,
		CreatedDt:    time.Now(),
	}
	db.Create(&entry)
}

// LogActivityFromContext is a convenience helper for handlers. It extracts
// db from the echo context (set by ActivityLogDBMiddleware) and the current
// user from the session, then writes the log entry.
func LogActivityFromContext(c echo.Context, logType, detail string) {
	db, ok := c.Get(contextKeyDB).(*gorm.DB)
	if !ok || db == nil {
		return
	}
	user := GetCurrentUserFullname(c)
	if user == "" {
		user = GetCurrentUserName(c)
	}
	if user == "" {
		user = "Unknown"
	}
	LogActivity(db, logType, detail, user)
}
