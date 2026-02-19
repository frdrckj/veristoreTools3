package tms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Default AES-256-CBC constants matching veristoreTools2 TmsHelper.php.
// Deprecated: Use Encryptor with explicit keys from config instead.
const (
	aesSecretKey = "35136HH7B63C27AA74CDCC2BBRT9"
	aesSecretIV  = "J5g275fgf5H"
)

// Encryptor holds configurable AES-256-CBC encryption keys. Use NewEncryptor
// to create an instance with keys from config rather than hard-coded defaults.
type Encryptor struct {
	secretKey string
	secretIV  string
}

// NewEncryptor creates an Encryptor with the provided secret key and IV.
func NewEncryptor(secretKey, secretIV string) *Encryptor {
	return &Encryptor{secretKey: secretKey, secretIV: secretIV}
}

// Encrypt encrypts plaintext using AES-256-CBC with double base64 encoding,
// matching the PHP encrypt_decrypt() function in TmsHelper.php.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	key, iv := deriveKeyAndIV(e.secretKey, e.secretIV)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("tms: aes new cipher: %w", err)
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)

	encrypted := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted, padded)

	inner := base64.StdEncoding.EncodeToString(encrypted)
	outer := base64.StdEncoding.EncodeToString([]byte(inner))

	return outer, nil
}

// Decrypt decrypts ciphertext that was encrypted by Encrypt (or the
// equivalent PHP encrypt_decrypt function).
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	key, iv := deriveKeyAndIV(e.secretKey, e.secretIV)

	innerB64, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("tms: outer base64 decode: %w", err)
	}

	raw, err := base64.StdEncoding.DecodeString(string(innerB64))
	if err != nil {
		return "", fmt.Errorf("tms: inner base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("tms: aes new cipher: %w", err)
	}

	if len(raw)%aes.BlockSize != 0 {
		return "", fmt.Errorf("tms: ciphertext length %d is not a multiple of block size %d", len(raw), aes.BlockSize)
	}

	decrypted := make([]byte, len(raw))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, raw)

	unpadded, err := pkcs7Unpad(decrypted, aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("tms: pkcs7 unpad: %w", err)
	}

	return string(unpadded), nil
}

// EncryptAES encrypts plaintext using AES-256-CBC with double base64 encoding,
// matching the PHP encrypt_decrypt() function in TmsHelper.php.
//
// Deprecated: Use Encryptor.Encrypt with keys from config instead.
// This function uses hard-coded default keys for backward compatibility.
func EncryptAES(plaintext string) (string, error) {
	return defaultEncryptor.Encrypt(plaintext)
}

// DecryptAES decrypts ciphertext that was encrypted by EncryptAES (or the
// equivalent PHP encrypt_decrypt function).
//
// Deprecated: Use Encryptor.Decrypt with keys from config instead.
// This function uses hard-coded default keys for backward compatibility.
func DecryptAES(ciphertext string) (string, error) {
	return defaultEncryptor.Decrypt(ciphertext)
}

// defaultEncryptor uses the hard-coded keys for backward compatibility.
var defaultEncryptor = NewEncryptor(aesSecretKey, aesSecretIV)

// deriveKeyAndIV computes the AES key and IV from the provided secrets,
// matching the PHP derivation in TmsHelper.php:
//
//	$key = hash('sha256', $secret_key);          -> 64 hex chars
//	$iv  = substr(hash('sha256', $secret_iv), 0, 16);  -> first 16 hex chars
//
// PHP openssl_encrypt for AES-256-CBC uses the first 32 bytes of the key
// parameter and expects exactly 16 bytes for the IV.
func deriveKeyAndIV(secretKey, secretIV string) (key []byte, iv []byte) {
	keyHash := sha256.Sum256([]byte(secretKey))
	keyHex := fmt.Sprintf("%x", keyHash) // 64 hex chars
	key = []byte(keyHex[:32])            // first 32 bytes (chars) for AES-256

	ivHash := sha256.Sum256([]byte(secretIV))
	ivHex := fmt.Sprintf("%x", ivHash) // 64 hex chars
	iv = []byte(ivHex[:16])            // first 16 bytes (chars) for CBC IV

	return key, iv
}

