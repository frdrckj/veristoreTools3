package terminal

import (
	"time"

	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for Terminal and TerminalParameter models.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new terminal repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID retrieves a terminal by its primary key.
func (r *Repository) FindByID(id int) (*Terminal, error) {
	var t Terminal
	if err := r.db.First(&t, "term_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindByCSI retrieves terminals by CSI (stored in term_device_id).
// Also checks term_serial_num as fallback for terminals where SN was empty
// and CSI was stored there instead.
func (r *Repository) FindByCSI(csi string) ([]Terminal, error) {
	var terminals []Terminal
	if err := r.db.Where("term_device_id = ? OR term_serial_num = ?", csi, csi).Find(&terminals).Error; err != nil {
		return nil, err
	}
	return terminals, nil
}

// FindByDeviceID retrieves terminals by device ID.
func (r *Repository) FindByDeviceID(deviceID string) ([]Terminal, error) {
	var terminals []Terminal
	if err := r.db.Where("term_device_id = ?", deviceID).Find(&terminals).Error; err != nil {
		return nil, err
	}
	return terminals, nil
}

// Search returns a paginated list of terminals matching the given query.
// The query string is matched against term_serial_num, term_device_id, and term_app_name.
func (r *Repository) Search(query string, page, perPage int) ([]Terminal, shared.Pagination, error) {
	var terminals []Terminal
	var total int64

	tx := r.db.Model(&Terminal{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("term_serial_num LIKE ? OR term_device_id LIKE ? OR term_app_name LIKE ?", like, like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("term_id DESC").Find(&terminals).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return terminals, p, nil
}

// Create inserts a new terminal record.
func (r *Repository) Create(t *Terminal) error {
	return r.db.Create(t).Error
}

// Update saves changes to an existing terminal record.
func (r *Repository) Update(t *Terminal) error {
	return r.db.Save(t).Error
}

// Delete removes a terminal and its associated parameters by ID.
func (r *Repository) Delete(id int) error {
	// Delete child parameters first to avoid foreign key constraint error.
	r.db.Where("param_term_id = ?", id).Delete(&TerminalParameter{})
	return r.db.Delete(&Terminal{}, "term_id = ?", id).Error
}

// Count returns the total number of terminal records.
func (r *Repository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&Terminal{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// --- TerminalParameter methods ---

// FindParametersByTermID retrieves all parameters for a given terminal.
func (r *Repository) FindParametersByTermID(termID int) ([]TerminalParameter, error) {
	var params []TerminalParameter
	if err := r.db.Where("param_term_id = ?", termID).Find(&params).Error; err != nil {
		return nil, err
	}
	return params, nil
}

// CreateParameter inserts a new terminal parameter record.
func (r *Repository) CreateParameter(p *TerminalParameter) error {
	return r.db.Create(p).Error
}

// UpdateParameter saves changes to an existing terminal parameter.
func (r *Repository) UpdateParameter(p *TerminalParameter) error {
	return r.db.Save(p).Error
}

// FindParameterByID retrieves a single terminal parameter by its primary key.
func (r *Repository) FindParameterByID(id int) (*TerminalParameter, error) {
	var p TerminalParameter
	if err := r.db.Preload("Terminal").First(&p, "param_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// SearchParameters returns a paginated list of terminal parameters matching the query.
// The query string is matched against param_tid, param_mid, and param_host_name.
func (r *Repository) SearchParameters(query string, page, perPage int) ([]TerminalParameter, shared.Pagination, error) {
	var params []TerminalParameter
	var total int64

	tx := r.db.Model(&TerminalParameter{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("param_tid LIKE ? OR param_mid LIKE ? OR param_host_name LIKE ?", like, like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("param_id DESC").Find(&params).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return params, p, nil
}

// DeleteParameter removes a terminal parameter by ID.
func (r *Repository) DeleteParameter(id int) error {
	return r.db.Delete(&TerminalParameter{}, "param_id = ?", id).Error
}

// TerminalFilter holds per-column filter values for the Data CSI page.
type TerminalFilter struct {
	CSI           string
	SerialNumber  string
	ProductNumber string
	Model         string
	AppVersion    string
}

// SearchFiltered returns a paginated list of terminals matching per-column filters (like V2's Data CSI page).
func (r *Repository) SearchFiltered(filters TerminalFilter, page, perPage int) ([]Terminal, shared.Pagination, error) {
	var terminals []Terminal
	var total int64

	tx := r.db.Model(&Terminal{})
	if filters.CSI != "" {
		tx = tx.Where("term_serial_num LIKE ?", "%"+filters.CSI+"%")
	}
	if filters.SerialNumber != "" {
		tx = tx.Where("term_device_id LIKE ?", "%"+filters.SerialNumber+"%")
	}
	if filters.ProductNumber != "" {
		tx = tx.Where("term_product_num LIKE ?", "%"+filters.ProductNumber+"%")
	}
	if filters.Model != "" {
		tx = tx.Where("term_model = ?", filters.Model)
	}
	if filters.AppVersion != "" {
		tx = tx.Where("term_app_version = ?", filters.AppVersion)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("term_id DESC").Find(&terminals).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return terminals, p, nil
}

// DistinctModels returns all unique terminal model values (non-empty).
func (r *Repository) DistinctModels() ([]string, error) {
	var models []string
	if err := r.db.Model(&Terminal{}).Distinct("term_model").Where("term_model != ''").Order("term_model ASC").Pluck("term_model", &models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// DistinctAppVersions returns all unique app version values (non-empty), ordered descending.
func (r *Repository) DistinctAppVersions() ([]string, error) {
	var versions []string
	if err := r.db.Model(&Terminal{}).Distinct("term_app_version").Where("term_app_version != ''").Order("term_app_version DESC").Pluck("term_app_version", &versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// DeleteStaleSynced deletes terminals that were previously synced (last_synced_at
// is NOT NULL) but were not included in the current sync (last_synced_at is older
// than syncStart). Also deletes their associated terminal_parameter records.
func (r *Repository) DeleteStaleSynced(syncStart time.Time) (int64, error) {
	// Find stale terminal IDs.
	var staleIDs []int
	if err := r.db.Model(&Terminal{}).
		Where("last_synced_at IS NOT NULL AND last_synced_at < ?", syncStart).
		Pluck("term_id", &staleIDs).Error; err != nil {
		return 0, err
	}
	if len(staleIDs) == 0 {
		return 0, nil
	}

	// Delete their parameters first.
	r.db.Where("param_term_id IN ?", staleIDs).Delete(&TerminalParameter{})

	// Delete the terminals.
	result := r.db.Where("term_id IN ?", staleIDs).Delete(&Terminal{})
	return result.RowsAffected, result.Error
}
