package middleware

import (
	"time"

	"gorm.io/gorm"
)

// Activity log type constants matching v2 ActivityLogHelper.php
const (
	LogLogin              = "LOGIN"
	LogLogout             = "LOGOUT"
	LogUserCreate         = "USER_CREATE"
	LogUserUpdate         = "USER_UPDATE"
	LogUserDelete         = "USER_DELETE"
	LogUserActivate       = "USER_ACTIVATE"
	LogUserDeactivate     = "USER_DEACTIVATE"
	LogUserChangePassword = "USER_CHANGE_PASSWORD"
	LogTerminalAdd        = "TERMINAL_ADD"
	LogTerminalEdit       = "TERMINAL_EDIT"
	LogTerminalCopy       = "TERMINAL_COPY"
	LogTerminalDelete     = "TERMINAL_DELETE"
	LogTerminalReplace    = "TERMINAL_REPLACE"
	LogTerminalImport     = "TERMINAL_IMPORT"
	LogTerminalExport     = "TERMINAL_EXPORT"
	LogMerchantAdd        = "MERCHANT_ADD"
	LogMerchantEdit       = "MERCHANT_EDIT"
	LogMerchantDelete     = "MERCHANT_DELETE"
	LogMerchantImport     = "MERCHANT_IMPORT"
	LogGroupAdd           = "GROUP_ADD"
	LogGroupEdit          = "GROUP_EDIT"
	LogGroupDelete        = "GROUP_DELETE"
	LogSyncStart          = "SYNC_START"
	LogSyncComplete       = "SYNC_COMPLETE"
	LogVerification       = "VERIFICATION"
	LogTechnicianCreate   = "TECHNICIAN_CREATE"
	LogTechnicianUpdate   = "TECHNICIAN_UPDATE"
	LogTechnicianDelete   = "TECHNICIAN_DELETE"
)

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

// LogActivity records an activity log entry into the database.
// This is a helper function, not Echo middleware. Handlers call it at
// appropriate points to record user actions.
func LogActivity(db *gorm.DB, logType, logDesc, logUser, logIP string) {
	entry := activityLogEntry{
		ActLogAction: logType,
		ActLogDetail: &logDesc,
		CreatedBy:    logUser,
		CreatedDt:    time.Now(),
	}
	db.Create(&entry)
}
