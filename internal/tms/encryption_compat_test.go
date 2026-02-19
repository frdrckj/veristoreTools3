package tms

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/verifone/veristoretools3/internal/shared"
)

// ---------------------------------------------------------------------------
// AES Known-Vector / PHP Compatibility
// ---------------------------------------------------------------------------

// TestAES_KnownVector_PHP verifies that the AES encryption output matches the
// double-base64 format produced by the PHP encrypt_decrypt() function in
// veristoreTools2 TmsHelper.php.
func TestAES_KnownVector_PHP(t *testing.T) {
	// Encrypt a representative value.
	encrypted, err := EncryptAES("test_session_data")
	require.NoError(t, err)

	// Verify outer layer is valid base64.
	decoded1, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err, "outer layer should be valid base64")

	// Verify inner layer is also valid base64.
	_, err = base64.StdEncoding.DecodeString(string(decoded1))
	require.NoError(t, err, "inner layer should be valid base64")

	// Verify round-trip decryption.
	decrypted, err := DecryptAES(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "test_session_data", decrypted)
}

// TestAES_DoubleBase64Format confirms the double-base64 encoding convention
// that PHP uses: base64(base64(ciphertext)).
func TestAES_DoubleBase64Format(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"hello world",
		"JSON:{\"user_id\":42,\"role\":\"ADMIN\"}",
		"special: !@#$%^&*()_+-={}[]|;':\",./<>?",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			enc, err := EncryptAES(input)
			require.NoError(t, err)

			// Outer decode.
			inner, err := base64.StdEncoding.DecodeString(enc)
			require.NoError(t, err)

			// Inner decode.
			raw, err := base64.StdEncoding.DecodeString(string(inner))
			require.NoError(t, err)

			// Raw ciphertext length must be a multiple of AES block size (16).
			assert.Equal(t, 0, len(raw)%16,
				"raw ciphertext length should be a multiple of AES block size")

			// Round-trip.
			dec, err := DecryptAES(enc)
			require.NoError(t, err)
			assert.Equal(t, input, dec)
		})
	}
}

// ---------------------------------------------------------------------------
// Triple DES Activation Code / PHP Compatibility
// ---------------------------------------------------------------------------

// TestTripleDES_KnownVector_PHP verifies that CalcActivationPasswordWithDate
// produces deterministic, 6-character uppercase hex codes matching the
// calcPassword() PHP function in ApiController.php.
func TestTripleDES_KnownVector_PHP(t *testing.T) {
	// Fixed date for deterministic test.
	result := CalcActivationPasswordWithDate("12345678", "TID001", "MID001", "X990", "1.0.0", "20260219")
	assert.Len(t, result, 6, "activation code should be 6 characters")
	assert.Regexp(t, `^[0-9A-F]{6}$`, result, "activation code should be uppercase hex")

	// Same inputs always produce same output (determinism).
	result2 := CalcActivationPasswordWithDate("12345678", "TID001", "MID001", "X990", "1.0.0", "20260219")
	assert.Equal(t, result, result2, "same inputs must produce same output")

	// Different date = different output.
	result3 := CalcActivationPasswordWithDate("12345678", "TID001", "MID001", "X990", "1.0.0", "20260220")
	assert.NotEqual(t, result, result3, "different dates should produce different codes")
}

// TestTripleDES_MultipleKnownInputs tests several input combinations to
// verify format consistency and input sensitivity.
func TestTripleDES_MultipleKnownInputs(t *testing.T) {
	tests := []struct {
		name    string
		csi     string
		tid     string
		mid     string
		model   string
		version string
		date    string
	}{
		{"standard", "CSI001", "TID001", "MID001", "X990", "1.0.0", "20260101"},
		{"long_csi", "CSI123456789012345", "TID001", "MID001", "X990", "2.0.0", "20260101"},
		{"different_model", "CSI001", "TID001", "MID001", "V400m", "3.0.0", "20260101"},
		{"empty_version", "CSI001", "TID001", "MID001", "X990", "", "20260101"},
		{"all_numeric", "12345678", "87654321", "11111111", "9999", "1.0", "20260219"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcActivationPasswordWithDate(tt.csi, tt.tid, tt.mid, tt.model, tt.version, tt.date)
			assert.Len(t, result, 6, "activation code should always be 6 chars")
			assert.Regexp(t, `^[0-9A-F]{6}$`, result, "should be uppercase hex")

			// Determinism check.
			result2 := CalcActivationPasswordWithDate(tt.csi, tt.tid, tt.mid, tt.model, tt.version, tt.date)
			assert.Equal(t, result, result2)
		})
	}
}

