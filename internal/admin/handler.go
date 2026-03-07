package admin

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	adminTmpl "github.com/verifone/veristoretools3/templates/admin"
)

// Handler holds dependencies for admin HTTP handlers (activity log, technician,
// FAQ, backup).
type Handler struct {
	repo        *Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
	faqDir      string // directory containing FAQ HTML content files
	assetsDir   string // directory containing downloadable assets (user guides)
}

// NewHandler creates a new admin handler.
func NewHandler(repo *Repository, store sessions.Store, sessionName, appName, appVersion, faqDir, assetsDir string) *Handler {
	return &Handler{
		repo:        repo,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
		faqDir:      faqDir,
		assetsDir:   assetsDir,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:            title,
		AppName:          h.appName,
		AppVersion:       h.appVersion,
		AppIcon:          "favicon.png",
		AppLogo:          "verifone_logo.png",
		AppVeristoreLogo: "veristore_logo.png",
		UserName:         mw.GetCurrentUserName(c),
		UserFullname:     mw.GetCurrentUserFullname(c),
		UserPrivileges:   mw.GetCurrentUserPrivileges(c),
		CopyrightTitle:   "Verifone",
		CopyrightURL:     "https://www.verifone.com",
		Flashes:          flashes,
	}
}

// ---------------------------------------------------------------------------
// Activity Log
// ---------------------------------------------------------------------------

// toActivityLogData converts an ActivityLog model to a view struct.
func toActivityLogData(l ActivityLog) adminTmpl.ActivityLogData {
	detail := ""
	if l.ActLogDetail != nil {
		detail = *l.ActLogDetail
	}
	return adminTmpl.ActivityLogData{
		ActLogID:     l.ActLogID,
		ActLogAction: l.ActLogAction,
		ActLogDetail: detail,
		CreatedBy:    l.CreatedBy,
		CreatedDt:    l.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toActivityLogDataSlice converts a slice of ActivityLog models to view structs.
func toActivityLogDataSlice(logs []ActivityLog) []adminTmpl.ActivityLogData {
	result := make([]adminTmpl.ActivityLogData, len(logs))
	for i, l := range logs {
		result[i] = toActivityLogData(l)
	}
	return result
}

// ActivityLogIndex lists activity logs with search and pagination.
func (h *Handler) ActivityLogIndex(c echo.Context) error {
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	filter := ActivityLogFilter{
		Action:   c.QueryParam("action"),
		Detail:   c.QueryParam("detail"),
		User:     c.QueryParam("user"),
		DateFrom: c.QueryParam("dateFrom"),
		DateTo:   c.QueryParam("dateTo"),
	}

	logs, pagination, err := h.repo.SearchActivityLogsFiltered(filter, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load activity logs")
	}

	// Build query string for pagination links to preserve filters.
	var qs string
	if filter.Action != "" {
		qs += "&action=" + filter.Action
	}
	if filter.Detail != "" {
		qs += "&detail=" + filter.Detail
	}
	if filter.User != "" {
		qs += "&user=" + filter.User
	}
	if filter.DateFrom != "" {
		qs += "&dateFrom=" + filter.DateFrom
	}
	if filter.DateTo != "" {
		qs += "&dateTo=" + filter.DateTo
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/activitylog/index",
		HTMXTarget:  "actlog-table-container",
		QueryString: qs,
	}

	page := h.pageData(c, "Activity Log")
	logData := toActivityLogDataSlice(logs)
	users := h.repo.GetActivityLogUsers()

	filterData := adminTmpl.ActivityLogFilterData{
		Action:   filter.Action,
		Detail:   filter.Detail,
		User:     filter.User,
		DateFrom: filter.DateFrom,
		DateTo:   filter.DateTo,
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, adminTmpl.ActivityLogTablePartial(logData, paginationData, filterData, users))
	}

	return shared.Render(c, http.StatusOK, adminTmpl.ActivityLogIndexPage(page, logData, paginationData, filterData, users))
}

// ActivityLogView displays activity log details by ID.
func (h *Handler) ActivityLogView(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid activity log ID")
	}

	l, err := h.repo.FindActivityLogByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "activity log not found")
	}

	page := h.pageData(c, "Activity Log Detail")
	return shared.Render(c, http.StatusOK, adminTmpl.ActivityLogViewPage(page, toActivityLogData(*l)))
}

