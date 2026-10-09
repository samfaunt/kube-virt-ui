package api

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// captureLogs routes the default slog logger into a buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestInternalErrorDoesNotLogInviteToken(t *testing.T) {
	buf := captureLogs(t)
	const token = "s3cr3t-invite-token-value"

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Get("/invites/{token}", func(w http.ResponseWriter, r *http.Request) {
			internalError(w, r, errors.New("boom"))
		})
		r.Post("/invites/{token}/redeem", func(w http.ResponseWriter, r *http.Request) {
			internalError(w, r, errors.New("boom"))
		})
	})

	for _, tc := range []struct{ method, path, pattern string }{
		{http.MethodGet, "/api/invites/" + token, "/api/invites/{token}"},
		{http.MethodPost, "/api/invites/" + token + "/redeem", "/api/invites/{token}/redeem"},
	} {
		buf.Reset()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d, want 500", tc.path, rec.Code)
		}
		out := buf.String()
		if strings.Contains(out, token) {
			t.Errorf("%s: log contains invite token: %s", tc.path, out)
		}
		if !strings.Contains(out, tc.pattern) {
			t.Errorf("%s: log missing route pattern %q: %s", tc.path, tc.pattern, out)
		}
	}
}

func TestInternalErrorWithoutRouteContext(t *testing.T) {
	buf := captureLogs(t)
	rec := httptest.NewRecorder()
	internalError(rec, httptest.NewRequest(http.MethodGet, "/api/invites/leaky", nil), errors.New("boom"))
	out := buf.String()
	if strings.Contains(out, "leaky") {
		t.Errorf("log contains raw path: %s", out)
	}
	if !strings.Contains(out, "route=unknown") {
		t.Errorf("log missing fallback route: %s", out)
	}
}
