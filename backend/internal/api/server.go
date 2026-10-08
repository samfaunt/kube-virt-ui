// Package api is the HTTP JSON API consumed by the frontend.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"k8s.io/client-go/kubernetes"

	"kvui/internal/auth"
	"kvui/internal/image"
	"kvui/internal/store"
	"kvui/internal/tenant"
)

type Options struct {
	Store         *store.Store
	Tenants       *tenant.Provisioner
	Kube          kubernetes.Interface // the backend's own identity; cluster-scoped reads only
	UploadProxy   *image.UploadProxy   // nil disables image uploads
	Sealer        *auth.Sealer
	PublicURL     string // e.g. https://vms.example.com, used in invite links
	SecureCookies bool
	TrustProxy    bool   // honour X-Forwarded-For from the ingress
	StaticDir     string // built frontend; empty disables static serving
}

type Server struct {
	Options
	loginLimiter  *auth.FailureLimiter
	inviteLimiter *auth.FailureLimiter
	now           func() time.Time
	// consoleRecheck is how often an open console re-validates the session
	// and membership; removing access closes consoles within this interval.
	consoleRecheck time.Duration
}

func New(o Options) *Server {
	return &Server{
		Options:       o,
		loginLimiter:  auth.NewFailureLimiter(10, 15*time.Minute),
		inviteLimiter: auth.NewFailureLimiter(20, 15*time.Minute),
		now:           time.Now,

		consoleRecheck: 30 * time.Second,
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	if s.TrustProxy {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.Recoverer, securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Use(requireCSRFHeader)
		r.Post("/login", s.login)
		r.Post("/logout", s.logout)
		r.Get("/invites/{token}", s.getInvite)
		r.Post("/invites/{token}/totp", s.inviteTOTP)
		r.Post("/invites/{token}/redeem", s.redeemInvite)

		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)
			r.Get("/me", s.me)
			r.Post("/invites/{token}/accept", s.acceptInvite)
			r.Route("/namespaces/{ns}", func(r chi.Router) {
				r.Get("/access", s.access)
				r.Get("/vm-options", s.vmOptions)
				r.Get("/vms", s.listVMs)
				r.Post("/vms", s.createVM)
				r.Get("/watch/vms", s.watchVMs)
				r.Get("/vms/{name}", s.getVM)
				r.Delete("/vms/{name}", s.deleteVM)
				r.Post("/vms/{name}/{action}", s.vmAction)
				r.Get("/vms/{name}/{kind}", s.console)
				r.Get("/images", s.listImages)
				r.Post("/images", s.createImage)
				r.Delete("/images/{image}", s.deleteImage)
				r.Post("/images/{image}/upload", s.uploadImage)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(requireAdmin)
				r.Get("/namespaces", s.adminNamespaces)
				r.Get("/invites", s.adminListInvites)
				r.Post("/invites", s.adminCreateInvite)
				r.Delete("/invites/{id}", s.adminRevokeInvite)
				r.Get("/users", s.adminListUsers)
				r.Patch("/users/{id}", s.adminUpdateUser)
				r.Put("/users/{id}/memberships/{ns}", s.adminPutMembership)
				r.Delete("/users/{id}/memberships/{ns}", s.adminDeleteMembership)
				r.Get("/audit", s.adminAudit)
			})
		})
	})

	if s.StaticDir != "" {
		r.NotFound(spa(s.StaticDir))
	}
	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		// Invite tokens live in the URL path; never leak them via Referer.
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// requireCSRFHeader: a custom header cannot be set cross-origin without a
// CORS preflight, which we never grant. Together with SameSite=Strict
// cookies this blocks cross-site request forgery.
func requireCSRFHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("X-Requested-With") != "kvui" {
				writeError(w, http.StatusForbidden, "missing X-Requested-With header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// spa serves the built frontend, falling back to index.html for client routes.
func spa(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if info, err := os.Stat(filepath.Join(dir, filepath.Clean("/"+r.URL.Path))); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// internalError logs err and returns a generic 500 so internals don't leak.
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) audit(r *http.Request, actor *store.User, action, namespace, target string) {
	e := store.AuditEntry{At: s.now(), Action: action, Namespace: namespace, Target: target, IP: clientIP(r)}
	if actor != nil {
		e.ActorID, e.Actor = &actor.ID, actor.Username
	}
	if err := s.Store.Audit(r.Context(), e); err != nil {
		slog.Error("audit write failed", "action", action, "err", err)
	}
}

// provision applies a membership to Kubernetes. Failures are logged rather
// than returned where the membership is already committed: UserConfig
// reprovisions on first use, so the user is not left stuck.
func (s *Server) provision(ctx context.Context, u store.User, namespace, role string) {
	if err := s.Tenants.Ensure(ctx, namespace, u.ID, u.Username, role); err != nil {
		slog.Error("provision membership", "user", u.Username, "namespace", namespace, "err", err)
	}
}
