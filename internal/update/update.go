// Package update checks the running relwatch version against the project's
// GitHub releases so that `serve` can surface "a newer version is available".
// Installation and upgrading are handled externally by install-relwatch.sh.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	version "github.com/hashicorp/go-version"
)

// DefaultServer is the GitHub "latest release" endpoint used when settings do
// not override it.
const DefaultServer = "https://api.github.com/repos/epiclabs-cc/relwatch/releases/latest"

// DefaultTimeout bounds each HTTP request when the caller supplies no client.
const DefaultTimeout = 30 * time.Second

// ErrNoRelease is returned when GitHub has no published releases yet.
var ErrNoRelease = errors.New("no release published yet")

// Client talks to an update server. Server defaults to DefaultServer; HTTP may
// be nil, in which case a client with DefaultTimeout is used.
type Client struct {
	Server string
	HTTP   *http.Client
}

// NewClient builds a client for the given server (empty means DefaultServer).
func NewClient(server string, hc *http.Client) *Client {
	if strings.TrimSpace(server) == "" {
		server = DefaultServer
	}
	return &Client{Server: strings.TrimRight(server, "/"), HTTP: hc}
}

// Status is the outcome of a version check.
type Status struct {
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Available  bool   `json:"update_available"`
	Comparable bool   `json:"comparable"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: DefaultTimeout}
}

// Latest fetches the latest published release tag (GitHub's releases/latest
// endpoint) with the leading "v" stripped.
func (c *Client) Latest(ctx context.Context) (string, error) {
	body, status, err := c.get(ctx, c.Server)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", ErrNoRelease
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GET %s: unexpected status %d", c.Server, status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("release endpoint returned invalid JSON: %v", err)
	}
	v := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if v == "" {
		return "", ErrNoRelease
	}
	if _, err := version.NewVersion(v); err != nil {
		return "", fmt.Errorf("release tag is not a valid version %q", payload.TagName)
	}
	return v, nil
}

// Check compares current against the latest published version.
func (c *Client) Check(ctx context.Context, current string) (Status, error) {
	latest, err := c.Latest(ctx)
	if err != nil {
		return Status{}, err
	}
	available, comparable, err := Compare(current, latest)
	if err != nil {
		return Status{}, err
	}
	return Status{Current: current, Latest: latest, Available: available, Comparable: comparable}, nil
}

// Compare reports whether latest is newer than current. comparable is false
// when current is not a parseable version (a dev/source build), in which case
// available is false and no error is returned.
func Compare(current, latest string) (available, comparable bool, err error) {
	cur, err := version.NewVersion(current)
	if err != nil {
		return false, false, nil
	}
	lat, err := version.NewVersion(latest)
	if err != nil {
		return false, false, fmt.Errorf("invalid latest version %q", latest)
	}
	return lat.GreaterThan(cur), true, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "relwatch")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
