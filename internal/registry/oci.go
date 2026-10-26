package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var manifestAccept = []string{
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.oci.image.index.v1+json",
}

// userAgent identifies relwatch to registries and their edge proxies. Some
// (notably Docker Hub and GitHub) return an HTML page or an empty token to
// requests without a User-Agent, so every registry request sets it.
const userAgent = "relwatch"

// canonicalHost rewrites the well-known Docker Hub aliases to the canonical
// registry endpoint. docker.io / index.docker.io serve the web UI and redirect
// to registry-1.docker.io; Go's http.Client follows the redirect but strips the
// Authorization header across hosts, so the token-authenticated retry would
// arrive unauthenticated (and sometimes land on an HTML page).
func canonicalHost(host string) string {
	switch strings.ToLower(host) {
	case "docker.io", "index.docker.io", "registry.docker.io":
		return "registry-1.docker.io"
	default:
		return host
	}
}

type ociClient struct {
	factory *Factory
	host    string
	http    *http.Client
}

func (c *ociClient) Name() string { return c.host }

func (c *ociClient) base(repo string) string {
	return fmt.Sprintf("%s://%s/v2/%s", c.factory.cfg.Scheme, canonicalHost(c.host), repo)
}

// newRequest builds a request carrying the relwatch User-Agent header.
func (c *ociClient) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func (c *ociClient) ListImages(ctx context.Context, repo string) ([]Image, error) {
	tags, err := c.listTags(ctx, repo)
	if err != nil {
		return nil, err
	}
	images := make([]Image, 0, len(tags))
	for _, tag := range tags {
		digest, err := c.manifestDigest(ctx, repo, tag)
		if err != nil {
			// Tags that cannot be resolved are skipped, not fatal.
			continue
		}
		images = append(images, Image{
			Repo:        repo,
			Tag:         tag,
			Digest:      digest,
			URL:         fmt.Sprintf("%s/manifests/%s", c.base(repo), tag),
			PublishedAt: time.Now(),
		})
	}
	return images, nil
}

func (c *ociClient) listTags(ctx context.Context, repo string) ([]string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.base(repo)+"/tags/list")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry %s: GET /v2/%s/tags/list: %s", c.host, repo, resp.Status)
	}
	var payload struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("registry %s: GET /v2/%s/tags/list returned non-JSON: %v", c.host, repo, err)
	}
	return payload.Tags, nil
}

func (c *ociClient) manifestDigest(ctx context.Context, repo, tag string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.base(repo)+"/manifests/"+tag)
	if err != nil {
		return "", err
	}
	for _, mt := range manifestAccept {
		req.Header.Add("Accept", mt)
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry %s: GET /v2/%s/manifests/%s: %s", c.host, repo, tag, resp.Status)
	}
	if d := resp.Header.Get("Docker-Content-Digest"); d != "" {
		return d, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// do performs one request, transparently satisfying a Bearer auth challenge
// from a v2 token endpoint when the registry returns 401 Unaunorized.
func (c *ociClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if t := c.factory.cfg.Token; t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	ch := parseChallenge(resp.Header.Get("WWW-Authenticate"))
	resp.Body.Close()
	if ch == nil || ch.realm == "" {
		return nil, fmt.Errorf("registry %s: unauthorized without bearer challenge", c.host)
	}
	token, err := c.exchangeToken(ctx, ch)
	if err != nil {
		return nil, err
	}
	retry := req.Clone(ctx)
	retry.Header.Set("Authorization", "Bearer "+token)
	return c.http.Do(retry)
}

type authChallenge struct{ realm, service, scope string }

func parseChallenge(h string) *authChallenge {
	rest, ok := strings.CutPrefix(h, "Bearer")
	if !ok {
		return nil
	}
	ch := &authChallenge{}
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), "\"")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "realm":
			ch.realm = val
		case "service":
			ch.service = val
		case "scope":
			ch.scope = val
		}
	}
	return ch
}

func (c *ociClient) exchangeToken(ctx context.Context, ch *authChallenge) (string, error) {
	u, err := url.Parse(ch.realm)
	if err != nil {
		return "", fmt.Errorf("registry %s: bad token realm: %v", c.host, err)
	}
	q := u.Query()
	if ch.service != "" {
		q.Set("service", ch.service)
	}
	if ch.scope != "" {
		q.Set("scope", ch.scope)
	}
	u.RawQuery = q.Encode()
	req, err := c.newRequest(ctx, http.MethodGet, u.String())
	if err != nil {
		return "", err
	}
	if t := c.factory.cfg.Token; t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry %s: token endpoint %s: %s", c.host, u.String(), resp.Status)
	}
	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("registry %s: token endpoint %s returned non-JSON: %v", c.host, u.String(), err)
	}
	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", fmt.Errorf("registry %s: token endpoint %s returned no token (body: %s)", c.host, u.String(), truncate(body, 200))
}

// truncate shortens s to at most max bytes, appending an ellipsis when trimmed.
func truncate(b []byte, max int) string {
	s := string(b)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
