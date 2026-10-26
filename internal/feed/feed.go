// Package feed parses web syndication feeds into a uniform list of items. It
// supports Atom and RSS 2.0 using only encoding/xml, so the "rss" provider in
// internal/scm adds no third-party dependency. RSS 1.0/RDF is not handled yet.
package feed

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Item is a single entry (Atom) or item (RSS) from a feed.
type Item struct {
	// GUID is a stable identity: Atom <id> or RSS <guid>, falling back to the
	// item link when the feed omits one. It may be empty if the item has neither.
	GUID        string
	Title       string
	Link        string
	PublishedAt time.Time // zero when the feed has no parseable date
}

// Parse reads an Atom or RSS 2.0 document and returns its items in feed order
// (typically newest first).
func Parse(r io.Reader) ([]Item, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	switch root := rootName(data); strings.ToLower(root) {
	case "feed":
		return parseAtom(data)
	case "rss":
		return parseRSS(data)
	default:
		return nil, fmt.Errorf("unsupported feed format (root element <%s>)", root)
	}
}

func rootName(data []byte) string {
	var probe struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(data, &probe); err != nil {
		return ""
	}
	return probe.XMLName.Local
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	ID        string     `xml:"id"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Links     []atomLink `xml:"link"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func parseAtom(data []byte) ([]Item, error) {
	var f atomFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing Atom feed: %w", err)
	}
	items := make([]Item, 0, len(f.Entries))
	for _, e := range f.Entries {
		link := atomLinkHref(e.Links)
		guid := strings.TrimSpace(e.ID)
		if guid == "" {
			guid = link
		}
		date := e.Updated
		if strings.TrimSpace(date) == "" {
			date = e.Published
		}
		items = append(items, Item{
			GUID:        guid,
			Title:       strings.TrimSpace(e.Title),
			Link:        link,
			PublishedAt: parseTime(date),
		})
	}
	return items, nil
}

// atomLinkHref prefers the "alternate" link (rel empty or "alternate"), which
// is the human-facing page; it falls back to the first link present.
func atomLinkHref(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "" || l.Rel == "alternate" {
			return strings.TrimSpace(l.Href)
		}
	}
	if len(links) > 0 {
		return strings.TrimSpace(links[0].Href)
	}
	return ""
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	GUID    string `xml:"guid"`
	PubDate string `xml:"pubDate"`
	Date    string `xml:"date"` // Dublin Core dc:date, when present
}

func parseRSS(data []byte) ([]Item, error) {
	var f rssFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing RSS feed: %w", err)
	}
	items := make([]Item, 0, len(f.Channel.Items))
	for _, it := range f.Channel.Items {
		link := strings.TrimSpace(it.Link)
		guid := strings.TrimSpace(it.GUID)
		if guid == "" {
			guid = link
		}
		date := it.PubDate
		if strings.TrimSpace(date) == "" {
			date = it.Date
		}
		items = append(items, Item{
			GUID:        guid,
			Title:       strings.TrimSpace(it.Title),
			Link:        link,
			PublishedAt: parseTime(date),
		})
	}
	return items, nil
}

// timeLayouts covers RFC3339 (Atom) plus the RFC822/1123 variants RSS feeds use.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.RFC850,
	time.ANSIC,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
