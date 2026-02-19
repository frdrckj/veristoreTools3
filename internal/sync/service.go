package sync

import (
	"time"

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
