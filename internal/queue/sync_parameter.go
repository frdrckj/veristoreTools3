package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/sync"
	"github.com/verifone/veristoretools3/internal/terminal"
	"github.com/verifone/veristoretools3/internal/tms"
)

// SyncParameterPayload is the JSON payload for the sync:parameter task.
type SyncParameterPayload struct {
	SyncID  int    `json:"sync_id"`
	Session string `json:"session"`
	User    string `json:"user"`
}

// SyncParameterHandler fetches terminal parameters from the TMS API and
// updates the local terminal and terminal_parameter tables. This is the Go
// equivalent of veristoreTools2's SyncTerminalParameter.php component.
type SyncParameterHandler struct {
	tmsService *tms.Service
	tmsClient  *tms.Client
	termRepo   *terminal.Repository
	adminRepo  *admin.Repository
	syncRepo   *sync.Repository
	db         *gorm.DB
	batchSize  int
}

// NewSyncParameterHandler creates a new handler for the sync:parameter task.
func NewSyncParameterHandler(
	tmsService *tms.Service,
	tmsClient *tms.Client,
	termRepo *terminal.Repository,
	adminRepo *admin.Repository,
	syncRepo *sync.Repository,
	db *gorm.DB,
	batchSize int,
) *SyncParameterHandler {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &SyncParameterHandler{
		tmsService: tmsService,
		tmsClient:  tmsClient,
		termRepo:   termRepo,
		adminRepo:  adminRepo,
		syncRepo:   syncRepo,
		db:         db,
		batchSize:  batchSize,
	}
}

// ProcessTask implements asynq.Handler. It:
//  1. Gets an active TMS session.
//  2. Fetches the terminal list from TMS API (paginated).
//  3. For each terminal, fetches parameters from TMS.
//  4. Updates the local terminal and terminal_parameter tables.
//  5. Logs progress to queue_log and updates the sync_terminal record.
func (h *SyncParameterHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload SyncParameterPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("sync_parameter: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskSyncParameter).Int("sync_id", payload.SyncID).Logger()
	logger.Info().Msg("starting parameter sync job")

	// Log job start to queue_log.
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  createTime,
		ExecTime:    createTime,
		ProcessName: "SYNC",
		ServiceName: strPtr("sync:parameter"),
	})

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		return fmt.Errorf("sync_parameter: no active TMS session")
	}

	// Update sync record status to "in progress" (status = '2').
	if payload.SyncID > 0 {
		_ = h.syncRepo.UpdateStatus(payload.SyncID, "2")
	}

	var totalProcessed int
	page := 1

	for {
		select {
		case <-ctx.Done():
			logger.Warn().Int("processed", totalProcessed).Msg("context cancelled, stopping sync")
			return ctx.Err()
		default:
		}

		// Fetch a page of terminals from TMS.
		termListResp, err := h.tmsClient.GetTerminalList(session, page)
		if err != nil {
			logger.Error().Err(err).Int("page", page).Msg("failed to get terminal list")
			break
		}
		if termListResp.ResultCode != 0 {
			logger.Warn().Str("desc", termListResp.Desc).Int("page", page).Msg("terminal list returned error")
			break
		}
		if termListResp.Data == nil {
			break
		}

		terminalList, ok := termListResp.Data["terminalList"].([]interface{})
		if !ok || len(terminalList) == 0 {
			break
		}

		// Process each terminal on this page.
		for _, rawTerm := range terminalList {
			termData, ok := rawTerm.(map[string]interface{})
			if !ok {
				continue
			}

			serialNum := tms.ToString(termData["sn"])
			deviceID := tms.ToString(termData["deviceId"])
			model := tms.ToString(termData["model"])

			if serialNum == "" {
				continue
			}

			// Get terminal detail for app list.
			detailResp, err := h.tmsClient.GetTerminalDetail(session, serialNum)
			if err != nil || detailResp.ResultCode != 0 {
				logger.Warn().Str("serial", serialNum).Msg("failed to get terminal detail during sync")
				continue
			}

			// Find the first app with parameters.
			appID, appName, appVersion := h.findFirstApp(detailResp)
			if appID == "" {
				logger.Debug().Str("serial", serialNum).Msg("no apps found on terminal, skipping parameters")
			}

			// Get terminal parameter data if we have an app (batch multi-tab).
			var paramResp *tms.TMSResponse
			if appID != "" {
				tabNames := tms.GetAllTabNames(h.db)
				resp, err := h.tmsClient.GetTerminalParameterMultiTab(session, serialNum, appID, tabNames)
				if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
					if pl, ok := resp.Data["paraList"].([]interface{}); ok && len(pl) > 0 {
						paramResp = resp
					}
				}
				if paramResp == nil {
					logger.Warn().Str("serial", serialNum).Msg("failed to get terminal parameters during sync")
				}
			}

			// Update local database in a transaction.
			if err := h.updateLocalTerminal(ctx, serialNum, deviceID, model, appName, appVersion, payload.User, paramResp); err != nil {
				logger.Error().Err(err).Str("serial", serialNum).Msg("failed to update local terminal")
				continue
			}

			totalProcessed++
		}

		// Check if we've reached the last page.
		totalPages := 1
		if tp := termListResp.Data["totalPage"]; tp != nil {
			if tpFloat, ok := tp.(float64); ok {
				totalPages = int(tpFloat)
			}
		}

		if page >= totalPages {
			break
		}
		page++
	}

	// Update sync record status to "completed" (status = '3').
	if payload.SyncID > 0 {
		processStr := strconv.Itoa(totalProcessed)
		syncRecord, err := h.syncRepo.FindByID(payload.SyncID)
		if err == nil && syncRecord != nil {
			syncRecord.SyncTermProcess = &processStr
			syncRecord.SyncTermStatus = "3"
			_ = h.syncRepo.Update(syncRecord)
		}
	}

	logger.Info().
		Int("processed", totalProcessed).
		Msg("parameter sync job completed")

	return nil
}

