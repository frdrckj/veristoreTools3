package activation

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/tms"
	"gorm.io/gorm"
)

// ActivationCodeRequest is the JSON request body for the activation code API.
type ActivationCodeRequest struct {
	CSI     string `json:"csi"`
	TID     string `json:"tid"`
	MID     string `json:"mid"`
	Model   string `json:"model"`
	Version string `json:"version"`
}

// ActivationCodeResponse is the JSON response body for the activation code API.
type ActivationCodeResponse struct {
	ActivationCode string `json:"activation_code"`
}

// APIHandler handles REST API endpoints for activation.
type APIHandler struct {
	repo *Repository
	db   *gorm.DB
}

// NewAPIHandler creates a new activation API handler.
func NewAPIHandler(repo *Repository, db *gorm.DB) *APIHandler {
	return &APIHandler{
		repo: repo,
		db:   db,
	}
}

// ActivationCode handles POST /feature/api/activation-code.
// It computes and returns the activation code using Triple DES ECB.
// This endpoint is protected by BasicAuth middleware.
func (h *APIHandler) ActivationCode(c echo.Context) error {
	var req ActivationCodeRequest
	if err := c.Bind(&req); err != nil {
		return shared.APIError(c, http.StatusBadRequest, "invalid request body")
	}

	if req.CSI == "" || req.TID == "" || req.MID == "" {
		return shared.APIError(c, http.StatusBadRequest, "csi, tid, and mid are required")
	}

	// Compute the activation password.
	code := tms.CalcActivationPassword(req.CSI, req.TID, req.MID, req.Model, req.Version)

	// Log the activation to the database.
	act := &AppActivation{
		AppActCSI:      req.CSI,
		AppActTID:      req.TID,
		AppActMID:      req.MID,
		AppActModel:    req.Model,
		AppActVersion:  req.Version,
		AppActEngineer: "API",
		CreatedBy:      "api",
		CreatedDt:      time.Now(),
	}
	// Best-effort: log the activation but don't fail the response.
	_ = h.repo.CreateActivation(act)

	return shared.APISuccess(c, ActivationCodeResponse{
		ActivationCode: code,
	})
}
