package store

import (
	"context"
	"strings"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, idHash string, userID int64, now, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		idHash, userID, unix(now), unix(expires))
	return err
}

// SessionUser returns the user owning an unexpired session.
func (s *Store) SessionUser(ctx context.Context, idHash string, now time.Time) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+prefixed("u.", userCols)+` FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.id_hash = ? AND s.expires_at > ?`, idHash, unix(now)))
}

func (s *Store) DeleteSession(ctx context.Context, idHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash = ?`, idHash)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, unix(now))
	return err
}

func prefixed(p, cols string) string {
	return p + strings.ReplaceAll(cols, ", ", ", "+p)
}
