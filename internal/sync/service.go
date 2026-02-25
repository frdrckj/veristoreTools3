package sync

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Service provides business logic for sync terminal operations.
type Service struct {
	repo *Repository
	db   *gorm.DB
}

// NewService creates a new sync terminal service.
func NewService(repo *Repository, db *gorm.DB) *Service {
	return &Service{
		repo: repo,
		db:   db,
	}
}

// GetSyncList returns a paginated list of sync terminal records.
func (s *Service) GetSyncList(page, perPage int) ([]SyncTerminal, int64, error) {
	syncs, pagination, err := s.repo.Search("", page, perPage)
	if err != nil {
		return nil, 0, err
	}
	return syncs, pagination.Total, nil
}

// GetSyncByID retrieves a sync terminal record by its ID.
func (s *Service) GetSyncByID(id int) (*SyncTerminal, error) {
	return s.repo.FindByID(id)
}

// CreateSync inserts a new sync terminal record with initial status.
func (s *Service) CreateSync(sync *SyncTerminal) error {
	if sync.SyncTermStatus == "" {
		sync.SyncTermStatus = "0" // Queued
	}
	if sync.SyncTermCreatedTime.IsZero() {
		sync.SyncTermCreatedTime = time.Now()
	}
	if sync.CreatedDt.IsZero() {
		sync.CreatedDt = time.Now()
	}
	return s.repo.Create(sync)
}

// DeleteSync removes a sync terminal record by ID.
func (s *Service) DeleteSync(id int) error {
	return s.db.Delete(&SyncTerminal{}, "sync_term_id = ?", id).Error
}

// ResetAllSync resets all sync terminal statuses to "3" (reset state).
func (s *Service) ResetAllSync() error {
	return s.db.Model(&SyncTerminal{}).
		Where("1 = 1").
		Update("sync_term_status", "3").Error
}

// HasPendingSync returns true if there are any sync terminal records with
// status 0 (Menunggu), 1 (Download), or 2 (Proses) — i.e., not yet completed.
func (s *Service) HasPendingSync() bool {
	var count int64
	s.db.Model(&SyncTerminal{}).
		Where("sync_term_status IN ?", []string{"0", "1", "2"}).
		Count(&count)
	return count > 0
}

// GetLastSyncTime returns the most recent sync time, or nil if no records exist.
func (s *Service) GetLastSyncTime() (*time.Time, error) {
	var sync SyncTerminal
	err := s.db.Model(&SyncTerminal{}).
		Order("sync_term_created_time DESC").
		First(&sync).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &sync.SyncTermCreatedTime, nil
}

// GetLatestReportAppVersion retrieves the app version from the most recently
// generated tms_report. It opens the Excel file and reads the sheet name,
// which follows the format "{appName}_{version}" (e.g., "CIMB BRIS_4.3.0.0").
// Returns empty string if no report exists or version cannot be extracted.
func (s *Service) GetLatestReportAppVersion() string {
	var rpt admin.TmsReport
	if err := s.db.Order("tms_rpt_id DESC").First(&rpt).Error; err != nil {
		return ""
	}
	if len(rpt.TmsRptFile) == 0 {
		return ""
	}

	f, err := excelize.OpenReader(bytes.NewReader(rpt.TmsRptFile))
	if err != nil {
		return ""
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ""
	}

	// Sheet name format: "{appName}_{version}" e.g., "CIMB BRIS_4.3.0.0"
	sheetName := sheets[0]
	lastUnderscore := strings.LastIndex(sheetName, "_")
	if lastUnderscore < 0 || lastUnderscore >= len(sheetName)-1 {
		return ""
	}

	return sheetName[lastUnderscore+1:]
}

// GetReportFileForSync finds the tms_report XLSX file associated with a sync record.
// The report name convention is "{userId}_{timestamp}.xlsx" matching the sync record's
// creator ID and created time.
func (s *Service) GetReportFileForSync(syncRec *SyncTerminal) ([]byte, string, error) {
	// Build the expected report name: {userId}_{YYYYMMDDHHmmss}.xlsx
	reportName := fmt.Sprintf("%d_%s.xlsx",
		syncRec.SyncTermCreatorID,
		syncRec.SyncTermCreatedTime.Format("20060102150405"),
	)

	var rpt admin.TmsReport
	if err := s.db.Where("tms_rpt_name = ?", reportName).First(&rpt).Error; err != nil {
		// Also try a broader search by user ID prefix in case of timing differences.
		prefix := fmt.Sprintf("%d_%%", syncRec.SyncTermCreatorID)
		if err2 := s.db.Where("tms_rpt_name LIKE ?", prefix).
			Order("tms_rpt_id DESC").First(&rpt).Error; err2 != nil {
			return nil, "", fmt.Errorf("report not found: %w", err)
		}
	}

	if len(rpt.TmsRptFile) == 0 {
		return nil, "", fmt.Errorf("report file is empty")
	}

	return rpt.TmsRptFile, rpt.TmsRptName, nil
}
