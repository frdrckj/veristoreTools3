package queue

// Task type constants for the Asynq queue system.
// Each constant corresponds to a specific background job that can be enqueued.
const (
	TaskImportTerminal = "import:terminal"
	TaskExportTerminal = "export:terminal"
	TaskImportMerchant = "import:merchant"
	TaskSyncParameter  = "sync:parameter"
	TaskExportAll      = "export:all_terminals"
	TaskTMSPing        = "tms:ping"
	TaskSchedulerCheck = "tms:scheduler_check"
	TaskReportTerminal = "report:terminal"
)
