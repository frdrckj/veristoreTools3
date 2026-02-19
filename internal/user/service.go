package user

import (
	"fmt"
	"time"

	"github.com/verifone/veristoretools3/internal/shared"
)

// Service provides business logic for user management operations.
type Service struct {
	repo *Repository
	salt string
}

// NewService creates a new user service.
func NewService(repo *Repository, salt string) *Service {
	return &Service{
		repo: repo,
		salt: salt,
	}
}

// CreateUser creates a new user record with a hashed password and the given
// fields. It validates that the username does not already exist.
func (s *Service) CreateUser(fullname, username, password, privileges, email, createdBy string) error {
	// Check for duplicate username.
	existing, _ := s.repo.FindByUsername(username)
	if existing != nil {
		return fmt.Errorf("username %q already exists", username)
	}

	hashed := shared.HashPasswordSHA256(password, s.salt)
	now := time.Now()
	status := 10 // Active (matches mdm/admin UserStatus::ACTIVE)

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	u := &User{
		UserFullname: fullname,
		UserName:     username,
		Password:     hashed,
		UserPrivileges: privileges,
		CreatedDtm:   now,
		CreatedBy:    createdBy,
		Email:        emailPtr,
		Status:       &status,
		CreatedAt:    intPtr(int(now.Unix())),
		UpdatedAt:    intPtr(int(now.Unix())),
	}

	return s.repo.Create(u)
}

// ChangePassword verifies the old password, then updates to the new password.
func (s *Service) ChangePassword(userID int, oldPassword, newPassword string) error {
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !shared.VerifyPasswordSHA256(oldPassword, u.Password, s.salt) {
		return fmt.Errorf("old password is incorrect")
	}

	u.Password = shared.HashPasswordSHA256(newPassword, s.salt)
	now := time.Now()
	u.UserLastChangePassword = &now
	u.UpdatedAt = intPtr(int(now.Unix()))

	return s.repo.Update(u)
}

// ToggleActivation toggles the user's status between active (10) and inactive (0).
// This mirrors v2's UserStatus::ACTIVE (10) and UserStatus::INACTIVE (0).
func (s *Service) ToggleActivation(userID int) error {
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	newStatus := 10 // Default to active
	if u.Status != nil && *u.Status == 10 {
		newStatus = 0 // Deactivate
	}

	return s.repo.UpdateStatus(userID, newStatus)
}

// GetAppTypeLabel returns the application type label for a given username.
// Returns "(Verifikasi CSI)" for ADMIN/OPERATOR, "(Profiling)" for TMS roles,
// or an empty string for unknown users/roles.
func (s *Service) GetAppTypeLabel(username string) string {
	u, err := s.repo.FindByUsername(username)
	if err != nil {
		return ""
	}

	switch u.UserPrivileges {
	case "ADMIN", "OPERATOR":
		return "(Verifikasi CSI)"
	case "TMS ADMIN", "TMS SUPERVISOR", "TMS OPERATOR":
		return "(Profiling)"
	default:
		return ""
	}
}

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int {
	return &v
}
