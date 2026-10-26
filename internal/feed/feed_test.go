package feed

import (
	"strings"
	"testing"
	"time"
)

const atomDoc = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Alpine Linux News</title>
  <id>http://alpinelinux.org/atom.xml</id>
  <link href="https://alpinelinux.org/atom.xml" rel="self" type="application/rss+xml" />
  <updated>2026-09-17T19:53:38Z</updated>
  <entry>
    <title>Alpine 3.21.8 released</title>
    <link href="https://alpinelinux.org/posts/a.html"/>
    <id>http://alpinelinux.org/posts/a.html</id>
    <updated>2026-09-17T00:00:00Z</updated>
    <summary>Alpine 3.21.8 released</summary>
  </entry>
  <entry>
    <title>No id here</title>
    <link rel="alternate" href="https://alpinelinux.org/posts/b.html"/>
    <link rel="self" href="https://alpinelinux.org/self"/>
    <updated>not a date</updated>
  </entry>
</feed>`

const rssDoc = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example</title>
    <item>
      <title>v1.2.0</title>
      <link>https://example.com/v1.2.0</link>
      <guid>example-1.2.0</guid>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>v1.1.0</title>
      <link>https://example.com/v1.1.0</link>
      <pubDate>garbage</pubDate>
    </item>
  </channel>
</rss>`

func TestParseAtom(t *testing.T) {
	items, err := Parse(strings.NewReader(atomDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.GUID != "http://alpinelinux.org/posts/a.html" {
		t.Errorf("first.GUID = %q", first.GUID)
	}
	if first.Title != "Alpine 3.21.8 released" {
		t.Errorf("first.Title = %q", first.Title)
	}
	if first.Link != "https://alpinelinux.org/posts/a.html" {
		t.Errorf("first.Link = %q", first.Link)
	}
	want := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if !first.PublishedAt.Equal(want) {
		t.Errorf("first.PublishedAt = %v, want %v", first.PublishedAt, want)
	}

	// Missing <id> → fall back to the alternate link; bad date → zero time.
	second := items[1]
	if second.GUID != "https://alpinelinux.org/posts/b.html" {
		t.Errorf("second.GUID = %q (want alternate link)", second.GUID)
	}
	if second.Link != "https://alpinelinux.org/posts/b.html" {
		t.Errorf("second.Link = %q (want rel=alternate)", second.Link)
	}
	if !second.PublishedAt.IsZero() {
		t.Errorf("second.PublishedAt = %v, want zero", second.PublishedAt)
	}
}

func TestParseRSS(t *testing.T) {
	items, err := Parse(strings.NewReader(rssDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].GUID != "example-1.2.0" || items[0].Title != "v1.2.0" {
		t.Errorf("first item = %+v", items[0])
	}
	want := time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*3600))
	if !items[0].PublishedAt.Equal(want) {
		t.Errorf("first.PublishedAt = %v, want %v", items[0].PublishedAt, want)
	}

	// No <guid> → fall back to <link>; unparseable date → zero.
	if items[1].GUID != "https://example.com/v1.1.0" {
		t.Errorf("second.GUID = %q (want link fallback)", items[1].GUID)
	}
	if !items[1].PublishedAt.IsZero() {
		t.Errorf("second.PublishedAt = %v, want zero", items[1].PublishedAt)
	}
}

func TestParseUnsupported(t *testing.T) {
	if _, err := Parse(strings.NewReader(`<html><body>nope</body></html>`)); err == nil {
		t.Fatal("expected error for unsupported root element")
	}
	if _, err := Parse(strings.NewReader(`not xml at all`)); err == nil {
		t.Fatal("expected error for non-XML input")
	}
}

func TestParseTimeLayouts(t *testing.T) {
	cases := map[string]bool{
		"2026-09-17T00:00:00Z":            false,
		"Mon, 02 Jan 2006 15:04:05 -0700": false,
		"Mon, 2 Jan 2006 15:04:05 MST":    false,
		"":                                true,
		"nonsense":                        true,
	}
	for in, wantZero := range cases {
		if got := parseTime(in).IsZero(); got != wantZero {
			t.Errorf("parseTime(%q) zero = %v, want %v", in, got, wantZero)
		}
	}
}