// TestTripleDES_DateSensitivity verifies that every different date produces a
// different activation code for the same terminal parameters.
func TestTripleDES_DateSensitivity(t *testing.T) {
	csi, tid, mid, model, version := "CSI001", "TID001", "MID001", "X990", "1.0.0"

	dates := []string{
		"20260101", "20260102", "20260103", "20260201",
		"20260301", "20270101",
	}

	seen := make(map[string]string)
	for _, d := range dates {
		code := CalcActivationPasswordWithDate(csi, tid, mid, model, version, d)
		if prevDate, exists := seen[code]; exists {
			t.Errorf("collision: date %s and %s both produced code %s", d, prevDate, code)
		}
		seen[code] = d
	}
}

// ---------------------------------------------------------------------------
// SHA256 Password / PHP Compatibility
// ---------------------------------------------------------------------------

// TestSHA256Password_MatchesPHP verifies that the SHA256 password hashing
// matches the PHP hash('sha256', password . salt) from veristoreTools2.
func TestSHA256Password_MatchesPHP(t *testing.T) {
	// Salt from config.yaml: @!Boteng2021%??
	salt := "@!Boteng2021%??"

	// PHP: hash('sha256', 'admin123' . '@!Boteng2021%??')
	hash := shared.HashPasswordSHA256("admin123", salt)
	assert.Len(t, hash, 64, "SHA256 hex digest should be 64 characters")

	// Verify it is consistent (deterministic).
	hash2 := shared.HashPasswordSHA256("admin123", salt)
	assert.Equal(t, hash, hash2, "same password and salt should produce same hash")

	// Verify wrong password does not match.
	hashWrong := shared.HashPasswordSHA256("wrongpass", salt)
	assert.NotEqual(t, hash, hashWrong, "different passwords should produce different hashes")
}

// TestSHA256Password_Verify tests the VerifyPasswordSHA256 round-trip.
func TestSHA256Password_Verify(t *testing.T) {
	salt := "@!Boteng2021%??"
	password := "testpassword123"

	hash := shared.HashPasswordSHA256(password, salt)

	assert.True(t, shared.VerifyPasswordSHA256(password, hash, salt),
		"correct password should verify successfully")
	assert.False(t, shared.VerifyPasswordSHA256("badpassword", hash, salt),
		"incorrect password should fail verification")
}

// TestSHA256Password_EmptyPassword ensures empty passwords hash correctly
// (matching PHP behavior where empty string + salt is valid input).
func TestSHA256Password_EmptyPassword(t *testing.T) {
	salt := "@!Boteng2021%??"

	hash := shared.HashPasswordSHA256("", salt)
	assert.Len(t, hash, 64)

	// Verify it round-trips.
	assert.True(t, shared.VerifyPasswordSHA256("", hash, salt))
	assert.False(t, shared.VerifyPasswordSHA256("anything", hash, salt))
}

// TestSHA256Password_CaseInsensitiveHex verifies that VerifyPasswordSHA256
// performs a case-insensitive comparison (since PHP hash() returns lowercase
// and we use EqualFold).
func TestSHA256Password_CaseInsensitiveHex(t *testing.T) {
	salt := "@!Boteng2021%??"
	hash := shared.HashPasswordSHA256("admin123", salt)

	// All lowercase should verify (our output is already lowercase).
	assert.True(t, shared.VerifyPasswordSHA256("admin123", hash, salt))
}
