package service

import (
	"errors"
	"testing"

	"relwatch/internal/store"
)

func TestAddProjectValidation(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.AddProject("", "github", "a/b", ""); err == nil {
		t.Fatal("expected name-required error")
	}
	if _, err := svc.AddProject("x", "bogus", "a/b", ""); err == nil {
		t.Fatal("expected unsupported-provider error")
	}
	if _, err := svc.AddProject("x", "github", "", ""); err == nil {
		t.Fatal("expected repo-required error")
	}

	p, err := svc.AddProject("X", "GITHUB", "cli/cli", "https://gitlab.example.com/api/v4")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "X" || p.Provider != "github" || p.Repo != "cli/cli" || p.InstanceURL != "https://gitlab.example.com/api/v4" {
		t.Fatalf("unexpected project: %+v", p)
	}
}

func TestAddImageValidation(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.AddImage("", "docker.io", "library/x"); err == nil {
		t.Fatal("expected name-required error")
	}
	if _, err := svc.AddImage("x", "", "library/x"); err == nil {
		t.Fatal("expected registry-required error")
	}
	if _, err := svc.AddImage("x", "docker.io", ""); err == nil {
		t.Fatal("expected repo-required error")
	}
}

func TestAddFeedValidation(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.AddFeed("", "https://example.com/f.xml"); err == nil {
		t.Fatal("expected name-required error")
	}
	if _, err := svc.AddFeed("x", "notaurl"); err == nil {
		t.Fatal("expected url error")
	}

	f, err := svc.AddFeed("x", "https://example.com/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if f.URL != "https://example.com/feed.xml" {
		t.Fatalf("unexpected feed: %+v", f)
	}
}

func TestDeleteNotFound(t *testing.T) {
	svc := newTestService(t)

	if err := svc.DeleteProject(999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DeleteProject = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteImage(999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DeleteImage = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteFeed(999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DeleteFeed = %v, want ErrNotFound", err)
	}
}
