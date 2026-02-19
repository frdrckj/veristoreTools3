package tms

import (
	"gorm.io/gorm"
)

// Repository handles local database operations for TMS login and report tables.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new TMS repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetActiveLogin returns the currently enabled TMS login row (tms_login_enable = '1').
func (r *Repository) GetActiveLogin() (*TmsLogin, error) {
	var login TmsLogin
	err := r.db.Where("tms_login_enable = ?", "1").First(&login).Error
	if err != nil {
		return nil, err
	}
	return &login, nil
}

// UpdateSession updates the session token for a given TMS login ID.
func (r *Repository) UpdateSession(id int, session string) error {
	return r.db.Model(&TmsLogin{}).
		Where("tms_login_id = ?", id).
		Update("tms_login_session", session).Error
}

// FindAllLogins retrieves all TMS login rows.
func (r *Repository) FindAllLogins() ([]TmsLogin, error) {
	var logins []TmsLogin
	err := r.db.Find(&logins).Error
	return logins, err
}

// CreateLogin inserts a new TMS login row.
func (r *Repository) CreateLogin(login *TmsLogin) error {
	return r.db.Create(login).Error
}

// DeleteLogin removes a TMS login row by ID.
func (r *Repository) DeleteLogin(id int) error {
	return r.db.Delete(&TmsLogin{}, id).Error
}

// FindLoginByID retrieves a TMS login row by ID.
func (r *Repository) FindLoginByID(id int) (*TmsLogin, error) {
	var login TmsLogin
	err := r.db.First(&login, "tms_login_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &login, nil
}

// UpdateScheduled updates the scheduled JSON text for a given TMS login ID.
func (r *Repository) UpdateScheduled(id int, scheduled string) error {
	return r.db.Model(&TmsLogin{}).
		Where("tms_login_id = ?", id).
		Update("tms_login_scheduled", scheduled).Error
}

// GetReport retrieves a TMS report by name.
func (r *Repository) GetReport(name string) (*TmsReport, error) {
	var report TmsReport
	err := r.db.Where("tms_rpt_name = ?", name).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// SaveReport creates or updates a TMS report by name.
func (r *Repository) SaveReport(report *TmsReport) error {
	var existing TmsReport
	err := r.db.Where("tms_rpt_name = ?", report.TmsRptName).First(&existing).Error
	if err == nil {
		// Update existing record.
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"tms_rpt_file":       report.TmsRptFile,
			"tms_rpt_row":        report.TmsRptRow,
			"tms_rpt_cur_page":   report.TmsRptCurPage,
			"tms_rpt_total_page": report.TmsRptTotalPage,
		}).Error
	}
	return r.db.Create(report).Error
}

// DeleteReport removes a TMS report by name.
func (r *Repository) DeleteReport(name string) error {
	return r.db.Where("tms_rpt_name = ?", name).Delete(&TmsReport{}).Error
}
