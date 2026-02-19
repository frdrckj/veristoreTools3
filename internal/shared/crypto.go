package shared

import (
	"crypto/sha256"
	"fmt"
	"strings"
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
