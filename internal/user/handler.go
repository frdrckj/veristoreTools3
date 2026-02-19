package user

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	userTmpl "github.com/verifone/veristoretools3/templates/user"
)

// Handler holds dependencies for user management HTTP handlers.
type Handler struct {
	repo             *Repository
	service          *Service
	store            sessions.Store
	sessionName      string
	appName          string
	appVersion       string
	appIcon          string
	appLogo          string
	appClientLogo    string
	appVeristoreLogo string
	appTmsURL        string
	appBgColor       string
	copyrightTitle   string
	copyrightURL     string
}

// NewHandler creates a new user handler.
func NewHandler(
	repo *Repository,
	service *Service,
	store sessions.Store,
	sessionName string,
	appName string,
	appVersion string,
) *Handler {
	return &Handler{
		repo:             repo,
		service:          service,
		store:            store,
		sessionName:      sessionName,
		appName:          appName,
		appVersion:       appVersion,
		appIcon:          "favicon.png",
		appLogo:          "verifone_logo.png",
		appClientLogo:    "",
		appVeristoreLogo: "veristore_logo.png",
		appTmsURL:        "",
		appBgColor:       "",
		copyrightTitle:   "Verifone",
		copyrightURL:     "https://www.verifone.com",
	}
}

// SetBranding configures optional branding fields on the handler.
func (h *Handler) SetBranding(clientLogo, veristoreLogo, tmsURL, bgColor, copyrightTitle, copyrightURL string) {
	if clientLogo != "" {
		h.appClientLogo = clientLogo
	}
	if veristoreLogo != "" {
		h.appVeristoreLogo = veristoreLogo
	}
	if tmsURL != "" {
		h.appTmsURL = tmsURL
	}
	if bgColor != "" {
		h.appBgColor = bgColor
	}
	if copyrightTitle != "" {
		h.copyrightTitle = copyrightTitle
	}
	if copyrightURL != "" {
		h.copyrightURL = copyrightURL
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:            title,
		AppName:          h.appName,
		AppVersion:       h.appVersion,
		AppIcon:          h.appIcon,
		AppLogo:          h.appLogo,
		AppClientLogo:    h.appClientLogo,
		AppVeristoreLogo: h.appVeristoreLogo,
		AppTmsURL:        h.appTmsURL,
		AppBgColor:       h.appBgColor,
		UserName:         mw.GetCurrentUserName(c),
		UserFullname:     mw.GetCurrentUserFullname(c),
		UserPrivileges:   mw.GetCurrentUserPrivileges(c),
		CopyrightTitle:   h.copyrightTitle,
		CopyrightURL:     h.copyrightURL,
		Flashes:          flashes,
	}
}

// toUserData converts a User model to a UserData view struct for templates.
func toUserData(u User) userTmpl.UserData {
	status := 0
	if u.Status != nil {
		status = *u.Status
	}
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	lastChangePwd := ""
	if u.UserLastChangePassword != nil {
		lastChangePwd = u.UserLastChangePassword.Format("2006-01-02 15:04:05")
	}
	return userTmpl.UserData{
		UserID:         u.UserID,
		UserFullname:   u.UserFullname,
		UserName:       u.UserName,
		UserPrivileges: u.UserPrivileges,
		Email:          email,
		Status:         status,
		CreatedBy:      u.CreatedBy,
		CreatedDtm:     u.CreatedDtm.Format("2006-01-02 15:04:05"),
		LastChangePwd:  lastChangePwd,
	}
}

// toUserDataSlice converts a slice of User models to UserData view structs.
func toUserDataSlice(users []User) []userTmpl.UserData {
	result := make([]userTmpl.UserData, len(users))
	for i, u := range users {
		result[i] = toUserData(u)
	}
	return result
}

// privilegeOptions returns the list of available user privilege values.
func privilegeOptions() []string {
	return []string{
		"ADMIN",
		"OPERATOR",
		"TMS ADMIN",
		"TMS SUPERVISOR",
		"TMS OPERATOR",
	}
}

