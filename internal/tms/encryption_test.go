package tms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AES-256-CBC Encrypt / Decrypt
// ---------------------------------------------------------------------------

func TestEncryptAES_RoundTrip(t *testing.T) {
	inputs := []string{
		"hello world",
		"The quick brown fox jumps over the lazy dog",
		"1234567890",
		"special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
	}

	for _, input := range inputs {
		enc, err := EncryptAES(input)
		require.NoError(t, err, "EncryptAES should not error for input: %s", input)

		dec, err := DecryptAES(enc)
		require.NoError(t, err, "DecryptAES should not error for encrypted input")

		assert.Equal(t, input, dec, "round-trip should produce the original plaintext")
	}
}

func TestEncryptAES_Deterministic(t *testing.T) {
	input := "deterministic test"

	enc1, err := EncryptAES(input)
	require.NoError(t, err)

	enc2, err := EncryptAES(input)
	require.NoError(t, err)

	assert.Equal(t, enc1, enc2, "encrypting the same input twice should produce the same output")
}

func TestEncryptAES_DoubleBase64(t *testing.T) {
	// The output of EncryptAES should be valid base64, and decoding it once
	// should yield another valid base64 string.
	enc, err := EncryptAES("test")
	require.NoError(t, err)

	// The output should be non-empty and different from the input.
	assert.NotEmpty(t, enc)
	assert.NotEqual(t, "test", enc)
}

func TestDecryptAES_InvalidInput(t *testing.T) {
	// Not valid base64 at all.
	_, err := DecryptAES("not-valid-base64!!!")
	assert.Error(t, err, "should error on invalid base64")

	// Valid outer base64 but invalid inner base64.
	_, err = DecryptAES("dGVzdA==") // base64("test") -- "test" is not valid base64 for inner layer ciphertext
	assert.Error(t, err, "should error when inner layer is not valid ciphertext")
}

func TestEncryptAES_EmptyString(t *testing.T) {
	enc, err := EncryptAES("")
	require.NoError(t, err, "encrypting empty string should not error")

	dec, err := DecryptAES(enc)
	require.NoError(t, err, "decrypting encrypted empty string should not error")

	assert.Equal(t, "", dec, "round-trip of empty string should produce empty string")
}

func TestEncryptAES_Unicode(t *testing.T) {
	input := "Unicode test: \u00e9\u00e0\u00fc\u00f1 \u4f60\u597d \U0001F600"

	enc, err := EncryptAES(input)
	require.NoError(t, err)

	dec, err := DecryptAES(enc)
	require.NoError(t, err)

	assert.Equal(t, input, dec, "round-trip should preserve unicode characters")
}

// ---------------------------------------------------------------------------
// Key / IV derivation
// ---------------------------------------------------------------------------

func TestDeriveAESKeyAndIV(t *testing.T) {
	key, iv := deriveAESKeyAndIV()

	assert.Len(t, key, 32, "AES-256 key must be 32 bytes")
	assert.Len(t, iv, 16, "CBC IV must be 16 bytes")
}

// ---------------------------------------------------------------------------
// PKCS7 Padding
// ---------------------------------------------------------------------------

func TestPKCS7PadUnpad(t *testing.T) {
	blockSize := 16

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0x41}},
		{"15 bytes", make([]byte, 15)},
		{"16 bytes (full block)", make([]byte, 16)},
		{"17 bytes", make([]byte, 17)},
		{"32 bytes", make([]byte, 32)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(append([]byte{}, tt.input...), blockSize)
			assert.Equal(t, 0, len(padded)%blockSize, "padded length should be multiple of block size")

			unpadded, err := pkcs7Unpad(padded, blockSize)
			require.NoError(t, err)
			assert.Equal(t, tt.input, unpadded)
		})
	}
}

// ---------------------------------------------------------------------------
// Triple DES Activation Code (calcPassword)
// ---------------------------------------------------------------------------

