package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"relwatch/internal/store"
)

const sessionDuration = 30 * 24 * time.Hour

// CreateUser validates and stores a new user with a bcrypt-hashed password.
func (s *Service) CreateUser(email, password string) (store.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return store.User{}, errors.New("email is required and must contain '@'")
	}
	if len(password) < 8 {
		return store.User{}, errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return store.User{}, err
	}
	return s.st.CreateUser(email, string(hash))
}

func (s *Service) ListUsers() ([]store.User, error) {
	return s.st.ListUsers()
}

// Authenticate verifies email+password. The error is the same for an unknown
// email and a wrong password, so neither leaks which one was wrong.
func (s *Service) Authenticate(email, password string) (store.User, error) {
	u, err := s.st.GetUserByEmail(strings.TrimSpace(strings.ToLower(email)))
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, errors.New("invalid email or password")
	}
	if err != nil {
		return store.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return store.User{}, errors.New("invalid email or password")
	}
	return u, nil
}

// CreateSession issues an opaque token for the user, valid for sessionDuration.
func (s *Service) CreateSession(userID int64) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	csrf, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.st.CreateSession(userID, token, csrf, time.Now().Add(sessionDuration)); err != nil {
		return "", err
	}
	return token, nil
}

// CSRFToken returns the per-session CSRF token used to guard write forms.
func (s *Service) CSRFToken(sessionToken string) (string, error) {
	sess, err := s.st.GetSession(sessionToken)
	if err != nil {
		return "", err
	}
	return sess.CSRFToken, nil
}

// UserForSession returns the user owning an unexpired session token.
func (s *Service) UserForSession(token string) (store.User, error) {
	sess, err := s.st.GetSession(token)
	if err != nil {
		return store.User{}, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.st.DeleteSession(token)
		return store.User{}, errors.New("session expired")
	}
	return s.st.GetUserByID(sess.UserID)
}

func (s *Service) DeleteSession(token string) error {
	return s.st.DeleteSession(token)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
