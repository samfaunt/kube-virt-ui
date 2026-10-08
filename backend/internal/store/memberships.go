package store

import (
	"context"
	"database/sql"
	"time"
)

type Membership struct {
	UserID    int64  `json:"userId"`
	Namespace string `json:"namespace"`
	Role      string `json:"role"`
}

func upsertMembership(ctx context.Context, tx *sql.Tx, m Membership, now time.Time) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO memberships (user_id, namespace, role, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (user_id, namespace) DO UPDATE SET role = excluded.role`,
		m.UserID, m.Namespace, m.Role, unix(now))
	return err
}

func (s *Store) UpsertMembership(ctx context.Context, m Membership, now time.Time) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return upsertMembership(ctx, tx, m, now) })
}

func (s *Store) DeleteMembership(ctx context.Context, userID int64, namespace string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM memberships WHERE user_id = ? AND namespace = ?`, userID, namespace)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Membership(ctx context.Context, userID int64, namespace string) (Membership, error) {
	m := Membership{UserID: userID, Namespace: namespace}
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM memberships WHERE user_id = ? AND namespace = ?`, userID, namespace).Scan(&m.Role)
	return m, notFound(err)
}

// Memberships lists memberships for one user, or for everyone if userID is 0.
func (s *Store) Memberships(ctx context.Context, userID int64) ([]Membership, error) {
	q := `SELECT user_id, namespace, role FROM memberships`
	var args []any
	if userID != 0 {
		q += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY namespace`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.UserID, &m.Namespace, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
