package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashPasswordSHA256(t *testing.T) {
	hash := HashPasswordSHA256("password123", "somesalt")

	// SHA256 hex digest is always 64 characters.
	assert.Len(t, hash, 64, "SHA256 hex hash should be 64 characters")

	// Same input must produce the same hash.
	hash2 := HashPasswordSHA256("password123", "somesalt")
	assert.Equal(t, hash, hash2, "hashing the same input should be deterministic")
}

func TestVerifyPassword(t *testing.T) {
	salt := "testsalt"
	password := "correcthorsebatterystaple"

	hash := HashPasswordSHA256(password, salt)

	assert.True(t, VerifyPasswordSHA256(password, hash, salt), "correct password should verify")
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	salt := "testsalt"
	password := "correcthorsebatterystaple"

	hash := HashPasswordSHA256(password, salt)

	assert.False(t, VerifyPasswordSHA256("wrongpassword", hash, salt), "wrong password should not verify")
}