// ---------------------------------------------------------------------------
// Technician
// ---------------------------------------------------------------------------

// toTechnicianData converts a Technician model to a view struct.
func toTechnicianData(t Technician) adminTmpl.TechnicianData {
	updatedBy := ""
	if t.UpdatedBy != nil {
		updatedBy = *t.UpdatedBy
	}
	updatedDt := ""
	if t.UpdatedDt != nil {
		updatedDt = t.UpdatedDt.Format("2006-01-02 15:04:05")
	}
	return adminTmpl.TechnicianData{
		TechID:           t.TechID,
		TechName:         t.TechName,
		TechNip:          t.TechNip,
		TechNumber:       t.TechNumber,
		TechAddress:      t.TechAddress,
		TechCompany:      t.TechCompany,
		TechServicePoint: t.TechSercivePoint,
		TechPhone:        t.TechPhone,
		TechGender:       t.TechGender,
		TechStatus:       t.TechStatus,
		CreatedBy:        t.CreatedBy,
		CreatedDt:        t.CreatedDt.Format("2006-01-02 15:04:05"),
		UpdatedBy:        updatedBy,
		UpdatedDt:        updatedDt,
	}
}

// toTechnicianDataSlice converts a slice of Technician models to view structs.
func toTechnicianDataSlice(techs []Technician) []adminTmpl.TechnicianData {
	result := make([]adminTmpl.TechnicianData, len(techs))
	for i, t := range techs {
		result[i] = toTechnicianData(t)
	}
	return result
}

// TechnicianIndex lists technicians with search and pagination.
func (h *Handler) TechnicianIndex(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	techs, pagination, err := h.repo.SearchTechnicians(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load technicians")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/technician/index",
		HTMXTarget:  "tech-table-container",
	}

	page := h.pageData(c, "Technicians")
	techData := toTechnicianDataSlice(techs)

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianTablePartial(techData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, adminTmpl.TechnicianIndexPage(page, techData, paginationData, query))
}

// TechnicianView displays technician details by ID.
func (h *Handler) TechnicianView(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid technician ID")
	}

	t, err := h.repo.FindTechnicianByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "technician not found")
	}

	page := h.pageData(c, "Technician Detail")
	return shared.Render(c, http.StatusOK, adminTmpl.TechnicianViewPage(page, toTechnicianData(*t)))
}

// TechnicianCreate handles GET (show form) and POST (process form) for creating a technician.
func (h *Handler) TechnicianCreate(c echo.Context) error {
	page := h.pageData(c, "Create Technician")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, adminTmpl.TechnicianData{}, false, nil))
	}

	t, errors := h.parseTechnicianForm(c)
	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, toTechnicianData(*t), false, errors))
	}

	t.CreatedBy = mw.GetCurrentUserName(c)
	t.CreatedDt = time.Now()

	if err := h.repo.CreateTechnician(t); err != nil {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, toTechnicianData(*t), false, []string{err.Error()}))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreAddTech, "Add technician "+t.TechName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Technician created successfully")
	return c.Redirect(http.StatusFound, "/technician/index")
}

