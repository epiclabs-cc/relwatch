package scm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitLabListReleases(t *testing.T) {
	body := `[
		{
			"tag_name": "v16.0.0",
			"name": "Release 16.0",
			"released_at": "2026-05-19T12:00:00.000Z",
			"description": "a release"
		},
		{
			"tag_name": "v15.9.0",
			"name": "",
			"released_at": "2026-03-15T08:30:00.000Z"
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/gitlab-org/gitlab/releases" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("per_page"); got != "10" {
			t.Errorf("expected per_page=10, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	p, err := NewGitLab("glpat-test", srv.URL+"/api/v4")
	if err != nil {
		t.Fatal(err)
	}

	releases, err := p.ListReleases(context.Background(), "gitlab-org/gitlab", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}

	first := releases[0]
	if first.Tag != "v16.0.0" || first.Name != "Release 16.0" {
		t.Fatalf("unexpected first release: %+v", first)
	}
	want, _ := time.Parse(time.RFC3339, "2026-05-19T12:00:00Z")
	if !first.PublishedAt.Equal(want) {
		t.Fatalf("published_at = %v, want %v", first.PublishedAt, want)
	}
	host := strings.TrimSuffix(srv.URL, "/")
	if want := host + "/gitlab-org/gitlab/-/releases/v16.0.0"; first.URL != want {
		t.Fatalf("url = %q, want %q", first.URL, want)
	}
	if releases[1].URL == "" {
		t.Fatal("expected URL on second release")
	}
}