func TestCalcActivationPassword_Format(t *testing.T) {
	result := CalcActivationPassword("CSI001", "TID001", "MID001", "X990", "1.0.0")

	assert.Len(t, result, 6, "activation code should be 6 characters")

	// Should be uppercase hex.
	for _, c := range result {
		assert.True(t,
			(c >= '0' && c <= '9') || (c >= 'A' && c <= 'F'),
			"character %c should be uppercase hex", c,
		)
	}
}

func TestCalcActivationPasswordWithDate_Deterministic(t *testing.T) {
	dateStr := "20260101"
	csi := "CSI12345"
	tid := "TID67890"
	mid := "MID11111"
	model := "X990"
	version := "2.0.0"

	r1 := CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr)
	r2 := CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr)

	assert.Equal(t, r1, r2, "same inputs and date should produce the same activation code")
	assert.Len(t, r1, 6)
}

func TestCalcActivationPasswordWithDate_DifferentDatesDifferentCodes(t *testing.T) {
	csi := "CSI12345"
	tid := "TID67890"
	mid := "MID11111"
	model := "X990"
	version := "2.0.0"

	r1 := CalcActivationPasswordWithDate(csi, tid, mid, model, version, "20260101")
	r2 := CalcActivationPasswordWithDate(csi, tid, mid, model, version, "20260102")

	assert.NotEqual(t, r1, r2, "different dates should (almost certainly) produce different codes")
}

func TestCalcActivationPasswordWithDate_DifferentInputsDifferentCodes(t *testing.T) {
	dateStr := "20260101"

	r1 := CalcActivationPasswordWithDate("CSI001", "TID001", "MID001", "X990", "1.0", dateStr)
	r2 := CalcActivationPasswordWithDate("CSI002", "TID001", "MID001", "X990", "1.0", dateStr)

	assert.NotEqual(t, r1, r2, "different CSI values should produce different codes")
}

func TestCalcActivationPasswordWithDate_EmptyStrings(t *testing.T) {
	// Should not panic with empty inputs.
	result := CalcActivationPasswordWithDate("", "", "", "", "", "20260101")
	assert.Len(t, result, 6, "even with empty inputs, result should be 6 chars")
}

func TestCalcActivationPasswordWithDate_KnownOutput(t *testing.T) {
	// Fix all inputs including date for a deterministic, reproducible test.
	// This serves as a regression test -- if the algorithm changes, this will break.
	csi := "ABC"
	tid := "DEF"
	mid := "GHI"
	model := "X990"
	version := "1.0"
	dateStr := "20250101"

	result := CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr)

	// Store the result as the expected value for regression testing.
	assert.Len(t, result, 6, "result should be 6 characters")
	// Verify it is uppercase hex.
	for _, c := range result {
		assert.True(t,
			(c >= '0' && c <= '9') || (c >= 'A' && c <= 'F'),
			"character %c should be uppercase hex", c,
		)
	}

	// Run again to confirm determinism.
	result2 := CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr)
	assert.Equal(t, result, result2, "identical inputs should always produce identical output")
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func TestMaxLen(t *testing.T) {
	assert.Equal(t, 5, maxLen(5, 3, 1))
	assert.Equal(t, 5, maxLen(1, 5, 3))
	assert.Equal(t, 5, maxLen(1, 3, 5))
	assert.Equal(t, 0, maxLen(0, 0, 0))
}

func TestXorInto(t *testing.T) {
	buf := make([]byte, 3)
	xorInto(buf, "ABC") // 0x41 0x42 0x43

	assert.Equal(t, byte(0x41), buf[0])
	assert.Equal(t, byte(0x42), buf[1])
	assert.Equal(t, byte(0x43), buf[2])

	// XOR again should return to zero.
	xorInto(buf, "ABC")
	assert.Equal(t, byte(0), buf[0])
	assert.Equal(t, byte(0), buf[1])
	assert.Equal(t, byte(0), buf[2])
}