// TechnicianUpdate handles GET (show form) and POST (process form) for editing a technician.
func (h *Handler) TechnicianUpdate(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid technician ID")
	}

	existing, err := h.repo.FindTechnicianByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "technician not found")
	}

	page := h.pageData(c, "Edit Technician")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, toTechnicianData(*existing), true, nil))
	}

	updated, errors := h.parseTechnicianForm(c)
	if len(errors) > 0 {
		updated.TechID = existing.TechID
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, toTechnicianData(*updated), true, errors))
	}

	existing.TechName = updated.TechName
	existing.TechNip = updated.TechNip
	existing.TechNumber = updated.TechNumber
	existing.TechAddress = updated.TechAddress
	existing.TechCompany = updated.TechCompany
	existing.TechSercivePoint = updated.TechSercivePoint
	existing.TechPhone = updated.TechPhone
	existing.TechGender = updated.TechGender
	existing.TechStatus = updated.TechStatus

	now := time.Now()
	userName := mw.GetCurrentUserName(c)
	existing.UpdatedBy = &userName
	existing.UpdatedDt = &now

	if err := h.repo.UpdateTechnician(existing); err != nil {
		return shared.Render(c, http.StatusOK, adminTmpl.TechnicianFormPage(page, toTechnicianData(*existing), true, []string{err.Error()}))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreEditTech, "Edit technician "+existing.TechName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Technician updated successfully")
	return c.Redirect(http.StatusFound, "/technician/index")
}

// TechnicianDelete removes a technician by ID.
func (h *Handler) TechnicianDelete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid technician ID")
	}

	if err := h.repo.DeleteTechnician(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete technician: %v", err))
	} else {
		mw.LogActivityFromContext(c, mw.LogVeristoreDelTech, fmt.Sprintf("Delete technician ID %d", id))
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Technician deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/technician/index")
}

// parseTechnicianForm extracts form values and validates required fields.
func (h *Handler) parseTechnicianForm(c echo.Context) (*Technician, []string) {
	t := &Technician{
		TechName:         c.FormValue("tech_name"),
		TechNip:          c.FormValue("tech_nip"),
		TechNumber:       c.FormValue("tech_number"),
		TechAddress:      c.FormValue("tech_address"),
		TechCompany:      c.FormValue("tech_company"),
		TechSercivePoint: c.FormValue("tech_service_point"),
		TechPhone:        c.FormValue("tech_phone"),
		TechGender:       c.FormValue("tech_gender"),
		TechStatus:       c.FormValue("tech_status"),
	}
	if t.TechStatus == "" {
		t.TechStatus = "1"
	}

	var errors []string
	if t.TechName == "" {
		errors = append(errors, "Nama is required")
	}
	if t.TechNip == "" {
		errors = append(errors, "NIP is required")
	}
	if t.TechNumber == "" {
		errors = append(errors, "ID Number (KTP) is required")
	}
	if t.TechAddress == "" {
		errors = append(errors, "Alamat is required")
	}
	if t.TechCompany == "" {
		errors = append(errors, "Perusahaan is required")
	}
	if t.TechSercivePoint == "" {
		errors = append(errors, "Service Point is required")
	}
	if t.TechPhone == "" {
		errors = append(errors, "Telepon is required")
	}
	if t.TechGender == "" {
		errors = append(errors, "Jenis Kelamin is required")
	}

	return t, errors
}

// ---------------------------------------------------------------------------
// FAQ
// ---------------------------------------------------------------------------

