package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"relwatch/internal/store"
)

// atomFeed builds a minimal Atom document from the given entry titles (used as
// both id and link suffix).
func atomFeed(titles ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?><feed xmlns="http://www.w3.org/2005/Atom">`)
	for _, title := range titles {
		fmt.Fprintf(&b, `<entry><title>%s</title><id>tag:%s</id>`+
			`<link href="https://example.com/%s"/><updated>2026-01-01T00:00:00Z</updated></entry>`,
			title, title, title)
	}
	b.WriteString(`</feed>`)
	return b.String()
}

// TestCheckAllFeed drives the full poll path for a tracked feed: the first run
// baselines silently, and a newly appeared feed item is reported next run.
func TestCheckAllFeed(t *testing.T) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body.Load().(string))
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if _, err := st.AddFeed("alpine", srv.URL); err != nil {
		t.Fatalf("add feed: %v", err)
	}

	opts := &options{}
	body.Store(atomFeed("A", "B"))
	if _, err := checkAll(context.Background(), opts, st, "", "", false, io.Discard); err != nil {
		t.Fatalf("baseline check: %v", err)
	}

	body.Store(atomFeed("C", "A", "B"))
	newRels, err := checkAll(context.Background(), opts, st, "", "", false, io.Discard)
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if len(newRels) != 1 || newRels[0].Tag != "tag:C" {
		t.Fatalf("got %+v, want exactly one new release tag:C", newRels)
	}
	if newRels[0].Provider != "feed" {
		t.Errorf("provider = %q, want feed", newRels[0].Provider)
	}
}
