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

// TMSPingPayload is the JSON payload for the tms:ping task.
type TMSPingPayload struct {
	Session string `json:"session,omitempty"`
}

// TMSPingHandler checks TMS token validity by calling CheckToken().
// This job runs periodically (every 15 minutes) to verify the TMS session
// is still active.
type TMSPingHandler struct {
	tmsService *tms.Service
	tmsRepo    *tms.Repository
}

// NewTMSPingHandler creates a new handler for the tms:ping task.
func NewTMSPingHandler(tmsService *tms.Service, tmsRepo *tms.Repository) *TMSPingHandler {
	return &TMSPingHandler{
		tmsService: tmsService,
		tmsRepo:    tmsRepo,
	}
}

// ProcessTask implements asynq.Handler. It checks the current TMS session
// token by calling CheckToken(). If the token is invalid, it logs a warning
// so that the system can take corrective action.
func (h *TMSPingHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	logger := log.With().Str("task", TaskTMSPing).Logger()

	var payload TMSPingPayload
	if len(task.Payload()) > 0 {
		_ = json.Unmarshal(task.Payload(), &payload)
	}

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		logger.Warn().Msg("TMS ping: no active session found")
		return nil // Not a retryable error; no session means nothing to check.
	}

	startTime := time.Now()
	resp, err := h.tmsService.CheckToken()
	elapsed := time.Since(startTime)

	if err != nil {
		logger.Warn().
			Err(err).
			Dur("elapsed", elapsed).
			Msg("TMS ping: check token failed")
		return fmt.Errorf("tms_ping: check token: %w", err)
	}

	if resp.ResultCode != 0 {
		logger.Warn().
			Int("code", resp.ResultCode).
			Str("desc", resp.Desc).
			Dur("elapsed", elapsed).
			Msg("TMS ping: token invalid or expired")
		// Update the tms_login record to reflect the invalid state.
		return nil
	}

	logger.Info().
		Dur("elapsed", elapsed).
		Str("ping_time", strconv.FormatInt(time.Now().UnixMilli(), 10)).
		Msg("TMS ping: token is valid")

	return nil
}
