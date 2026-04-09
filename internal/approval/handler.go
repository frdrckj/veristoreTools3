package approval

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/queue"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/tms"
	"github.com/verifone/veristoretools3/templates/approval"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
)

// Handler handles HTTP requests for the CSI approval page.
type Handler struct {
	repo          *Repository
	tmsService    *tms.Service
	importHandler *queue.ImportTerminalHandler
	store         sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new approval handler.
func NewHandler(repo *Repository, tmsService *tms.Service, importHandler *queue.ImportTerminalHandler, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		repo:          repo,
		tmsService:    tmsService,
		importHandler: importHandler,
		store:         store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// requireTmsSession checks if the user has an active TMS session.
func (h *Handler) requireTmsSession(c echo.Context) error {
	currentUser := mw.GetCurrentUserName(c)
	if h.tmsService.GetUserSession(currentUser) == "" {
		redirect := c.Request().URL.String()
		return c.Redirect(http.StatusFound, "/veristore/login?redirect="+url.QueryEscape(redirect))
	}
	return nil
}

func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:          title,
		AppName:        h.appName,
		AppVersion:     h.appVersion,
		AppIcon:        "favicon.png",
		AppLogo:        "verifone_logo.png",
		UserName:       mw.GetCurrentUserName(c),
		UserFullname:   mw.GetCurrentUserFullname(c),
		UserPrivileges: mw.GetCurrentUserPrivileges(c),
		CopyrightTitle: "Verifone",
		CopyrightURL:   "https://www.verifone.com",
		Flashes:        flashes,
	}
}

// Index shows the list of CSI requests with pagination.
func (h *Handler) Index(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}
	page := h.pageData(c, "Persetujuan CSI")

	perPage := 10
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	requests, total, err := h.repo.FindPaginated(pageNum, perPage)
	if err != nil {
		page.Flashes = map[string][]string{shared.FlashError: {fmt.Sprintf("Failed to load requests: %v", err)}}
		return shared.Render(c, http.StatusOK, approval.ApprovalPage(page, nil, false, components.PaginationData{}))
	}

	var views []approval.RequestView
	for _, r := range requests {
		appDisplay := r.AppName
		if appDisplay == "" {
			appDisplay = r.App
		}
		views = append(views, approval.RequestView{
			ID:         r.ReqID,
			DeviceID:   r.DeviceID,
			Vendor:     r.Vendor,
			Model:      r.Model,
			MerchantID: r.MerchantID,
			SN:         r.SN,
			App:        appDisplay,
			TemplateSN: r.TemplateSN,
			Source:     r.Source,
			Status:     r.Status,
			CreatedBy:  r.CreatedBy,
			CreatedDt:  r.CreatedDt.Format("2006-01-02 15:04:05"),
		})
	}

	priv := mw.GetCurrentUserPrivileges(c)
	canApprove := priv == "ADMIN" || priv == "TMS ADMIN" || priv == "TMS SUPERVISOR"

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	pagination := components.PaginationData{
		CurrentPage: pageNum,
		TotalPages:  totalPages,
		Total:       total,
		PerPage:     perPage,
		BaseURL:     "/approval/index",
	}

	return shared.Render(c, http.StatusOK, approval.ApprovalPage(page, views, canApprove, pagination))
}

// View shows the detail of a CSI request.
func (h *Handler) View(c echo.Context) error {
	page := h.pageData(c, "Detail Permintaan CSI")

	id, _ := strconv.Atoi(c.QueryParam("id"))
	if id == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid request ID")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	req, err := h.repo.FindByID(id)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Request not found")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	appDisplay := req.AppName
	if appDisplay == "" {
		appDisplay = req.App
	}
	view := approval.RequestView{
		ID:         req.ReqID,
		DeviceID:   req.DeviceID,
		Vendor:     req.Vendor,
		Model:      req.Model,
		MerchantID: req.MerchantID,
		GroupIDs:   req.GroupIDs,
		SN:         req.SN,
		App:        appDisplay,
		MoveConf:   req.MoveConf,
		TemplateSN: req.TemplateSN,
		Source:     req.Source,
		Status:     req.Status,
		CreatedBy:  req.CreatedBy,
		CreatedDt:  req.CreatedDt.Format("2006-01-02 15:04:05"),
	}

	priv := mw.GetCurrentUserPrivileges(c)
	canApprove := priv == "ADMIN" || priv == "TMS ADMIN" || priv == "TMS SUPERVISOR"

	return shared.Render(c, http.StatusOK, approval.ApprovalDetailPage(page, view, canApprove))
}