// deriveAESKeyAndIV is kept for backward compatibility.
// Deprecated: Use deriveKeyAndIV with explicit keys instead.
func deriveAESKeyAndIV() (key []byte, iv []byte) {
	return deriveKeyAndIV(aesSecretKey, aesSecretIV)
}

// pkcs7Pad appends PKCS#7 padding to data so its length is a multiple of
// blockSize.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padByte := byte(padding)
	for i := 0; i < padding; i++ {
		data = append(data, padByte)
	}
	return data
}

// pkcs7Unpad removes PKCS#7 padding from data.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	if len(data)%blockSize != 0 {
		return nil, fmt.Errorf("data length %d not multiple of block size %d", len(data), blockSize)
	}

	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, fmt.Errorf("invalid padding length %d", padLen)
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding byte at position %d", i)
		}
	}

	return data[:len(data)-padLen], nil
}

// CalcActivationPassword computes the 6-character hex activation code using
// Triple DES ECB, matching the PHP calcPassword() in ApiController.php.
// It uses the current date (format: YYYYMMDD) as the date input.
func CalcActivationPassword(csi, tid, mid, model, version string) string {
	dateStr := time.Now().Format("20060102")
	return CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr)
}

// CalcActivationPasswordWithDate computes the 6-character hex activation code
// using Triple DES ECB, matching the PHP calcPassword(). The dateStr parameter
// must be in "YYYYMMDD" format (e.g., "20260219"). This variant is exposed for
// deterministic testing.
func CalcActivationPasswordWithDate(csi, tid, mid, model, version, dateStr string) string {
	// --- LEFT password ---
	leftMax := maxLen(len(csi), len(tid), len(mid))
	left := make([]byte, leftMax)
	xorInto(left, csi)
	xorInto(left, tid)
	xorInto(left, mid)
	leftHash := sha256.Sum256(left)

	// --- RIGHT password ---
	rightMax := maxLen(len(csi), len(model), len(version))
	right := make([]byte, rightMax)
	xorInto(right, csi)
	xorInto(right, model)
	xorInto(right, version)
	rightHash := sha256.Sum256(right)

	// --- Construct 24-byte Triple DES key ---
	key := make([]byte, 24)
	copy(key[0:12], leftHash[0:12])
	copy(key[12:24], rightHash[0:12])

	// --- Data to encrypt: SHA-256 of date string (raw 32 bytes) ---
	dateHash := sha256.Sum256([]byte(dateStr))
	data := dateHash[:]

	// --- Triple DES ECB encrypt ---
	encrypted := tripleDESECBEncrypt(key, data)

	// --- Return first 6 chars of uppercase hex ---
	hexStr := strings.ToUpper(hex.EncodeToString(encrypted))
	if len(hexStr) < 6 {
		return hexStr
	}
	return hexStr[:6]
}

// tripleDESECBEncrypt encrypts data using Triple DES in ECB mode with no
// padding. The data length must be a multiple of 8 (the DES block size).
func tripleDESECBEncrypt(key, data []byte) []byte {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		// Key must be 24 bytes; this is guaranteed by the caller.
		panic(fmt.Sprintf("tms: triple des cipher: %v", err))
	}

	bs := block.BlockSize() // 8
	if len(data)%bs != 0 {
		panic(fmt.Sprintf("tms: data length %d not multiple of block size %d", len(data), bs))
	}

	encrypted := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Encrypt(encrypted[i:i+bs], data[i:i+bs])
	}

	return encrypted
}

// xorInto XORs each byte of s into buf at the corresponding position.
func xorInto(buf []byte, s string) {
	for i := 0; i < len(s); i++ {
		buf[i] ^= s[i]
	}
}

// maxLen returns the maximum of up to three integers.
func maxLen(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
