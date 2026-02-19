package auth

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/verifone/veristoretools3/internal/user"
)

// HashPasswordSHA256 produces a hex-encoded SHA256 hash of the password
// concatenated with the salt. This matches the algorithm used in
// veristoreTools v2.
func HashPasswordSHA256(password, salt string) string {
	data := password + salt
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// VerifyPasswordSHA256 checks that the given password, when hashed with the
// provided salt, matches the expected hash.
func VerifyPasswordSHA256(password, hash, salt string) bool {
	computed := HashPasswordSHA256(password, salt)
	return strings.EqualFold(computed, hash)
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

	if !VerifyPasswordSHA256(password, u.Password, s.salt) {
		return nil, fmt.Errorf("invalid username or password")
	}

	return u, nil
}
