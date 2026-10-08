package auth

import (
	"bytes"
	"encoding/base32"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 rows (truncated to 6 digits).
func TestTOTPRFCVectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	cases := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		step, ok := VerifyTOTP(secret, c.code, time.Unix(c.unix, 0))
		if !ok || step != c.unix/30 {
			t.Errorf("t=%d code=%s: ok=%v step=%d", c.unix, c.code, ok, step)
		}
	}
	if _, ok := VerifyTOTP(secret, "000000", time.Unix(59, 0)); ok {
		t.Error("wrong code accepted")
	}
	// Two steps away is outside the skew window.
	if _, ok := VerifyTOTP(secret, "287082", time.Unix(59+60, 0)); ok {
		t.Error("stale code accepted")
	}
}

func TestPassword(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrWeakPassword {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword(h, "correct horse battery"); !ok || err != nil {
		t.Fatalf("verify: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword(h, "correct horse battery!"); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestSealer(t *testing.T) {
	s, err := NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := s.Seal("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Open(sealed)
	if err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("open: %q %v", got, err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := s.Open(sealed); err == nil {
		t.Fatal("tampered ciphertext opened")
	}
}

func TestFailureLimiter(t *testing.T) {
	l := NewFailureLimiter(2, time.Hour)
	l.Fail("k")
	if !l.Allowed("k") {
		t.Fatal("locked after 1 failure")
	}
	l.Fail("k")
	if l.Allowed("k") {
		t.Fatal("not locked after 2 failures")
	}
	l.Reset("k")
	if !l.Allowed("k") {
		t.Fatal("still locked after reset")
	}
}
