package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kvui/internal/auth"
	"kvui/internal/store"
)

const (
	issuer            = "KubeVirt UI"
	recoveryCodeCount = 10
	defaultInviteTTL  = 72 * time.Hour
	maxInviteTTL      = 7 * 24 * time.Hour
)

var usernameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// CreateInvite stores a new invite and returns it with its one-time link.
// The raw token is never stored, so the link cannot be shown again.
func CreateInvite(ctx context.Context, st *store.Store, publicURL string, inv store.Invite) (store.Invite, string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return store.Invite{}, "", err
	}
	inv, err = st.CreateInvite(ctx, auth.HashToken(token), inv)
	if err != nil {
		return store.Invite{}, "", err
	}
	return inv, strings.TrimRight(publicURL, "/") + "/invite/" + token, nil
}

// usableInvite loads the invite named in the URL, writing the error
// response itself when it is unknown or no longer usable.
func (s *Server) usableInvite(w http.ResponseWriter, r *http.Request) (store.Invite, bool) {
	ipKey := "ip:" + clientIP(r)
	if !s.inviteLimiter.Allowed(ipKey) {
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
		return store.Invite{}, false
	}
	inv, err := s.Store.InviteByTokenHash(r.Context(), auth.HashToken(chi.URLParam(r, "token")))
	if errors.Is(err, store.ErrNotFound) {
		s.inviteLimiter.Fail(ipKey)
		writeError(w, http.StatusNotFound, "invite not found")
		return store.Invite{}, false
	}
	if err != nil {
		internalError(w, r, err)
		return store.Invite{}, false
	}
	if !inv.Usable(s.now()) {
		writeError(w, http.StatusGone, "invite has expired, been revoked or already been used")
		return store.Invite{}, false
	}
	return inv, true
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	inv, ok := s.usableInvite(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email": inv.Email, "namespace": inv.Namespace, "role": inv.Role,
		"makeAdmin": inv.MakeAdmin, "expiresAt": inv.ExpiresAt,
	})
}

// inviteTOTP generates the TOTP secret the new user enrols in their
// authenticator. Calling it again replaces the secret.
func (s *Server) inviteTOTP(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username string }
	if !decode(w, r, &req) {
		return
	}
	inv, ok := s.usableInvite(w, r)
	if !ok {
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		internalError(w, r, err)
		return
	}
	sealed, err := s.Sealer.Seal(secret)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := s.Store.SetInvitePendingTOTP(r.Context(), inv.ID, sealed); err != nil {
		internalError(w, r, err)
		return
	}
	account := req.Username
	if account == "" {
		account = inv.Email
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "uri": auth.TOTPURI(issuer, account, secret)})
}

func (s *Server) redeemInvite(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password, Code string }
	if !decode(w, r, &req) {
		return
	}
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	if !usernameRE.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters: a-z, 0-9, '.', '_' or '-'")
		return
	}
	inv, ok := s.usableInvite(w, r)
	if !ok {
		return
	}
	if inv.PendingTOTP == nil {
		writeError(w, http.StatusBadRequest, "set up the authenticator first")
		return
	}
	secret, err := s.Sealer.Open(inv.PendingTOTP)
	if err != nil {
		internalError(w, r, err)
		return
	}
	step, ok := auth.VerifyTOTP(secret, strings.TrimSpace(req.Code), s.now())
	if !ok {
		s.inviteLimiter.Fail("ip:" + clientIP(r))
		writeError(w, http.StatusBadRequest, "authenticator code is incorrect")
		return
	}
	pwHash, err := auth.HashPassword(req.Password)
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	codes, err := auth.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		internalError(w, r, err)
		return
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = auth.HashToken(c)
	}

	u, err := s.Store.RedeemInviteNewUser(r.Context(), inv, store.User{
		Username: req.Username, Email: inv.Email, PasswordHash: pwHash,
		TOTPSecret: inv.PendingTOTP, TOTPLastStep: step,
	}, hashes, s.now())
	switch {
	case errors.Is(err, store.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username is already taken")
		return
	case errors.Is(err, store.ErrInviteUnusable):
		writeError(w, http.StatusGone, "invite has expired, been revoked or already been used")
		return
	case err != nil:
		internalError(w, r, err)
		return
	}

	if inv.Namespace != "" {
		s.provision(r.Context(), u, inv.Namespace, inv.Role)
	}
	s.audit(r, &u, "invite.redeem", inv.Namespace, req.Username)
	if err := s.startSession(w, r, u); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "recoveryCodes": codes})
}

// acceptInvite applies an invite to the logged-in user, e.g. to join
// another namespace.
func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	inv, ok := s.usableInvite(w, r)
	if !ok {
		return
	}
	err := s.Store.AcceptInvite(r.Context(), inv, u.ID, s.now())
	if errors.Is(err, store.ErrInviteUnusable) {
		writeError(w, http.StatusGone, "invite has expired, been revoked or already been used")
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if inv.Namespace != "" {
		s.provision(r.Context(), *u, inv.Namespace, inv.Role)
	}
	s.audit(r, u, "invite.accept", inv.Namespace, u.Username)
	w.WriteHeader(http.StatusNoContent)
}