// findFirstApp looks for the first app in a terminal detail response.
func (h *SyncParameterHandler) findFirstApp(resp *tms.TMSResponse) (appID, appName, appVersion string) {
	if resp.Data == nil {
		return "", "", ""
	}
	apps, ok := resp.Data["terminalShowApps"].([]interface{})
	if !ok || len(apps) == 0 {
		return "", "", ""
	}
	first, ok := apps[0].(map[string]interface{})
	if !ok {
		return "", "", ""
	}
	return fmt.Sprintf("%v", first["id"]),
		tms.ToString(first["name"]),
		tms.ToString(first["version"])
}

// nonPrintableRegexp matches non-printable characters (outside ASCII 0x20-0x7F).
var nonPrintableRegexp = regexp.MustCompile(`[^\x20-\x7F]`)

// updateLocalTerminal creates or updates the local terminal and its parameters.
// This closely follows the v2 SyncTerminalParameter.php process == 2 logic.
func (h *SyncParameterHandler) updateLocalTerminal(
	ctx context.Context,
	serialNum, deviceID, model, appName, appVersion, user string,
	paramResp *tms.TMSResponse,
) error {
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		termRepo := terminal.NewRepository(tx)

		// Find existing terminal or create a new one.
		existing, _ := termRepo.FindByCSI(serialNum)

		now := time.Now()
		var term *terminal.Terminal

		if len(existing) > 0 {
			term = &existing[0]
			term.TermDeviceID = deviceID
			term.TermProductNum = ""
			term.TermModel = model
			term.TermAppName = appName
			term.TermAppVersion = appVersion
			term.TermTmsUpdateOperator = &user
			term.TermTmsUpdateDtOperator = &now
			term.UpdatedBy = &user
			term.UpdatedDt = &now
			if err := termRepo.Update(term); err != nil {
				return fmt.Errorf("update terminal: %w", err)
			}
		} else {
			term = &terminal.Terminal{
				TermDeviceID:            deviceID,
				TermSerialNum:           serialNum,
				TermProductNum:          "",
				TermModel:               model,
				TermAppName:             appName,
				TermAppVersion:          appVersion,
				TermTmsCreateOperator:   user,
				TermTmsCreateDtOperator: now,
				CreatedBy:               user,
				CreatedDt:               now,
			}
			if err := termRepo.Create(term); err != nil {
				return fmt.Errorf("create terminal: %w", err)
			}
		}

		// Delete existing parameters for this terminal.
		tx.Where("param_term_id = ?", term.TermID).Delete(&terminal.TerminalParameter{})

		// Parse and insert new parameters if we have param data.
		if paramResp == nil || paramResp.Data == nil {
			return nil
		}

		paraList, ok := paramResp.Data["paraList"].([]interface{})
		if !ok || len(paraList) == 0 {
			return nil
		}

		// Build parameter maps matching v2 logic.
		hostIdx := map[int]int{}     // merchant index -> host index
		hostName := map[int]string{} // host index -> name
		merchantEnable := map[int]string{}
		merchantName := map[int]string{}
		merchantID := map[int]string{}
		terminalID := map[int]string{}
		addressIdx := map[int]int{}
		address := map[int]map[int]string{}

		for _, raw := range paraList {
			p, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			dataName := tms.ToString(p["dataName"])
			value := tms.ToString(p["value"])

			if strings.Contains(dataName, "TP-HOST-MERCHANT_INDEX-") {
				idx := extractLastIndex(dataName)
				for _, hidx := range strings.Split(value, ",") {
					if h, err := strconv.Atoi(strings.TrimSpace(hidx)); err == nil {
						hostIdx[h] = idx
					}
				}
			}
			if strings.Contains(dataName, "TP-HOST-HOST_NAME-") {
				idx := extractLastIndex(dataName)
				hostName[idx] = value
			}
			if strings.Contains(dataName, "TP-MERCHANT-ENABLE-") {
				idx := extractLastIndex(dataName)
				merchantEnable[idx] = value
			}
			if strings.Contains(dataName, "TP-MERCHANT-MERCHANT_NAME-") {
				idx := extractLastIndex(dataName)
				merchantName[idx] = value
			}
			if strings.Contains(dataName, "TP-MERCHANT-MERCHANT_ID-") {
				idx := extractLastIndex(dataName)
				merchantID[idx] = value
			}
			if strings.Contains(dataName, "TP-MERCHANT-TERMINAL_ID-") {
				idx := extractLastIndex(dataName)
				terminalID[idx] = value
			}
			if strings.Contains(dataName, "TP-MERCHANT-PRINT_PARAM_INDEX-") {
				idx := extractLastIndex(dataName)
				if v, err := strconv.Atoi(value); err == nil {
					addressIdx[idx] = v
				}
			}
			if strings.Contains(dataName, "TP-PRINT_CONFIG-HEADER") {
				parts := strings.Split(dataName, "-")
				if len(parts) >= 2 {
					idx := extractLastIndex(dataName)
					headerField := parts[len(parts)-2]
					if len(headerField) > 0 {
						headerNum, _ := strconv.Atoi(string(headerField[len(headerField)-1]))
						if address[idx] == nil {
							address[idx] = map[int]string{}
						}
						address[idx][headerNum] = nonPrintableRegexp.ReplaceAllString(value, "")
					}
				}
			}
		}

		// Create terminal parameter records for enabled merchants.
		for key, enable := range merchantEnable {
			if enable != "1" {
				continue
			}
			mid, hasMID := merchantID[key]
			tid, hasTID := terminalID[key]
			if !hasMID || !hasTID {
				continue
			}

			hostValue := hostIdx[key]
			hn := hostName[hostValue]
			mn := merchantName[key]

			param := &terminal.TerminalParameter{
				ParamTermID:      term.TermID,
				ParamHostName:    hn,
				ParamMerchantName: mn,
				ParamTID:         tid,
				ParamMID:         mid,
			}

			// Set address fields.
			if addrIdx, ok := addressIdx[key]; ok {
				if addrs, ok := address[addrIdx]; ok {
					param.ParamAddress1 = safeAddrPtr(addrs[1])
					param.ParamAddress2 = safeAddrPtr(addrs[2])
					param.ParamAddress3 = safeAddrPtr(addrs[3])
					param.ParamAddress4 = safeAddrPtr(addrs[4])
					param.ParamAddress5 = safeAddrPtr(addrs[5])
					param.ParamAddress6 = safeAddrPtr(addrs[6])
				}
			}

			if err := termRepo.CreateParameter(param); err != nil {
				log.Warn().Err(err).Str("serial", serialNum).Msg("failed to create terminal parameter")
			}
		}

		return nil
	})
}

// extractLastIndex extracts the last numeric index from a hyphen-separated
// parameter data name (e.g., "TP-HOST-HOST_NAME-3" -> 3).
func extractLastIndex(dataName string) int {
	parts := strings.Split(dataName, "-")
	if len(parts) == 0 {
		return 0
	}
	idx, _ := strconv.Atoi(parts[len(parts)-1])
	return idx
}

// safeAddrPtr returns a pointer to s if it is non-empty and not "null",
// otherwise nil.
func safeAddrPtr(s string) *string {
	if s == "" || s == "null" {
		return nil
	}
	return &s
}
