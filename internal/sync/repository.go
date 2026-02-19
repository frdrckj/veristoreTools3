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

// Search returns a paginated list of sync terminal records matching the given query.
// The query string is matched against creator name.
func (r *Repository) Search(query string, page, perPage int) ([]SyncTerminal, shared.Pagination, error) {
	var syncs []SyncTerminal
	var total int64

	tx := r.db.Model(&SyncTerminal{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("sync_term_creator_name LIKE ?", like)
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
