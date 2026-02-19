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
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

// ImportMerchantPayload is the JSON payload for the import:merchant task.
type ImportMerchantPayload struct {
	FilePath  string `json:"file_path"`
	Session   string `json:"session"`
	User      string `json:"user"`
	CountryID int    `json:"country_id"`
}

// ImportMerchantHandler processes Excel files containing merchant data and
// registers each merchant via the TMS API. This is the Go equivalent of
// veristoreTools2's ImportMerchant.php component.
type ImportMerchantHandler struct {
	tmsService *tms.Service
	tmsClient  *tms.Client
	adminRepo  *admin.Repository
	db         *gorm.DB
}

// NewImportMerchantHandler creates a new handler for the import:merchant task.
func NewImportMerchantHandler(tmsService *tms.Service, tmsClient *tms.Client, adminRepo *admin.Repository, db *gorm.DB) *ImportMerchantHandler {
	return &ImportMerchantHandler{
		tmsService: tmsService,
		tmsClient:  tmsClient,
		adminRepo:  adminRepo,
		db:         db,
	}
}

// ProcessTask implements asynq.Handler. It reads the Excel file specified in
// the payload, iterates over rows, and calls the TMS API to add each merchant.
// Progress is logged to the queue_log table.
//
// Excel column mapping (matching v2 ImportMerchant.php):
//   - Column A (0): Row Number / ID
//   - Column B (1): Merchant Name
//   - Column C (2): State ID
//   - Column D (3): City ID
//   - Column E (4): Post Code
//   - Column F (5): Address
//   - Column G (6): Time Zone
//   - Column H (7): Contact
//   - Column I (8): Email
//   - Column J (9): Cell Phone
//   - Column K (10): Telephone
func (h *ImportMerchantHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ImportMerchantPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("import_merchant: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskImportMerchant).Str("file", payload.FilePath).Logger()
	logger.Info().Msg("starting merchant import job")

	// Log job start to queue_log.
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  createTime,
		ExecTime:    createTime,
		ProcessName: "IMCH",
		ServiceName: strPtr("import:merchant"),
	})

	// Open the Excel file.
	f, err := excelize.OpenFile(payload.FilePath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to open Excel file")
		return fmt.Errorf("import_merchant: open file: %w", err)
	}
	defer f.Close()

	// Get the first sheet.
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("import_merchant: no sheets found in file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("import_merchant: read rows: %w", err)
	}

	if len(rows) < 2 {
		logger.Warn().Msg("Excel file has no data rows (only header)")
		return nil
	}

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		return fmt.Errorf("import_merchant: no active TMS session")
	}

	countryID := payload.CountryID
	if countryID == 0 {
		countryID = 1 // Default country ID
	}

	var successCount, failCount int

	// Process data rows (skip header row at index 0).
	for rowIdx, row := range rows[1:] {
		rowNum := rowIdx + 2

		select {
		case <-ctx.Done():
			logger.Warn().Int("row", rowNum).Msg("context cancelled, stopping import")
			return ctx.Err()
		default:
		}

		merchantName := strings.TrimSpace(cellValue(row, 1))
		if merchantName == "" {
			failCount++
			logger.Warn().Int("row", rowNum).Msg("skipping row: merchant name is empty")
			continue
		}

		stateID := cellValue(row, 2)
		cityID := cellValue(row, 3)
		postCode := cellValue(row, 4)
		address := cellValue(row, 5)
		timeZone := cellValue(row, 6)
		contact := cellValue(row, 7)
		email := cellValue(row, 8)
		cellPhone := cellValue(row, 9)
		telePhone := cellValue(row, 10)

		// Default email if empty (matching v2 logic).
		if email == "" {
			email = "dummy@sample.com"
		}

		// Step 1: Look up district from city ID.
		var districtID string
		if cityID != "" {
			cityIDInt, err := strconv.Atoi(cityID)
			if err == nil {
				distResp, err := h.tmsClient.GetDistrictList(session, cityIDInt)
				if err == nil && distResp.ResultCode == 0 && distResp.Data != nil {
					if districts, ok := distResp.Data["districts"].([]interface{}); ok && len(districts) > 0 {
						if first, ok := districts[0].(map[string]interface{}); ok {
							districtID = fmt.Sprintf("%v", first["id"])
						}
					}
				}
			}
		}

		// Step 2: Add merchant via TMS API.
		merchantData := tms.MerchantData{
			MerchantName: strings.ToUpper(merchantName),
			Address:      strings.ToUpper(address),
			PostCode:     postCode,
			TimeZone:     timeZone,
			Contact:      strings.ToUpper(contact),
			Email:        strings.ToUpper(email),
			CellPhone:    cellPhone,
			TelePhone:    telePhone,
			CountryId:    strconv.Itoa(countryID),
			StateId:      stateID,
			CityId:       cityID,
			DistrictId:   districtID,
		}

		addResp, err := h.tmsClient.AddMerchant(session, merchantData)
		if err != nil {
			failCount++
			logger.Error().Err(err).Int("row", rowNum).Msg("add merchant API call failed")
			continue
		}
		if addResp.ResultCode != 0 {
			failCount++
			logger.Warn().Int("row", rowNum).Str("desc", addResp.Desc).Msg("add merchant returned error")
			continue
		}

		successCount++
		logger.Info().Int("row", rowNum).Str("merchant", merchantName).Msg("merchant imported successfully")
	}

	logger.Info().
		Int("success", successCount).
		Int("failed", failCount).
		Msg("merchant import job completed")

	return nil
}
