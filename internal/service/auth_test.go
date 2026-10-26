package service

import (
	"errors"
	"testing"

	"relwatch/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st)
}

func TestCreateUserAndAuthenticate(t *testing.T) {
	svc := newTestService(t)

	u, err := svc.CreateUser("Admin@Example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "admin@example.com" {
		t.Fatalf("email not normalized: %q", u.Email)
	}

	if _, err := svc.Authenticate("admin@example.com", "password123"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if _, err := svc.Authenticate("admin@example.com", "wrong"); err == nil {
		t.Fatal("expected wrong-password error")
	}
	if _, err := svc.Authenticate("nobody@example.com", "password123"); err == nil {
		t.Fatal("expected unknown-email error")
	}

	if _, err := svc.CreateUser("x@y.z", "short"); err == nil {
		t.Fatal("expected short-password error")
	}
}

func TestSessions(t *testing.T) {
	svc := newTestService(t)

	u, err := svc.CreateUser("a@b.c", "password123")
	if err != nil {
		t.Fatal(err)
	}

	token, err := svc.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.UserForSession(token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserForSession = %+v, %v", got, err)
	}

	if _, err := svc.UserForSession("bogus"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for bogus token, got %v", err)
	}

	if err := svc.DeleteSession(token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UserForSession(token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
