package scm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const rssAtomFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Feed</title>
  <entry>
    <title>Three</title>
    <id>https://example.com/3</id>
    <link href="https://example.com/3"/>
    <updated>2026-09-17T00:00:00Z</updated>
  </entry>
  <entry>
    <title>Two</title>
    <id>https://example.com/2</id>
    <link href="https://example.com/2"/>
    <updated>2026-06-21T00:00:00Z</updated>
  </entry>
  <entry>
    <title>No identity</title>
    <updated>2026-01-01T00:00:00Z</updated>
  </entry>
  <entry>
    <title>One</title>
    <id>https://example.com/1</id>
    <link href="https://example.com/1"/>
    <updated>2026-01-01T00:00:00Z</updated>
  </entry>
</feed>`

// startFeed serves body at /feed.xml and returns an RSS provider plus the URL.
func startFeed(t *testing.T, status int, body string, captureUA *string) (*RSS, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if captureUA != nil {
			*captureUA = req.Header.Get("User-Agent")
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &RSS{client: srv.Client()}, srv.URL + "/feed.xml"
}

func TestRSSListReleases(t *testing.T) {
	var ua string
	r, url := startFeed(t, http.StatusOK, rssAtomFeed, &ua)

	releases, err := r.ListReleases(context.Background(), url, 10)
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	// The "No identity" entry is skipped (no guid/id → no link either).
	if len(releases) != 3 {
		t.Fatalf("got %d releases, want 3: %+v", len(releases), releases)
	}

	first := releases[0]
	if first.Tag != "https://example.com/3" {
		t.Errorf("first.Tag = %q", first.Tag)
	}
	if first.Name != "Three" {
		t.Errorf("first.Name = %q", first.Name)
	}
	if first.URL != "https://example.com/3" {
		t.Errorf("first.URL = %q", first.URL)
	}
	want := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if !first.PublishedAt.Equal(want) {
		t.Errorf("first.PublishedAt = %v, want %v", first.PublishedAt, want)
	}
	if first.Prerelease {
		t.Errorf("feed releases must not be prereleases")
	}
	if ua != "relwatch" {
		t.Errorf("User-Agent = %q, want relwatch", ua)
	}
}

func TestRSSListReleasesLimit(t *testing.T) {
	r, url := startFeed(t, http.StatusOK, rssAtomFeed, nil)

	releases, err := r.ListReleases(context.Background(), url, 2)
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(releases))
	}
	if releases[0].Tag != "https://example.com/3" || releases[1].Tag != "https://example.com/2" {
		t.Errorf("limit kept wrong items: %+v", releases)
	}
}

func TestRSSBadFeedURL(t *testing.T) {
	r := NewRSS()
	if _, err := r.ListReleases(context.Background(), "alpinelinux.org/atom.xml", 10); err == nil {
		t.Fatal("expected error for non-http URL")
	}
}

func TestRSSNon200(t *testing.T) {
	r, url := startFeed(t, http.StatusNotFound, "nope", nil)
	if _, err := r.ListReleases(context.Background(), url, 10); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestRSSUnsupportedBody(t *testing.T) {
	r, url := startFeed(t, http.StatusOK, `<html><body>not a feed</body></html>`, nil)
	if _, err := r.ListReleases(context.Background(), url, 10); err == nil {
		t.Fatal("expected error for non-feed body")
	}
}

func TestRSSFactory(t *testing.T) {
	prov, err := NewFactory(Config{}).For("rss", "")
	if err != nil {
		t.Fatalf("For(rss): %v", err)
	}
	if prov.Name() != "rss" {
		t.Errorf("Name() = %q, want rss", prov.Name())
	}
}
