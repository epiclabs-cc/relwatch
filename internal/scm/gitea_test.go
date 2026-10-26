package scm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGiteaListReleases(t *testing.T) {
	body := `[
		{
			"id": 1,
			"tag_name": "v0.25.0",
			"name": "Gitea 0.25",
			"published_at": "2026-05-12T10:00:00Z",
			"html_url": "https://example.com/go-gitea/gitea/releases/tag/v0.25.0"
		},
		{
			"id": 2,
			"tag_name": "v0.24.0",
			"published_at": "2025-12-01T09:00:00Z",
			"html_url": "https://example.com/go-gitea/gitea/releases/tag/v0.24.0"
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"1.22.0"}`))
			return
		}
		if r.URL.Path != "/api/v1/repos/go-gitea/gitea/releases" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Errorf("expected limit=10, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	p, err := NewGitea("", srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	releases, err := p.ListReleases(context.Background(), "go-gitea/gitea", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}

	first := releases[0]
	if first.Tag != "v0.25.0" || first.Name != "Gitea 0.25" {
		t.Fatalf("unexpected first release: %+v", first)
	}
	want, _ := time.Parse(time.RFC3339, "2026-05-12T10:00:00Z")
	if !first.PublishedAt.Equal(want) {
		t.Fatalf("published_at = %v, want %v", first.PublishedAt, want)
	}
	if want := "https://example.com/go-gitea/gitea/releases/tag/v0.25.0"; first.URL != want {
		t.Fatalf("url = %q, want %q", first.URL, want)
	}
	if releases[1].Name != "" || releases[1].URL == "" {
		t.Fatalf("unexpected second release: %+v", releases[1])
	}
}

func TestGiteaInvalidRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.22.0"}`))
	}))
	defer srv.Close()

	p, err := NewGitea("", srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.ListReleases(context.Background(), "notanownerrepopair", 10); err == nil {
		t.Fatal("expected error for invalid repo")
	}
}
