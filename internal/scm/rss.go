package scm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"relwatch/internal/feed"
)

// maxFeedBytes caps how much of a feed response is read, so a hostile or
// misconfigured URL cannot exhaust memory.
const maxFeedBytes = 10 << 20 // 10 MiB

// RSS tracks a web feed (Atom or RSS 2.0) as a release source. The project's
// Repo field holds the feed URL, and each item becomes a release: the item's
// guid/id is the tag (the stable upsert key), the title the release name, and
// the item link the release URL. Feeds have no prerelease concept, so releases
// are never marked prerelease.
type RSS struct {
	client *http.Client
}

func NewRSS() *RSS {
	return &RSS{client: &http.Client{Timeout: 30 * time.Second}}
}

func (r *RSS) Name() string { return "rss" }

func (r *RSS) ListReleases(ctx context.Context, repo string, limit int) ([]Release, error) {
	url := strings.TrimSpace(repo)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("invalid feed URL %q, expected http(s)://...", repo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "relwatch")
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml;q=0.9, text/xml;q=0.8")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed %s: unexpected status %s", url, resp.Status)
	}

	items, err := feed.Parse(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, fmt.Errorf("feed %s: %w", url, err)
	}

	if limit <= 0 {
		limit = 10
	}
	out := make([]Release, 0, len(items))
	for _, it := range items {
		if it.GUID == "" {
			continue // no stable identity → cannot upsert safely
		}
		out = append(out, Release{
			Tag:         it.GUID,
			Name:        it.Title,
			PublishedAt: it.PublishedAt,
			URL:         it.Link,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}
