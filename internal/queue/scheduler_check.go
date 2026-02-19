package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/verifone/veristoretools3/internal/tms"
)

// ScheduledSync represents a single scheduled sync entry from the
// tms_login.tms_login_scheduled JSON field.
type ScheduledSync struct {
	Hour   int    `json:"hour"`
	Minute int    `json:"minute"`
	Day    string `json:"day,omitempty"`
	Active bool   `json:"active"`
}

// SchedulerCheckHandler checks the tms_login.tms_login_scheduled JSON for
// any scheduled syncs. If the current time matches a schedule, it enqueues
// a sync:parameter task. This job runs every 1 minute.
type SchedulerCheckHandler struct {
	tmsRepo     *tms.Repository
	queueClient *asynq.Client
}

// NewSchedulerCheckHandler creates a new handler for the tms:scheduler_check
// task.
func NewSchedulerCheckHandler(tmsRepo *tms.Repository, queueClient *asynq.Client) *SchedulerCheckHandler {
	return &SchedulerCheckHandler{
		tmsRepo:     tmsRepo,
		queueClient: queueClient,
	}
}

// ProcessTask implements asynq.Handler. It reads the active TMS login record,
// parses the scheduled JSON field, and enqueues a sync:parameter task if the
// current time matches any active schedule.
func (h *SchedulerCheckHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	logger := log.With().Str("task", TaskSchedulerCheck).Logger()

	// Get the active TMS login record.
	login, err := h.tmsRepo.GetActiveLogin()
	if err != nil {
		logger.Debug().Err(err).Msg("scheduler check: no active TMS login")
		return nil // Not a retryable error.
	}

	if login.TmsLoginScheduled == nil || *login.TmsLoginScheduled == "" {
		logger.Debug().Msg("scheduler check: no scheduled syncs configured")
		return nil
	}

	// Parse the scheduled JSON.
	var schedules []ScheduledSync
	if err := json.Unmarshal([]byte(*login.TmsLoginScheduled), &schedules); err != nil {
		logger.Warn().Err(err).Msg("scheduler check: failed to parse scheduled JSON")
		return nil // Don't retry on parse errors.
	}

	now := time.Now()
	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentDay := now.Weekday().String()

	for _, sched := range schedules {
		if !sched.Active {
			continue
		}

		// Check if the current time matches the schedule.
		if sched.Hour == currentHour && sched.Minute == currentMinute {
			// If a day is specified, check if it matches.
			if sched.Day != "" && sched.Day != currentDay {
				continue
			}

			logger.Info().
				Int("hour", sched.Hour).
				Int("minute", sched.Minute).
				Msg("scheduler check: time matches, enqueuing sync task")

			// Build the sync payload.
			session := ""
			if login.TmsLoginSession != nil {
				session = *login.TmsLoginSession
			}
			user := ""
			if login.TmsLoginUser != nil {
				user = *login.TmsLoginUser
			}

			syncPayload := SyncParameterPayload{
				SyncID:  0, // Will be set by the sync handler or caller.
				Session: session,
				User:    user,
			}

			payloadBytes, err := json.Marshal(syncPayload)
			if err != nil {
				logger.Error().Err(err).Msg("scheduler check: failed to marshal sync payload")
				continue
			}

			syncTask := asynq.NewTask(
				TaskSyncParameter,
				payloadBytes,
				asynq.MaxRetry(3),
				asynq.Timeout(1*time.Hour),
				asynq.Queue("default"),
				asynq.TaskID(fmt.Sprintf("scheduled-sync-%s", strconv.FormatInt(now.UnixMilli(), 10))),
			)

			info, err := h.queueClient.Enqueue(syncTask)
			if err != nil {
				logger.Error().Err(err).Msg("scheduler check: failed to enqueue sync task")
				continue
			}

			logger.Info().
				Str("task_id", info.ID).
				Str("queue", info.Queue).
				Msg("scheduler check: sync task enqueued successfully")
		}
	}

	return nil
}