// FaqIndex displays the FAQ list page.
func (h *Handler) FaqIndex(c echo.Context) error {
	privileges := mw.GetCurrentUserPrivileges(c)
	faqs, err := h.repo.FaqsByPrivileges(privileges)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load FAQs")
	}

	// Build a flat list then assemble into a tree.
	type flatFaq struct {
		id       int
		parentID int
		name     string
		link     string
		seq      int
	}
	var flat []flatFaq
	for _, f := range faqs {
		link := ""
		if f.FaqLink != nil {
			link = *f.FaqLink
		}
		pid := 0
		if f.FaqParent != nil {
			pid = *f.FaqParent
		}
		flat = append(flat, flatFaq{id: f.FaqID, parentID: pid, name: f.FaqName, link: link, seq: f.FaqSeq})
	}

	// Build tree recursively.
	var buildTree func(parentID int) []adminTmpl.FaqData
	buildTree = func(parentID int) []adminTmpl.FaqData {
		var nodes []adminTmpl.FaqData
		for _, f := range flat {
			if f.parentID == parentID {
				nodes = append(nodes, adminTmpl.FaqData{
					FaqID:    f.id,
					FaqName:  f.name,
					FaqLink:  f.link,
					FaqSeq:   f.seq,
					ParentID: f.parentID,
					Children: buildTree(f.id),
				})
			}
		}
		return nodes
	}
	tree := buildTree(0)

	// If a page param is present, load the HTML content file.
	var faqTitle, faqContent string
	if pageName := c.QueryParam("page"); pageName != "" {
		faqTitle = c.QueryParam("title")
		contentFile := filepath.Join(h.faqDir, pageName+".html")
		if data, err := os.ReadFile(contentFile); err == nil {
			faqContent = string(data)
		}
	}

	page := h.pageData(c, "Bantuan")
	return shared.Render(c, http.StatusOK, adminTmpl.FaqPage(page, tree, faqTitle, faqContent))
}

// FaqContent returns a partial HTML fragment for a single FAQ topic.
// It is called via AJAX from the FAQ page (matching V2 Pjax behaviour).
func (h *Handler) FaqContent(c echo.Context) error {
	pageName := c.QueryParam("page")
	faqTitle := c.QueryParam("title")
	if pageName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "page parameter is required")
	}

	contentFile := filepath.Join(h.faqDir, pageName+".html")
	data, err := os.ReadFile(contentFile)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "FAQ content not found")
	}

	// Return a card fragment matching V2's box box-success styling:
	// green top border, white header with h2 title and bottom border, then body.
	html := `<div class="card card-success card-outline">` +
		`<div class="card-header"><h2 class="card-title">` + faqTitle + `</h2></div>` +
		`<div class="card-body">` + string(data) + `</div>` +
		`</div>`
	return c.HTML(http.StatusOK, html)
}

// FaqDownload serves the role-specific user guide PDF (matching V2).
func (h *Handler) FaqDownload(c echo.Context) error {
	privileges := mw.GetCurrentUserPrivileges(c)
	var filename string
	switch privileges {
	case "ADMIN":
		filename = "User Guide Veristore Tools Verifikasi CSI (Administrator) English.pdf"
	case "OPERATOR":
		filename = "User Guide Veristore Tools Verifikasi CSI (Operator) English.pdf"
	case "TMS ADMIN":
		filename = "User Guide Veristore Tools Profiling (Administrator) English.pdf"
	case "TMS SUPERVISOR":
		filename = "User Guide Veristore Tools Profiling (Supervisor) English.pdf"
	case "TMS OPERATOR":
		filename = "User Guide Veristore Tools Profiling (Operator) English.pdf"
	default:
		return echo.NewHTTPError(http.StatusNotFound, "no user guide available for your role")
	}

	filePath := filepath.Join(h.assetsDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return echo.NewHTTPError(http.StatusNotFound, "user guide file not found")
	}
	return c.Attachment(filePath, filename)
}

// VersionPage renders the About > Versi page.
func (h *Handler) VersionPage(c echo.Context) error {
	page := h.pageData(c, "Versi")
	return shared.Render(c, http.StatusOK, adminTmpl.VersionPage(page))
}

// ---------------------------------------------------------------------------
// Backup
// ---------------------------------------------------------------------------

// BackupIndex displays the backup/export information page.
func (h *Handler) BackupIndex(c echo.Context) error {
	page := h.pageData(c, "Backup / Export")
	return shared.Render(c, http.StatusOK, adminTmpl.BackupPage(page))
}

// BackupDownload generates and serves an activity log backup as an Excel file.
func (h *Handler) BackupDownload(c echo.Context) error {
	// TODO: Generate an XLSX file from activity logs using excelize and serve it.
	return echo.NewHTTPError(http.StatusNotImplemented, "activity log backup download not yet implemented")
}
