package store

import (
	"errors"
	"testing"
	"time"
)

func TestUsers(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	u, err := s.CreateUser("a@b.c", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 || u.Email != "a@b.c" || u.PasswordHash != "hash" {
		t.Fatalf("unexpected user: %+v", u)
	}

	if _, err := s.CreateUser("a@b.c", "hash2"); err == nil {
		t.Fatal("expected duplicate email error")
	}

	byEmail, err := s.GetUserByEmail("a@b.c")
	if err != nil || byEmail.ID != u.ID {
		t.Fatalf("GetUserByEmail = %+v, %v", byEmail, err)
	}

	byID, err := s.GetUserByID(u.ID)
	if err != nil || byID.Email != "a@b.c" {
		t.Fatalf("GetUserByID = %+v, %v", byID, err)
	}

	if _, err := s.GetUserByEmail("nope@b.c"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	users, err := s.ListUsers()
	if err != nil || len(users) != 1 {
		t.Fatalf("ListUsers = %+v, %v", users, err)
	}
}

func TestSessions(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	u, err := s.CreateUser("a@b.c", "hash")
	if err != nil {
		t.Fatal(err)
	}

	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.CreateSession(u.ID, "token1", "csrf1", exp); err != nil {
		t.Fatal(err)
	}

	sess, err := s.GetSession("token1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID || !sess.ExpiresAt.Equal(exp) || sess.CSRFToken != "csrf1" {
		t.Fatalf("unexpected session: %+v", sess)
	}

	if _, err := s.GetSession("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := s.DeleteSession("token1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession("token1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMeta(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, ok, err := s.GetMeta("last_checked_at"); err != nil || ok {
		t.Fatalf("expected missing meta, got ok=%v err=%v", ok, err)
	}

	if err := s.SetMeta("last_checked_at", "123"); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := s.GetMeta("last_checked_at"); err != nil || !ok || v != "123" {
		t.Fatalf("expected 123, got %q ok=%v err=%v", v, ok, err)
	}

	if err := s.SetMeta("last_checked_at", "456"); err != nil {
		t.Fatal(err)
	}
	if v, _, _ := s.GetMeta("last_checked_at"); v != "456" {
		t.Fatalf("expected overwrite to 456, got %q", v)
	}
}