// Approve processes a CSI request — creates the terminal in TMS.
func (h *Handler) Approve(c echo.Context) error {
	id, _ := strconv.Atoi(c.QueryParam("id"))
	if id == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid request ID")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	req, err := h.repo.FindByID(id)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Request not found")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	if req.Status != "PENDING" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Request already processed")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	if req.Source == "import" && req.TemplateSN != "" && h.importHandler != nil {
		// Full import pipeline: copy + parameter update from Excel row data.
		var groupIDs []string
		if req.GroupIDs != "" {
			groupIDs = strings.Split(req.GroupIDs, ",")
		}
		errMsg := h.importHandler.ImportSingleFromApproval(req.DeviceID, req.TemplateSN, req.MerchantID, groupIDs, req.RowData)
		if errMsg != "" {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: %s", req.DeviceID, errMsg))
			return c.Redirect(http.StatusFound, "/approval/index")
		}
	} else if req.Source == "copy" && req.TemplateSN != "" {
		// Copy: just copy terminal from template (no parameter changes).
		resp, err := h.tmsService.CopyTerminal(req.TemplateSN, req.DeviceID)
		if err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: Copy dari %s gagal - %v", req.DeviceID, req.TemplateSN, err))
			return c.Redirect(http.StatusFound, "/approval/index")
		}
		if resp.ResultCode != 0 {
			errMsg := resp.Desc
			if strings.Contains(strings.ToLower(errMsg), "duplicate") {
				errMsg = "CSI " + req.DeviceID + " sudah ada di TMS (duplikat)"
			}
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: %s", req.DeviceID, errMsg))
			return c.Redirect(http.StatusFound, "/approval/index")
		}
	} else {
		var groupIDs []string
		if req.GroupIDs != "" {
			groupIDs = strings.Split(req.GroupIDs, ",")
		}

		addReq := tms.AddTerminalRequest{
			DeviceID:   req.DeviceID,
			Vendor:     req.Vendor,
			Model:      req.Model,
			MerchantID: req.MerchantID,
			GroupIDs:   groupIDs,
			SN:         req.SN,
			MoveConf:   req.MoveConf,
		}

		resp, err := h.tmsService.AddTerminal(addReq)
		if err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: %v", req.DeviceID, err))
			return c.Redirect(http.StatusFound, "/approval/index")
		}
		if resp.ResultCode != 0 {
			errMsg := resp.Desc
			if strings.Contains(strings.ToLower(errMsg), "duplicate") {
				errMsg = "CSI " + req.DeviceID + " sudah ada di TMS (duplikat)"
			}
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: %s", req.DeviceID, errMsg))
			return c.Redirect(http.StatusFound, "/approval/index")
		}

		if req.App != "" {
			paramResp, paramErr := h.tmsService.AddParameter(req.DeviceID, req.App)
			if paramErr != nil {
				h.tmsService.DeleteTerminals([]string{req.DeviceID})
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: Assign app gagal - %v", req.DeviceID, paramErr))
				return c.Redirect(http.StatusFound, "/approval/index")
			}
			if paramResp.ResultCode != 0 {
				h.tmsService.DeleteTerminals([]string{req.DeviceID})
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Gagal approve CSI %s: Assign app gagal - %s", req.DeviceID, paramResp.Desc))
				return c.Redirect(http.StatusFound, "/approval/index")
			}
		}
	}

	// Update request status.
	now := time.Now()
	approver := mw.GetCurrentUserName(c)
	req.Status = "APPROVED"
	req.ApprovedBy = &approver
	req.ApprovedDt = &now
	h.repo.UpdateStatus(req)

	mw.LogActivityFromContext(c, mw.LogVeristoreApproveCSI, "Approve CSI request: "+req.DeviceID)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "CSI "+req.DeviceID+" berhasil di-approve dan dibuat di TMS!")
	return c.Redirect(http.StatusFound, "/approval/index")
}

// Reject deletes a CSI request.
func (h *Handler) Reject(c echo.Context) error {
	id, _ := strconv.Atoi(c.QueryParam("id"))
	if id == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid request ID")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	req, err := h.repo.FindByID(id)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Request not found")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	now := time.Now()
	rejector := mw.GetCurrentUserName(c)
	req.Status = "REJECTED"
	req.ApprovedBy = &rejector
	req.ApprovedDt = &now
	h.repo.UpdateStatus(req)

	mw.LogActivityFromContext(c, mw.LogVeristoreRejectCSI, "Reject CSI request: "+req.DeviceID)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Permintaan CSI "+req.DeviceID+" ditolak.")
	return c.Redirect(http.StatusFound, "/approval/index")
}

