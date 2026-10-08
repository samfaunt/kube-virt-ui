package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"time"
)

// TOTP per RFC 6238: SHA-1, 6 digits, 30 second steps. These are the only
// parameters every authenticator app supports.
const (
	totpPeriod = 30
	totpDigits = 6
	// totpSkew accepts codes one step either side of now for clock drift.
	totpSkew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret, base32 encoded.
func NewTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// TOTPURI returns the otpauth:// URI that authenticator apps scan as a QR code.
func TOTPURI(issuer, account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprint(totpDigits))
	v.Set("period", fmt.Sprint(totpPeriod))
	label := url.PathEscape(issuer + ":" + account)
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// VerifyTOTP checks code against secret at time now. On success it returns
// the matched time step; callers must reject steps at or below the last one
// they accepted so that a code cannot be replayed.
func VerifyTOTP(secret, code string, now time.Time) (step int64, ok bool) {
	key, err := b32.DecodeString(secret)
	if err != nil || len(code) != totpDigits {
		return 0, false
	}
	current := now.Unix() / totpPeriod
	for d := int64(-totpSkew); d <= totpSkew; d++ {
		s := current + d
		if subtle.ConstantTimeCompare([]byte(hotp(key, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

func hotp(key []byte, counter int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, bin%1_000_000)
}

// TOTPCode returns the code for secret at time t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(secret)
	if err != nil {
		return "", err
	}
	return hotp(key, t.Unix()/totpPeriod), nil
}
