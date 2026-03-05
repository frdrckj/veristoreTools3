package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/tms"
)

// importMerchantWorkerCount controls the number of concurrent API callers for
// merchant import. Each worker makes ~2 API calls (district lookup + add),
// so 20 workers ≈ 40 concurrent connections at peak.
const importMerchantWorkerCount = 20

// ImportMerchantPayload is the JSON payload for the import:merchant task.
type ImportMerchantPayload struct {
	FilePath  string `json:"file_path"`
	Session   string `json:"session"`
	User      string `json:"user"`
	CountryID int    `json:"country_id"`
	ImportID  int    `json:"import_id"`
}

// importMerchantJob represents a single merchant row to be processed by a worker.
type importMerchantJob struct {
	RowNum       int
	MerchantName string
	StateID      string
	CityID       string
	PostCode     string
	Address      string
	TimeZone     string
	Contact      string
	Email        string
	CellPhone    string
	TelePhone    string
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
// the payload, distributes rows to concurrent workers, and calls the TMS API
// to add each merchant.
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

	// Parse all data rows into jobs, skip invalid ones.
	var jobs []importMerchantJob
	totalRows := len(rows) - 1
	var skipCount int

	for rowIdx, row := range rows[1:] {
		rowNum := rowIdx + 2
		merchantName := strings.TrimSpace(cellValue(row, 1))
		if merchantName == "" {
			skipCount++
			logger.Warn().Int("row", rowNum).Msg("skipping row: merchant name is empty")
			continue
		}

		email := cellValue(row, 8)
		if email == "" {
			email = "dummy@sample.com"
		}

		jobs = append(jobs, importMerchantJob{
			RowNum:       rowNum,
			MerchantName: merchantName,
			StateID:      cellValue(row, 2),
			CityID:       cellValue(row, 3),
			PostCode:     cellValue(row, 4),
			Address:      cellValue(row, 5),
			TimeZone:     cellValue(row, 6),
			Contact:      cellValue(row, 7),
			Email:        email,
			CellPhone:    cellValue(row, 9),
			TelePhone:    cellValue(row, 10),
		})
	}

	totalJobs := len(jobs)
	logger.Info().
		Int("total_rows", totalRows).
		Int("valid_jobs", totalJobs).
		Int("skipped", skipCount).
		Int("workers", importMerchantWorkerCount).
		Msg("parsed Excel, starting concurrent merchant import")

	// Start worker pool.
	jobCh := make(chan importMerchantJob, totalJobs)
	var processedCount int64
	var successCount int64
	var failCount int64
	var wg sync.WaitGroup

	for w := 0; w < importMerchantWorkerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				errMsg := h.importSingleMerchant(session, j, countryID)
				if errMsg == "" {
					atomic.AddInt64(&successCount, 1)
					logger.Info().Int("row", j.RowNum).Str("merchant", j.MerchantName).Msg("merchant imported successfully")
				} else {
					atomic.AddInt64(&failCount, 1)
					logger.Warn().Int("row", j.RowNum).Str("error", errMsg).Msg("merchant import failed")
				}

				count := atomic.AddInt64(&processedCount, 1)
				// Update progress every 5 rows to reduce DB writes.
				if payload.ImportID > 0 && (count%5 == 0 || int(count) == totalJobs) {
					processed := skipCount + int(count)
					h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(processed), strconv.Itoa(totalRows))
				}
			}
		}()
	}

	// Send all jobs to the channel. Stop early if cancelled.
sendLoop:
	for _, j := range jobs {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobCh <- j:
		}
	}
	close(jobCh)

	// Wait for all workers to finish.
	wg.Wait()

	// Mark as complete.
	if payload.ImportID > 0 {
		h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(totalRows), strconv.Itoa(totalRows))
	}

	sc := atomic.LoadInt64(&successCount)
	fc := atomic.LoadInt64(&failCount)

	logger.Info().
		Int64("success", sc).
		Int64("failed", fc).
		Int("skipped", skipCount).
		Msg("merchant import job completed")

	mw.LogActivity(h.db, mw.LogVeristoreImportMerch, fmt.Sprintf("Import data merchant: %d success, %d failed", sc, fc), payload.User)

	// Store import result as queue_log for the import merchant page to display.
	// Format: prefix|success_count|fail_count|suffix (same as CSI import).
	var prefix, successPart, failPart string
	prefix = "Import merchant selesai."
	if sc > 0 {
		successPart = fmt.Sprintf("%d merchant berhasil", sc)
	}
	if fc > 0 {
		failPart = fmt.Sprintf("%d merchant gagal", fc)
	}
	resultMsg := fmt.Sprintf("%s|%s|%s|", prefix, successPart, failPart)
	resultMsgPtr := &resultMsg
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  strconv.FormatInt(time.Now().UnixMilli(), 10),
		ExecTime:    strconv.FormatInt(time.Now().UnixMilli(), 10),
		ProcessName: "IMCHRS",
		ServiceName: resultMsgPtr,
	})

	return nil
}

// importSingleMerchant processes a single merchant row: resolves district and
// calls the AddMerchant API. Returns empty string on success, error message on failure.
func (h *ImportMerchantHandler) importSingleMerchant(session string, j importMerchantJob, countryID int) string {
	// Step 1: Look up district from city ID.
	var districtID string
	if j.CityID != "" {
		cityIDInt, err := strconv.Atoi(j.CityID)
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
		MerchantName: strings.ToUpper(j.MerchantName),
		Address:      strings.ToUpper(j.Address),
		PostCode:     j.PostCode,
		TimeZone:     j.TimeZone,
		Contact:      strings.ToUpper(j.Contact),
		Email:        strings.ToUpper(j.Email),
		CellPhone:    j.CellPhone,
		TelePhone:    j.TelePhone,
		CountryId:    strconv.Itoa(countryID),
		StateId:      j.StateID,
		CityId:       j.CityID,
		DistrictId:   districtID,
	}

	addResp, err := h.tmsClient.AddMerchant(session, merchantData)
	if err != nil {
		return "Add merchant API gagal: " + err.Error()
	}
	if addResp.ResultCode != 0 {
		return "Add merchant error: " + addResp.Desc
	}

	return ""
}
