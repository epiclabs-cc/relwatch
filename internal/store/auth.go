package store

import (
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID        int64
	Token     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	CSRFToken string
}

func (s *Store) CreateUser(email, passwordHash string) (User, error) {
	res, err := s.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?,?)`, email, passwordHash)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Email: email, PasswordHash: passwordHash}, nil
}

func (s *Store) GetUserByEmail(email string) (User, error) {
	return s.userBy(`SELECT id, email, password_hash, created_at FROM users WHERE email = ?`, email)
}

func (s *Store) GetUserByID(id int64) (User, error) {
	return s.userBy(`SELECT id, email, password_hash, created_at FROM users WHERE id = ?`, id)
}

func (s *Store) userBy(q string, arg any) (User, error) {
	var u User
	var createdAt int64
	err := s.db.QueryRow(q, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = time.Unix(createdAt, 0)
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, password_hash, created_at FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var createdAt int64
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &createdAt); err != nil {
			return nil, err
		}
		u.CreatedAt = time.Unix(createdAt, 0)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) CreateSession(userID int64, token, csrfToken string, expiresAt time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token, user_id, csrf_token, expires_at) VALUES (?,?,?,?)`,
		token, userID, csrfToken, expiresAt.Unix())
	return err
}

func (s *Store) GetSession(token string) (Session, error) {
	var sess Session
	var createdAt, expiresAt int64
	err := s.db.QueryRow(`SELECT id, token, user_id, created_at, expires_at, csrf_token FROM sessions WHERE token = ?`, token).
		Scan(&sess.ID, &sess.Token, &sess.UserID, &createdAt, &expiresAt, &sess.CSRFToken)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	sess.CreatedAt = time.Unix(createdAt, 0)
	sess.ExpiresAt = time.Unix(expiresAt, 0)
	return sess, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}