// Index lists users with search and pagination. Supports HTMX partial updates.
func (h *Handler) Index(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	users, pagination, err := h.repo.Search(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load users")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/user/index",
		HTMXTarget:  "user-table-container",
	}

	page := h.pageData(c, "User Management")
	userData := toUserDataSlice(users)

	// For HTMX requests, return only the table partial.
	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, userTmpl.UserTablePartial(userData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, userTmpl.IndexPage(page, userData, paginationData, query))
}

// View displays user details by ID.
func (h *Handler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	u, err := h.repo.FindByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	page := h.pageData(c, "User Detail")
	return shared.Render(c, http.StatusOK, userTmpl.ViewPage(page, toUserData(*u)))
}

// Create handles both GET (show form) and POST (process form) for creating a user.
func (h *Handler) Create(c echo.Context) error {
	page := h.pageData(c, "Create User")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, userTmpl.CreatePage(page, privilegeOptions(), "", nil))
	}

	// Process POST.
	fullname := c.FormValue("fullname")
	username := c.FormValue("username")
	password := c.FormValue("password")
	privileges := c.FormValue("privileges")
	email := c.FormValue("email")
	createdBy := mw.GetCurrentUserName(c)

	// Basic validation.
	var errors []string
	if fullname == "" {
		errors = append(errors, "Full name is required")
	}
	if username == "" {
		errors = append(errors, "Username is required")
	}
	if password == "" {
		errors = append(errors, "Password is required")
	}
	if privileges == "" {
		errors = append(errors, "Privileges is required")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, userTmpl.CreatePage(page, privilegeOptions(), "", errors))
	}

	if err := h.service.CreateUser(fullname, username, password, privileges, email, createdBy); err != nil {
		return shared.Render(c, http.StatusOK, userTmpl.CreatePage(page, privilegeOptions(), "", []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "User created successfully")
	return c.Redirect(http.StatusFound, "/user/index")
}

// Delete removes a user by ID and redirects to the user list.
func (h *Handler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.FormValue("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.repo.Delete(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to delete user")
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "User deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/user/index")
}

// Activate toggles a user's active/inactive status.
func (h *Handler) Activate(c echo.Context) error {
	id, err := strconv.Atoi(c.FormValue("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.service.ToggleActivation(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update user status: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "User status updated successfully")
	}

	return c.Redirect(http.StatusFound, "/user/index")
}

// ChangePassword handles both GET (show form) and POST (process change) for
// changing the current user's password.
func (h *Handler) ChangePassword(c echo.Context) error {
	page := h.pageData(c, "Change Password")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, userTmpl.ChangePasswordPage(page, nil))
	}

	// Process POST.
	oldPassword := c.FormValue("old_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("confirm_password")

	// Validation.
	var errors []string
	if oldPassword == "" {
		errors = append(errors, "Current password is required")
	}
	if newPassword == "" {
		errors = append(errors, "New password is required")
	}
	if confirmPassword == "" {
		errors = append(errors, "Confirm password is required")
	}
	if newPassword != confirmPassword {
		errors = append(errors, "New password and confirmation do not match")
	}
	if len(newPassword) < 6 {
		errors = append(errors, "New password must be at least 6 characters")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, userTmpl.ChangePasswordPage(page, errors))
	}

	userID := mw.GetCurrentUserID(c)
	if err := h.service.ChangePassword(userID, oldPassword, newPassword); err != nil {
		return shared.Render(c, http.StatusOK, userTmpl.ChangePasswordPage(page, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Password changed successfully")
	return c.Redirect(http.StatusFound, "/")
}

// GetAppType is an AJAX endpoint that returns the application type label
// for a given username. Used by the login form to display "(Verifikasi CSI)"
// or "(Profiling)" next to the username.
func (h *Handler) GetAppType(c echo.Context) error {
	username := c.QueryParam("username")
	if username == "" {
		username = c.Param("username")
	}
	label := h.service.GetAppTypeLabel(username)
	return c.String(http.StatusOK, label)
}
