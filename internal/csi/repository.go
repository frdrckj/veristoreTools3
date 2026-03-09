package csi

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for the VerificationReport model.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new CSI verification report repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID retrieves a verification report by its primary key.
func (r *Repository) FindByID(id int) (*VerificationReport, error) {
	var rpt VerificationReport
	if err := r.db.First(&rpt, "vfi_rpt_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rpt, nil
}

// Search returns a paginated list of verification reports matching the given query.
// The query string is matched against serial number, device ID, and tech name.
func (r *Repository) Search(query string, page, perPage int) ([]VerificationReport, shared.Pagination, error) {
	var reports []VerificationReport
	var total int64

	tx := r.db.Model(&VerificationReport{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where(
			"vfi_rpt_term_serial_num LIKE ? OR vfi_rpt_term_device_id LIKE ? OR vfi_rpt_tech_name LIKE ?",
			like, like, like,
		)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("vfi_rpt_id DESC").Find(&reports).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return reports, p, nil
}

// ReportFilter holds filter parameters for verification report search (matching v2).
type ReportFilter struct {
	DateFrom     string // YYYY-MM-DD
	DateTo       string // YYYY-MM-DD
	CSI          string // vfi_rpt_term_device_id (LIKE)
	SerialNumber string // vfi_rpt_term_serial_num (LIKE)
	EdcType      string // vfi_rpt_term_model (exact)
	AppVersion   string // vfi_rpt_term_app_version (exact)
	Technician   string // vfi_rpt_tech_name (exact)
	TMSOperator  string // vfi_rpt_term_tms_create_operator (exact)
	VfiOperator  string // created_by (exact)
}

// SearchFiltered returns a paginated list of verification reports with individual filters.
func (r *Repository) SearchFiltered(f ReportFilter, page, perPage int) ([]VerificationReport, shared.Pagination, error) {
	var reports []VerificationReport
	var total int64

	tx := r.db.Model(&VerificationReport{})
	if f.DateFrom != "" {
		tx = tx.Where("created_dt >= ?", f.DateFrom+" 00:00:00")
	}
	if f.DateTo != "" {
		tx = tx.Where("created_dt <= ?", f.DateTo+" 23:59:59")
	}
	if f.CSI != "" {
		tx = tx.Where("vfi_rpt_term_device_id LIKE ?", "%"+f.CSI+"%")
	}
	if f.SerialNumber != "" {
		tx = tx.Where("vfi_rpt_term_serial_num LIKE ?", "%"+f.SerialNumber+"%")
	}
	if f.EdcType != "" {
		tx = tx.Where("vfi_rpt_term_model = ?", f.EdcType)
	}
	if f.AppVersion != "" {
		tx = tx.Where("vfi_rpt_term_app_version = ?", f.AppVersion)
	}
	if f.Technician != "" {
		tx = tx.Where("vfi_rpt_tech_name = ?", f.Technician)
	}
	if f.TMSOperator != "" {
		tx = tx.Where("vfi_rpt_term_tms_create_operator = ?", f.TMSOperator)
	}
	if f.VfiOperator != "" {
		tx = tx.Where("created_by = ?", f.VfiOperator)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("vfi_rpt_id DESC").Find(&reports).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return reports, p, nil
}

// FindAllFiltered returns all verification reports matching the given filters
// without pagination. Used for export.
func (r *Repository) FindAllFiltered(f ReportFilter) ([]VerificationReport, error) {
	var reports []VerificationReport

	tx := r.db.Model(&VerificationReport{})
	if f.DateFrom != "" {
		tx = tx.Where("created_dt >= ?", f.DateFrom+" 00:00:00")
	}
	if f.DateTo != "" {
		tx = tx.Where("created_dt <= ?", f.DateTo+" 23:59:59")
	}
	if f.CSI != "" {
		tx = tx.Where("vfi_rpt_term_device_id LIKE ?", "%"+f.CSI+"%")
	}
	if f.SerialNumber != "" {
		tx = tx.Where("vfi_rpt_term_serial_num LIKE ?", "%"+f.SerialNumber+"%")
	}
	if f.EdcType != "" {
		tx = tx.Where("vfi_rpt_term_model = ?", f.EdcType)
	}
	if f.AppVersion != "" {
		tx = tx.Where("vfi_rpt_term_app_version = ?", f.AppVersion)
	}
	if f.Technician != "" {
		tx = tx.Where("vfi_rpt_tech_name = ?", f.Technician)
	}
	if f.TMSOperator != "" {
		tx = tx.Where("vfi_rpt_term_tms_create_operator = ?", f.TMSOperator)
	}
	if f.VfiOperator != "" {
		tx = tx.Where("created_by = ?", f.VfiOperator)
	}

	if err := tx.Order("vfi_rpt_id DESC").Find(&reports).Error; err != nil {
		return nil, err
	}

	return reports, nil
}

// GetDistinctModels returns distinct terminal models from the verification report table.
func (r *Repository) GetDistinctModels() []string {
	var vals []string
	r.db.Model(&VerificationReport{}).Distinct("vfi_rpt_term_model").Where("vfi_rpt_term_model != ''").Order("vfi_rpt_term_model ASC").Pluck("vfi_rpt_term_model", &vals)
	return vals
}

// GetDistinctAppVersions returns distinct app versions from the verification report table.
func (r *Repository) GetDistinctAppVersions() []string {
	var vals []string
	r.db.Model(&VerificationReport{}).Distinct("vfi_rpt_term_app_version").Where("vfi_rpt_term_app_version != ''").Order("vfi_rpt_term_app_version ASC").Pluck("vfi_rpt_term_app_version", &vals)
	return vals
}

// GetDistinctTechnicians returns distinct technician names from the verification report table.
func (r *Repository) GetDistinctTechnicians() []string {
	var vals []string
	r.db.Model(&VerificationReport{}).Distinct("vfi_rpt_tech_name").Where("vfi_rpt_tech_name != ''").Order("vfi_rpt_tech_name ASC").Pluck("vfi_rpt_tech_name", &vals)
	return vals
}

// GetDistinctTMSOperators returns distinct TMS operators from the verification report table.
func (r *Repository) GetDistinctTMSOperators() []string {
	var vals []string
	r.db.Model(&VerificationReport{}).Distinct("vfi_rpt_term_tms_create_operator").Where("vfi_rpt_term_tms_create_operator != ''").Order("vfi_rpt_term_tms_create_operator ASC").Pluck("vfi_rpt_term_tms_create_operator", &vals)
	return vals
}

// GetDistinctVfiOperators returns distinct verification operators (created_by) from the verification report table.
func (r *Repository) GetDistinctVfiOperators() []string {
	var vals []string
	r.db.Model(&VerificationReport{}).Distinct("created_by").Where("created_by != ''").Order("created_by ASC").Pluck("created_by", &vals)
	return vals
}

// Create inserts a new verification report record.
func (r *Repository) Create(rpt *VerificationReport) error {
	return r.db.Create(rpt).Error
}

// Update saves changes to an existing verification report.
func (r *Repository) Update(rpt *VerificationReport) error {
	return r.db.Save(rpt).Error
}

// Delete removes a verification report by ID.
func (r *Repository) Delete(id int) error {
	return r.db.Delete(&VerificationReport{}, "vfi_rpt_id = ?", id).Error
}

// CountDistinctVerified returns the number of distinct terminal serial numbers
// that have verification reports. This mirrors the v2 query:
//
//	SELECT COUNT(DISTINCT vfi_rpt_term_serial_num) FROM verification_report
func (r *Repository) CountDistinctVerified() (int64, error) {
	var count int64
	if err := r.db.Model(&VerificationReport{}).
		Distinct("vfi_rpt_term_serial_num").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