// BulkApprove approves multiple selected CSI requests.
func (h *Handler) BulkApprove(c echo.Context) error {
	idsStr := c.FormValue("selectedIds")
	if idsStr == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Tidak ada permintaan yang dipilih")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	var ids []int
	for _, s := range strings.Split(idsStr, ",") {
		s = strings.TrimSpace(s)
		if id, err := strconv.Atoi(s); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	requests, _ := h.repo.FindByIDs(ids)
	var approved int
	var failedDetails []string
	now := time.Now()
	approver := mw.GetCurrentUserName(c)

	for _, req := range requests {
		if req.Status != "PENDING" {
			continue
		}

		var success bool
		var failReason string

		if req.Source == "import" && req.TemplateSN != "" && h.importHandler != nil {
			var groupIDs []string
			if req.GroupIDs != "" {
				groupIDs = strings.Split(req.GroupIDs, ",")
			}
			errMsg := h.importHandler.ImportSingleFromApproval(req.DeviceID, req.TemplateSN, req.MerchantID, groupIDs, req.RowData)
			if errMsg != "" {
				failReason = fmt.Sprintf("%s: %s", req.DeviceID, errMsg)
			} else {
				success = true
			}
		} else if req.Source == "copy" && req.TemplateSN != "" {
			resp, err := h.tmsService.CopyTerminal(req.TemplateSN, req.DeviceID)
			if err != nil {
				failReason = fmt.Sprintf("%s: Copy gagal - %v", req.DeviceID, err)
			} else if resp.ResultCode != 0 {
				errMsg := resp.Desc
				if strings.Contains(strings.ToLower(errMsg), "duplicate") {
					errMsg = "sudah ada di TMS (duplikat)"
				}
				failReason = fmt.Sprintf("%s: %s", req.DeviceID, errMsg)
			} else {
				success = true
			}
		} else {
			var groupIDs []string
			if req.GroupIDs != "" {
				groupIDs = strings.Split(req.GroupIDs, ",")
			}
			addReq := tms.AddTerminalRequest{
				DeviceID: req.DeviceID, Vendor: req.Vendor, Model: req.Model,
				MerchantID: req.MerchantID, GroupIDs: groupIDs, SN: req.SN, MoveConf: req.MoveConf,
			}
			resp, err := h.tmsService.AddTerminal(addReq)
			if err != nil {
				failReason = fmt.Sprintf("%s: %v", req.DeviceID, err)
			} else if resp.ResultCode != 0 {
				errMsg := resp.Desc
				if strings.Contains(strings.ToLower(errMsg), "duplicate") {
					errMsg = "sudah ada di TMS (duplikat)"
				}
				failReason = fmt.Sprintf("%s: %s", req.DeviceID, errMsg)
			} else {
				if req.App != "" {
					paramResp, paramErr := h.tmsService.AddParameter(req.DeviceID, req.App)
					if paramErr != nil || paramResp.ResultCode != 0 {
						h.tmsService.DeleteTerminals([]string{req.DeviceID})
						failReason = fmt.Sprintf("%s: Assign app gagal", req.DeviceID)
					} else {
						success = true
					}
				} else {
					success = true
				}
			}
		}

		if success {
			req.Status = "APPROVED"
			req.ApprovedBy = &approver
			req.ApprovedDt = &now
			h.repo.UpdateStatus(&req)
			approved++
		} else {
			failedDetails = append(failedDetails, failReason)
		}
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreApproveCSI, fmt.Sprintf("Bulk approve %d CSI requests", approved))
	msg := fmt.Sprintf("%d permintaan berhasil di-approve.", approved)
	if len(failedDetails) > 0 {
		msg += fmt.Sprintf(" %d gagal: %s", len(failedDetails), strings.Join(failedDetails, "; "))
	}
	if len(failedDetails) > 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashWarning, msg)
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, msg)
	}
	return c.Redirect(http.StatusFound, "/approval/index")
}

// BulkReject rejects multiple selected CSI requests.
func (h *Handler) BulkReject(c echo.Context) error {
	idsStr := c.FormValue("selectedIds")
	if idsStr == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Tidak ada permintaan yang dipilih")
		return c.Redirect(http.StatusFound, "/approval/index")
	}

	var ids []int
	for _, s := range strings.Split(idsStr, ",") {
		s = strings.TrimSpace(s)
		if id, err := strconv.Atoi(s); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	requests, _ := h.repo.FindByIDs(ids)
	var rejected int
	now := time.Now()
	rejector := mw.GetCurrentUserName(c)

	for _, req := range requests {
		if req.Status != "PENDING" {
			continue
		}
		req.Status = "REJECTED"
		req.ApprovedBy = &rejector
		req.ApprovedDt = &now
		h.repo.UpdateStatus(&req)
		rejected++
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreRejectCSI, fmt.Sprintf("Bulk reject %d CSI requests", rejected))
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, fmt.Sprintf("%d permintaan ditolak.", rejected))
	return c.Redirect(http.StatusFound, "/approval/index")
}
