package store

import (
	"context"
	"database/sql"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	TOTPSecret   []byte    `json:"-"` // sealed
	TOTPLastStep int64     `json:"-"`
	IsAdmin      bool      `json:"isAdmin"`
	Disabled     bool      `json:"disabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

const userCols = `id, username, email, password_hash, totp_secret, totp_last_step, is_admin, disabled, created_at`

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.TOTPSecret,
		&u.TOTPLastStep, &u.IsAdmin, &u.Disabled, &created)
	u.CreatedAt = time.Unix(created, 0)
	return u, notFound(err)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetDisabled disables or enables a user; disabling also ends their sessions.
func (s *Store) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET disabled = ? WHERE id = ?`, disabled, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if disabled {
			_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id)
		}
		return err
	})
}

// AdvanceTOTPStep records step as used. It returns false if step is not
// newer than the last accepted one, which is how TOTP replay is rejected.
func (s *Store) AdvanceTOTPStep(ctx context.Context, userID, step int64) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_last_step = ? WHERE id = ? AND totp_last_step < ?`, step, userID, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UseRecoveryCode consumes a recovery code; false if unknown or already used.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, codeHash string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`,
		unix(now), userID, codeHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// insertUser is used inside invite redemption so the user only exists if the
// invite was successfully claimed.
func insertUser(ctx context.Context, tx *sql.Tx, u User, recoveryHashes []string) (User, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (username, email, password_hash, totp_secret, totp_last_step, is_admin, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.TOTPSecret, u.TOTPLastStep, u.IsAdmin, unix(u.CreatedAt))
	if isUnique(err) {
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	u.ID, _ = res.LastInsertId()
	for _, h := range recoveryHashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, u.ID, h); err != nil {
			return User{}, err
		}
	}
	return u, nil
}
