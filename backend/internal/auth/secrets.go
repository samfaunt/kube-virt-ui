package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"strings"
)

// Sealer encrypts small secrets (TOTP keys) at rest with AES-256-GCM.
type Sealer struct{ aead cipher.AEAD }

func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, errors.New("secret key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

func (s *Sealer) Seal(plaintext string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (s *Sealer) Open(sealed []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealed value too short")
	}
	pt, err := s.aead.Open(nil, sealed[:n], sealed[n:], nil)
	return string(pt), err
}

// NewToken returns a 256-bit random token for invite links and session ids.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToLower(b32.EncodeToString(buf)), nil
}

// HashToken is how tokens are stored: they are high entropy, so a plain
// SHA-256 is sufficient and lets the database be leaked without exposing
// usable links or sessions.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewRecoveryCodes returns n single-use codes formatted xxxxx-xxxxx.
func NewRecoveryCodes(n int) ([]string, error) {
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	codes := make([]string, n)
	for i := range codes {
		buf := make([]byte, 7)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		s := strings.ToLower(enc.EncodeToString(buf))[:10]
		codes[i] = s[:5] + "-" + s[5:]
	}
	return codes, nil
}

// NormalizeRecoveryCode makes user-typed codes comparable to stored ones.
func NormalizeRecoveryCode(code string) string {
	code = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	if len(code) == 10 {
		code = code[:5] + "-" + code[5:]
	}
	return code
}
