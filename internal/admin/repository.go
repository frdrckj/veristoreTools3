package admin

import (
	"strings"
	"time"

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

// FindTodayCSIs returns CSI serial numbers that were added, copied, or imported
// today. Used by partial sync to determine which CSIs to process.
func (r *Repository) FindTodayCSIs() []string {
	var logs []ActivityLog
	r.db.Where(
		"act_log_action IN ? AND DATE(created_dt) = CURDATE()",
		[]string{"VERISTORE ADD CSI", "VERISTORE DUPLICATE CSI", "VERISTORE IMPORT CSI"},
	).Find(&logs)

	seen := map[string]bool{}
	var csis []string
	for _, l := range logs {
		if l.ActLogDetail == nil {
			continue
		}
		detail := *l.ActLogDetail
		var csi string
		switch {
		case strings.HasPrefix(detail, "Add csi "):
			csi = strings.TrimPrefix(detail, "Add csi ")
		case strings.Contains(detail, " to "): // "Copy csi X to Y"
			parts := strings.Split(detail, " to ")
			csi = parts[len(parts)-1]
		case strings.HasPrefix(detail, "Import data csi "):
			csi = strings.TrimPrefix(detail, "Import data csi ")
		}
		csi = strings.TrimSpace(csi)
		if csi != "" && !seen[csi] {
			seen[csi] = true
			csis = append(csis, csi)
		}
	}
	return csis
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

// CountActiveTechnicians returns the number of technicians with tech_status = '1'.
func (r *Repository) CountActiveTechnicians() (int64, error) {
	var count int64
	if err := r.db.Model(&Technician{}).Where("tech_status = ?", "1").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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

// PopImportResult retrieves and deletes the most recent import result message
// (process_name='IMPRS'). Returns the message or empty string if none.
func (r *Repository) PopImportResult() string {
	var ql QueueLog
	err := r.db.Where("process_name = ?", "IMPRS").Order("create_time DESC").First(&ql).Error
	if err != nil {
		return ""
	}
	// Delete after reading (consume).
	r.db.Where("process_name = ?", "IMPRS").Delete(&QueueLog{})
	if ql.ServiceName != nil {
		return *ql.ServiceName
	}
	return ""
}

// PopMerchantImportResult retrieves and deletes the most recent merchant import
// result message (process_name='IMCHRS'). Returns the message or empty string if none.
func (r *Repository) PopMerchantImportResult() string {
	var ql QueueLog
	err := r.db.Where("process_name = ?", "IMCHRS").Order("create_time DESC").First(&ql).Error
	if err != nil {
		return ""
	}
	// Delete after reading (consume).
	r.db.Where("process_name = ?", "IMCHRS").Delete(&QueueLog{})
	if ql.ServiceName != nil {
		return *ql.ServiceName
	}
	return ""
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

// ==================== Import ====================

// CreateImport inserts a new import record.
func (r *Repository) CreateImport(i *Import) error {
	return r.db.Create(i).Error
}

// UpdateImportProgress updates only the progress fields of an import record.
func (r *Repository) UpdateImportProgress(id int, current, total string) error {
	return r.db.Model(&Import{}).Where("imp_id = ?", id).Updates(map[string]interface{}{
		"imp_cur_row":   current,
		"imp_total_row": total,
	}).Error
}

// FindInProgressImport finds an import that is still being processed (current != total).
func (r *Repository) FindInProgressImport() (*Import, error) {
	var i Import
	err := r.db.Where("imp_cur_row != imp_total_row").Order("imp_id DESC").First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// FindLatestImport retrieves the most recent import record.
func (r *Repository) FindLatestImport() (*Import, error) {
	var i Import
	err := r.db.Order("imp_id DESC").First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// DeleteIncompleteImports removes imports that are stuck (current != total).
func (r *Repository) DeleteIncompleteImports() error {
	return r.db.Where("imp_cur_row != imp_total_row").Delete(&Import{}).Error
}

// FindInProgressMerchantImport finds a merchant import still being processed.
func (r *Repository) FindInProgressMerchantImport() (*Import, error) {
	var i Import
	err := r.db.Where("imp_code_id = ? AND imp_cur_row != imp_total_row", "MCH").Order("imp_id DESC").First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// FindLatestMerchantImport retrieves the most recent merchant import record.
func (r *Repository) FindLatestMerchantImport() (*Import, error) {
	var i Import
	err := r.db.Where("imp_code_id = ?", "MCH").Order("imp_id DESC").First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// ==================== Export ====================

// CreateExport inserts a new export record.
func (r *Repository) CreateExport(e *Export) error {
	return r.db.Create(e).Error
}

// UpdateExport saves changes to an existing export record.
func (r *Repository) UpdateExport(e *Export) error {
	return r.db.Save(e).Error
}

// UpdateExportProgress updates only the progress fields (avoids rewriting the BLOB).
func (r *Repository) UpdateExportProgress(id int, current, total string) error {
	return r.db.Model(&Export{}).Where("exp_id = ?", id).Updates(map[string]interface{}{
		"exp_current": current,
		"exp_total":   total,
	}).Error
}

// DeleteExport removes an export record by ID.
func (r *Repository) DeleteExport(id int) error {
	return r.db.Delete(&Export{}, "exp_id = ?", id).Error
}

// FindInProgressExport finds an export that is still being processed (no data blob yet).
func (r *Repository) FindInProgressExport() (*Export, error) {
	var e Export
	err := r.db.Select("exp_id, exp_filename, exp_current, exp_total").
		Where("exp_data IS NULL").Order("exp_id DESC").First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// FindLatestExport retrieves the most recent export record.
func (r *Repository) FindLatestExport() (*Export, error) {
	var e Export
	err := r.db.Order("exp_id DESC").First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// DeleteIncompleteExports removes exports with no data (stuck/incomplete).
func (r *Repository) DeleteIncompleteExports() error {
	return r.db.Where("exp_data IS NULL").Delete(&Export{}).Error
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

// FaqsByPrivileges returns FAQ records filtered by user privileges, ordered by sequence.
func (r *Repository) FaqsByPrivileges(privileges string) ([]Faq, error) {
	var faqs []Faq
	if err := r.db.Where("faq_privileges = ?", privileges).Order("faq_seq ASC").Find(&faqs).Error; err != nil {
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

// ==================== TmsReport ====================

// CreateTmsReport inserts a new TMS report record.
func (r *Repository) CreateTmsReport(rpt *TmsReport) error {
	return r.db.Create(rpt).Error
}

// FindTmsReportByName retrieves a TMS report by name.
func (r *Repository) FindTmsReportByName(name string) (*TmsReport, error) {
	var rpt TmsReport
	if err := r.db.First(&rpt, "tms_rpt_name = ?", name).Error; err != nil {
		return nil, err
	}
	return &rpt, nil
}

// UpdateTmsReportProgress updates the current page and appends rows.
func (r *Repository) UpdateTmsReportProgress(name string, rows string, processedPages int) error {
	return r.db.Model(&TmsReport{}).Where("tms_rpt_name = ?", name).
		Updates(map[string]interface{}{
			"tms_rpt_row":      gorm.Expr("CONCAT_WS('', tms_rpt_row, ?, ',')", rows),
			"tms_rpt_cur_page": gorm.Expr("tms_rpt_cur_page + ?", processedPages),
		}).Error
}

// UpdateTmsReportFile stores the generated Excel file in the report record.
func (r *Repository) UpdateTmsReportFile(name string, fileData []byte, totalPage string) error {
	return r.db.Model(&TmsReport{}).Where("tms_rpt_name = ?", name).
		Updates(map[string]interface{}{
			"tms_rpt_file":       fileData,
			"tms_rpt_total_page": totalPage,
		}).Error
}

// ==================== SyncTerminal ====================

// CreateSyncTerminal inserts a new sync terminal record.
func (r *Repository) CreateSyncTerminal(st *SyncTerminal) error {
	return r.db.Create(st).Error
}

// HasPendingSync returns true if there are any SyncTerminal records with
// status 0, 1, or 2 (not yet completed). Used to disable buttons on the
// terminal list page while a report/sync is in progress (like v2).
//
// Stale records (status "0" older than 30 minutes) are automatically cleaned
// up to "4" (Gagal) to prevent permanently blocking buttons if a job was
// never enqueued or failed to start.
func (r *Repository) HasPendingSync() bool {
	// Auto-cleanup: mark old stuck queued records as failed.
	staleThreshold := time.Now().Add(-30 * time.Minute)
	r.db.Model(&SyncTerminal{}).
		Where("sync_term_status = ? AND sync_term_created_time < ?", "0", staleThreshold).
		Update("sync_term_status", "4")

	var count int64
	r.db.Model(&SyncTerminal{}).Where("sync_term_status IN ?", []string{"0", "1", "2"}).Count(&count)
	return count > 0
}

// HasPendingImport returns true if there is an import in progress.
func (r *Repository) HasPendingImport() bool {
	imp, _ := r.FindInProgressImport()
	return imp != nil
}

// CompletePendingSyncs marks all pending SyncTerminal records for a user as
// completed (status 3). Called when the report background job finishes.
func (r *Repository) CompletePendingSyncs(userID int) {
	r.db.Model(&SyncTerminal{}).
		Where("sync_term_creator_id = ? AND sync_term_status IN ?", userID, []string{"0", "1", "2"}).
		Update("sync_term_status", "3")
}

// UpdateSyncProcess updates the sync_term_process and sync_term_status fields
// for pending sync records of a given user. Used by background report jobs to
// report progress (e.g., "150/500") and transition status (0→1→3).
func (r *Repository) UpdateSyncProcess(userID int, process string, status string) {
	r.db.Model(&SyncTerminal{}).
		Where("sync_term_creator_id = ? AND sync_term_status IN ?", userID, []string{"0", "1", "2"}).
		Updates(map[string]interface{}{
			"sync_term_process": process,
			"sync_term_status":  status,
		})
}

// FailPendingSyncs marks all pending SyncTerminal records for a user as
// failed (status 4). Called when the report background job encounters an error.
func (r *Repository) FailPendingSyncs(userID int) {
	r.db.Model(&SyncTerminal{}).
		Where("sync_term_creator_id = ? AND sync_term_status IN ?", userID, []string{"0", "1", "2"}).
		Update("sync_term_status", "4")
}

// IsSyncCancelled returns true when no pending sync records exist for the user
// (i.e. all records have been reset to status "3" or completed). Background
// workers poll this every few seconds to detect user-initiated reset.
func (r *Repository) IsSyncCancelled(userID int) bool {
	var count int64
	r.db.Model(&SyncTerminal{}).
		Where("sync_term_creator_id = ? AND sync_term_status IN ?", userID, []string{"0", "1", "2"}).
		Count(&count)
	return count == 0
}
