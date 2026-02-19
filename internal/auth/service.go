package auth

import (
	"fmt"

	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/user"
)

// HashPasswordSHA256 is a convenience alias for shared.HashPasswordSHA256.
// Kept for backward compatibility with callers that import auth.HashPasswordSHA256.
func HashPasswordSHA256(password, salt string) string {
	return shared.HashPasswordSHA256(password, salt)
}

// VerifyPasswordSHA256 is a convenience alias for shared.VerifyPasswordSHA256.
// Kept for backward compatibility with callers that import auth.VerifyPasswordSHA256.
func VerifyPasswordSHA256(password, hash, salt string) bool {
	return shared.VerifyPasswordSHA256(password, hash, salt)
}

// Service provides authentication functionality.
type Service struct {
	userRepo *user.Repository
	salt     string
}

// NewService creates a new authentication service.
func NewService(userRepo *user.Repository, salt string) *Service {
	return &Service{
		userRepo: userRepo,
		salt:     salt,
	}
}

// Authenticate looks up a user by username and verifies their password.
// Returns the user on success or an error on failure.
func (s *Service) Authenticate(username, password string) (*user.User, error) {
	u, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	if !shared.VerifyPasswordSHA256(password, u.Password, s.salt) {
		return nil, fmt.Errorf("invalid username or password")
	}

	return u, nil
}
