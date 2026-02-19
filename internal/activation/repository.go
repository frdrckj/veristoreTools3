package activation

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for AppActivation and AppCredential models.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new activation repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// --- AppActivation methods ---

// FindActivationByID retrieves an app activation record by ID.
func (r *Repository) FindActivationByID(id int) (*AppActivation, error) {
	var a AppActivation
	if err := r.db.First(&a, "app_act_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// SearchActivations returns a paginated list of app activation records.
// The query string is matched against CSI, TID, and MID.
func (r *Repository) SearchActivations(query string, page, perPage int) ([]AppActivation, shared.Pagination, error) {
	var activations []AppActivation
	var total int64

	tx := r.db.Model(&AppActivation{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("app_act_csi LIKE ? OR app_act_tid LIKE ? OR app_act_mid LIKE ?", like, like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("app_act_id DESC").Find(&activations).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return activations, p, nil
}

// CreateActivation inserts a new app activation record.
func (r *Repository) CreateActivation(a *AppActivation) error {
	return r.db.Create(a).Error
}

// UpdateActivation saves changes to an existing app activation record.
func (r *Repository) UpdateActivation(a *AppActivation) error {
	return r.db.Save(a).Error
}

// DeleteActivation removes an app activation record by ID.
func (r *Repository) DeleteActivation(id int) error {
	return r.db.Delete(&AppActivation{}, "app_act_id = ?", id).Error
}

// --- AppCredential methods ---

// FindCredentialByID retrieves an app credential record by ID.
func (r *Repository) FindCredentialByID(id int) (*AppCredential, error) {
	var c AppCredential
	if err := r.db.First(&c, "app_cred_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// SearchCredentials returns a paginated list of app credential records.
// The query string is matched against user and name fields.
func (r *Repository) SearchCredentials(query string, page, perPage int) ([]AppCredential, shared.Pagination, error) {
	var creds []AppCredential
	var total int64

	tx := r.db.Model(&AppCredential{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("app_cred_user LIKE ? OR app_cred_name LIKE ?", like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("app_cred_id DESC").Find(&creds).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return creds, p, nil
}

// CreateCredential inserts a new app credential record.
func (r *Repository) CreateCredential(c *AppCredential) error {
	return r.db.Create(c).Error
}

// UpdateCredential saves changes to an existing app credential record.
func (r *Repository) UpdateCredential(c *AppCredential) error {
	return r.db.Save(c).Error
}

// DeleteCredential removes an app credential record by ID.
func (r *Repository) DeleteCredential(id int) error {
	return r.db.Delete(&AppCredential{}, "app_cred_id = ?", id).Error
}
