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
