package admin

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for admin-domain models including
// ActivityLog, Technician, TemplateParameter, QueueLog, Export, ExportResult,
// TidNote, Faq, Hash, and Session.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new admin repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ==================== ActivityLog ====================

// CreateActivityLog inserts a new activity log record.
func (r *Repository) CreateActivityLog(l *ActivityLog) error {
	return r.db.Create(l).Error
}

// FindActivityLogByID retrieves an activity log record by ID.
func (r *Repository) FindActivityLogByID(id int) (*ActivityLog, error) {
	var l ActivityLog
	if err := r.db.First(&l, "act_log_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

// SearchActivityLogs returns a paginated list of activity logs.
func (r *Repository) SearchActivityLogs(query string, page, perPage int) ([]ActivityLog, shared.Pagination, error) {
	var logs []ActivityLog
	var total int64

	tx := r.db.Model(&ActivityLog{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("act_log_action LIKE ? OR act_log_detail LIKE ? OR created_by LIKE ?", like, like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("act_log_id DESC").Find(&logs).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return logs, p, nil
}

// DeleteActivityLog removes an activity log record by ID.
func (r *Repository) DeleteActivityLog(id int) error {
	return r.db.Delete(&ActivityLog{}, "act_log_id = ?", id).Error
}

// ==================== Technician ====================

// FindTechnicianByID retrieves a technician by ID.
func (r *Repository) FindTechnicianByID(id int) (*Technician, error) {
	var t Technician
	if err := r.db.First(&t, "tech_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// SearchTechnicians returns a paginated list of technicians.
func (r *Repository) SearchTechnicians(query string, page, perPage int) ([]Technician, shared.Pagination, error) {
	var techs []Technician
	var total int64

	tx := r.db.Model(&Technician{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("tech_name LIKE ? OR tech_number LIKE ? OR tech_company LIKE ?", like, like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("tech_id DESC").Find(&techs).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return techs, p, nil
}

// CreateTechnician inserts a new technician record.
func (r *Repository) CreateTechnician(t *Technician) error {
	return r.db.Create(t).Error
}

// UpdateTechnician saves changes to an existing technician record.
func (r *Repository) UpdateTechnician(t *Technician) error {
	return r.db.Save(t).Error
}

// DeleteTechnician removes a technician by ID.
func (r *Repository) DeleteTechnician(id int) error {
	return r.db.Delete(&Technician{}, "tech_id = ?", id).Error
}

// ==================== TemplateParameter ====================

// FindTemplateParameterByID retrieves a template parameter by ID.
func (r *Repository) FindTemplateParameterByID(id int) (*TemplateParameter, error) {
	var tp TemplateParameter
	if err := r.db.First(&tp, "tparam_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tp, nil
}

// AllTemplateParameters returns all template parameters ordered by index.
func (r *Repository) AllTemplateParameters() ([]TemplateParameter, error) {
	var params []TemplateParameter
	if err := r.db.Order("tparam_index ASC").Find(&params).Error; err != nil {
		return nil, err
	}
	return params, nil
}

// CreateTemplateParameter inserts a new template parameter record.
func (r *Repository) CreateTemplateParameter(tp *TemplateParameter) error {
	return r.db.Create(tp).Error
}

// UpdateTemplateParameter saves changes to an existing template parameter.
func (r *Repository) UpdateTemplateParameter(tp *TemplateParameter) error {
	return r.db.Save(tp).Error
}

// DeleteTemplateParameter removes a template parameter by ID.
func (r *Repository) DeleteTemplateParameter(id int) error {
	return r.db.Delete(&TemplateParameter{}, "tparam_id = ?", id).Error
}

// ==================== QueueLog ====================

// CreateQueueLog inserts a new queue log record.
func (r *Repository) CreateQueueLog(ql *QueueLog) error {
	return r.db.Create(ql).Error
}

// SearchQueueLogs returns a paginated list of queue logs.
func (r *Repository) SearchQueueLogs(query string, page, perPage int) ([]QueueLog, shared.Pagination, error) {
	var logs []QueueLog
	var total int64

	tx := r.db.Model(&QueueLog{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("service_name LIKE ? OR process_name LIKE ?", like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("create_time DESC").Find(&logs).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return logs, p, nil
}

// DeleteQueueLog removes a queue log record by its composite key.
func (r *Repository) DeleteQueueLog(createTime, processName string) error {
	return r.db.Where("create_time = ? AND process_name = ?", createTime, processName).
		Delete(&QueueLog{}).Error
}

// ==================== Export ====================

// FindExportByID retrieves an export record by ID.
func (r *Repository) FindExportByID(id int) (*Export, error) {
	var e Export
	if err := r.db.First(&e, "exp_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// CreateExport inserts a new export record.
func (r *Repository) CreateExport(e *Export) error {
	return r.db.Create(e).Error
}

// UpdateExport saves changes to an existing export record.
func (r *Repository) UpdateExport(e *Export) error {
	return r.db.Save(e).Error
}

// DeleteExport removes an export record by ID.
func (r *Repository) DeleteExport(id int) error {
	return r.db.Delete(&Export{}, "exp_id = ?", id).Error
}

// ==================== ExportResult ====================

// FindExportResultByID retrieves an export result record by ID.
func (r *Repository) FindExportResultByID(id int) (*ExportResult, error) {
	var er ExportResult
	if err := r.db.First(&er, "exp_res_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &er, nil
}

// CreateExportResult inserts a new export result record.
func (r *Repository) CreateExportResult(er *ExportResult) error {
	return r.db.Create(er).Error
}

// DeleteExportResult removes an export result record by ID.
func (r *Repository) DeleteExportResult(id int) error {
	return r.db.Delete(&ExportResult{}, "exp_res_id = ?", id).Error
}

// ==================== TidNote ====================

// FindTidNoteByID retrieves a TID note by ID.
func (r *Repository) FindTidNoteByID(id int) (*TidNote, error) {
	var tn TidNote
	if err := r.db.First(&tn, "tid_note_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tn, nil
}

// SearchTidNotes returns a paginated list of TID notes.
func (r *Repository) SearchTidNotes(query string, page, perPage int) ([]TidNote, shared.Pagination, error) {
	var notes []TidNote
	var total int64

	tx := r.db.Model(&TidNote{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("tid_note_serial_num LIKE ? OR tid_note_data LIKE ?", like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("tid_note_id DESC").Find(&notes).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return notes, p, nil
}

// CreateTidNote inserts a new TID note record.
func (r *Repository) CreateTidNote(tn *TidNote) error {
	return r.db.Create(tn).Error
}

// UpdateTidNote saves changes to an existing TID note.
func (r *Repository) UpdateTidNote(tn *TidNote) error {
	return r.db.Save(tn).Error
}

// DeleteTidNote removes a TID note by ID.
func (r *Repository) DeleteTidNote(id int) error {
	return r.db.Delete(&TidNote{}, "tid_note_id = ?", id).Error
}

// ==================== Faq ====================

// FindFaqByID retrieves a FAQ record by ID.
func (r *Repository) FindFaqByID(id int) (*Faq, error) {
	var f Faq
	if err := r.db.First(&f, "faq_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

// AllFaqs returns all FAQ records ordered by sequence.
func (r *Repository) AllFaqs() ([]Faq, error) {
	var faqs []Faq
	if err := r.db.Order("faq_seq ASC").Find(&faqs).Error; err != nil {
		return nil, err
	}
	return faqs, nil
}

// CreateFaq inserts a new FAQ record.
func (r *Repository) CreateFaq(f *Faq) error {
	return r.db.Create(f).Error
}

// UpdateFaq saves changes to an existing FAQ record.
func (r *Repository) UpdateFaq(f *Faq) error {
	return r.db.Save(f).Error
}

// DeleteFaq removes a FAQ record by ID.
func (r *Repository) DeleteFaq(id int) error {
	return r.db.Delete(&Faq{}, "faq_id = ?", id).Error
}
