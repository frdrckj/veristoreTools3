package sync

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for the SyncTerminal model.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new sync terminal repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID retrieves a sync terminal record by its unique sync_term_id.
func (r *Repository) FindByID(id int) (*SyncTerminal, error) {
	var s SyncTerminal
	if err := r.db.Where("sync_term_id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByCreatorID retrieves all sync terminal records for a given creator.
func (r *Repository) FindByCreatorID(creatorID int) ([]SyncTerminal, error) {
	var syncs []SyncTerminal
	if err := r.db.Where("sync_term_creator_id = ?", creatorID).
		Order("sync_term_created_time DESC").
		Find(&syncs).Error; err != nil {
		return nil, err
	}
	return syncs, nil
}

// SyncSearchFilter holds filter parameters for searching sync terminal records.
type SyncSearchFilter struct {
	CreatorName string // LIKE match on sync_term_creator_name
	CreatedTime string // Date match (yyyy-mm-dd) on sync_term_created_time
	Status      string // Exact match on sync_term_status
	SyncedBy    string // LIKE match on created_by
	SyncedDate  string // Date match (yyyy-mm-dd) on created_dt
}

// Search returns a paginated list of sync terminal records matching the given query.
// The query string is matched against creator name.
func (r *Repository) Search(query string, page, perPage int) ([]SyncTerminal, shared.Pagination, error) {
	return r.SearchWithFilters(SyncSearchFilter{CreatorName: query}, page, perPage)
}

// SearchWithFilters returns a paginated list of sync terminal records matching the given filters.
func (r *Repository) SearchWithFilters(filters SyncSearchFilter, page, perPage int) ([]SyncTerminal, shared.Pagination, error) {
	var syncs []SyncTerminal
	var total int64

	tx := r.db.Model(&SyncTerminal{})
	if filters.CreatorName != "" {
		tx = tx.Where("sync_term_creator_name LIKE ?", "%"+filters.CreatorName+"%")
	}
	if filters.CreatedTime != "" {
		tx = tx.Where("DATE(sync_term_created_time) = ?", filters.CreatedTime)
	}
	if filters.Status != "" {
		tx = tx.Where("sync_term_status = ?", filters.Status)
	}
	if filters.SyncedBy != "" {
		tx = tx.Where("created_by LIKE ?", "%"+filters.SyncedBy+"%")
	}
	if filters.SyncedDate != "" {
		tx = tx.Where("DATE(created_dt) = ?", filters.SyncedDate)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("sync_term_id DESC").Find(&syncs).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return syncs, p, nil
}

// Create inserts a new sync terminal record.
func (r *Repository) Create(s *SyncTerminal) error {
	return r.db.Create(s).Error
}

// Update saves changes to an existing sync terminal record.
func (r *Repository) Update(s *SyncTerminal) error {
	return r.db.Save(s).Error
}

// UpdateStatus sets the status field for the given sync terminal record.
func (r *Repository) UpdateStatus(id int, status string) error {
	return r.db.Model(&SyncTerminal{}).
		Where("sync_term_id = ?", id).
		Update("sync_term_status", status).Error
}

// LastSyncTime returns the most recent sync_term_created_time value, or an empty
// string if no sync records exist.
func (r *Repository) LastSyncTime() string {
	var s SyncTerminal
	err := r.db.Model(&SyncTerminal{}).
		Order("sync_term_created_time DESC").
		First(&s).Error
	if err != nil {
		return ""
	}
	return s.SyncTermCreatedTime.Format("2006-01-02 15:04:05")
}
