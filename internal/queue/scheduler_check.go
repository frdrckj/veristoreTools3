package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/verifone/veristoretools3/internal/tms"
)

// SchedulerCheckHandler checks the tms_login.tms_login_scheduled field for
// a configured schedule. The field stores a pipe-delimited string:
// SETTING|DATE_FROM|DATE_TO|TIME_FROM|TIME_TO
// where SETTING is HOURLY, DAILY, or WEEKLY.
//
// If the current time falls within the scheduled window, it enqueues a
// report:terminal task with TriggerSync=true to run the full two-phase
// flow (report → sync). This job runs every 1 minute.
type SchedulerCheckHandler struct {
	tmsRepo     *tms.Repository
	queueClient *asynq.Client
	packageName string
}

// NewSchedulerCheckHandler creates a new handler for the tms:scheduler_check task.
func NewSchedulerCheckHandler(tmsRepo *tms.Repository, queueClient *asynq.Client, packageName string) *SchedulerCheckHandler {
	return &SchedulerCheckHandler{
		tmsRepo:     tmsRepo,
		queueClient: queueClient,
		packageName: packageName,
	}
}

// ProcessTask implements asynq.Handler. It reads the active TMS login record,
// parses the pipe-delimited schedule, and enqueues a report:terminal task with
// TriggerSync=true if the current time matches the schedule window.
func (h *SchedulerCheckHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	logger := log.With().Str("task", TaskSchedulerCheck).Logger()

	// Get the active TMS login record.
	login, err := h.tmsRepo.GetActiveLogin()
	if err != nil {
		logger.Debug().Err(err).Msg("scheduler check: no active TMS login")
		return nil
	}

	if login.TmsLoginScheduled == nil || *login.TmsLoginScheduled == "" {
		return nil
	}

	// Parse the pipe-delimited schedule: SETTING|DATE_FROM|DATE_TO|TIME_FROM|TIME_TO
	parts := strings.Split(*login.TmsLoginScheduled, "|")
	if len(parts) < 3 {
		logger.Warn().Str("raw", *login.TmsLoginScheduled).Msg("scheduler check: invalid schedule format")
		return nil
	}

	setting := parts[0]  // HOURLY, DAILY, WEEKLY
	dateFrom := parts[1] // 2006-01-02
	dateTo := parts[2]   // 2006-01-02
	timeFrom := ""
	timeTo := ""
	if len(parts) > 3 {
		timeFrom = parts[3] // "00"-"23"
	}
	if len(parts) > 4 {
		timeTo = parts[4] // "00"-"23"
	}

	now := time.Now()

	// Check if current date is within the scheduled date range.
	dfParsed, _ := time.Parse("2006-01-02", dateFrom)
	dtParsed, _ := time.Parse("2006-01-02", dateTo)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if dfParsed.IsZero() || dtParsed.IsZero() {
		return nil
	}
	if today.Before(dfParsed) || today.After(dtParsed) {
		return nil // Current date outside scheduled range.
	}

	// Check if this is the right time to trigger based on setting.
	shouldTrigger := false
	currentHour := now.Hour()
	currentMinute := now.Minute()

	switch setting {
	case "HOURLY":
		// Run every hour at minute 0, within the time window.
		hourFrom, _ := strconv.Atoi(timeFrom)
		hourTo, _ := strconv.Atoi(timeTo)
		if currentMinute == 0 && currentHour >= hourFrom && currentHour <= hourTo {
			shouldTrigger = true
		}
	case "DAILY":
		// Run once per day at 00:00.
		if currentHour == 0 && currentMinute == 0 {
			shouldTrigger = true
		}
	case "WEEKLY":
		// Run once per week on Sunday at 00:00.
		if now.Weekday() == time.Sunday && currentHour == 0 && currentMinute == 0 {
			shouldTrigger = true
		}
	default:
		logger.Warn().Str("setting", setting).Msg("scheduler check: unknown schedule setting")
		return nil
	}

	if !shouldTrigger {
		return nil
	}

	logger.Info().
		Str("setting", setting).
		Str("date_range", dateFrom+" - "+dateTo).
		Msg("scheduler check: schedule matched, enqueuing report+sync task")

	// Build the report:terminal payload with TriggerSync=true.
	session := ""
	if login.TmsLoginSession != nil {
		session = *login.TmsLoginSession
	}
	userName := ""
	if login.TmsLoginUser != nil {
		userName = *login.TmsLoginUser
	}

	reportPayload := ReportTerminalPayload{
		UserID:      0,
		UserName:    userName,
		AppVersion:  "", // Auto-detect latest version
		Session:     session,
		DateTime:    now.Format("2006-01-02 15:04:05"),
		PackageName: h.packageName,
		TriggerSync: true,
	}

	payloadBytes, err := json.Marshal(reportPayload)
	if err != nil {
		logger.Error().Err(err).Msg("scheduler check: failed to marshal report payload")
		return nil
	}

	reportTask := asynq.NewTask(
		TaskReportTerminal,
		payloadBytes,
		asynq.MaxRetry(3),
		asynq.Timeout(1*time.Hour),
		asynq.Queue("default"),
		asynq.TaskID(fmt.Sprintf("scheduled-sync-%s", strconv.FormatInt(now.UnixMilli(), 10))),
	)

	info, err := h.queueClient.Enqueue(reportTask)
	if err != nil {
		logger.Error().Err(err).Msg("scheduler check: failed to enqueue report task")
		return nil
	}

	logger.Info().
		Str("task_id", info.ID).
		Str("queue", info.Queue).
		Msg("scheduler check: report+sync task enqueued successfully")

	return nil
}
