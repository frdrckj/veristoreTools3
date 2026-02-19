package terminal

import (
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

// FindByCSI retrieves terminals by serial number (CSI).
func (r *Repository) FindByCSI(serialNum string) ([]Terminal, error) {
	var terminals []Terminal
	if err := r.db.Where("term_serial_num = ?", serialNum).Find(&terminals).Error; err != nil {
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

// Delete removes a terminal by ID.
func (r *Repository) Delete(id int) error {
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
