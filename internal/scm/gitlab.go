package scm

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLab struct {
	client *gitlab.Client
	base   string
}

func NewGitLab(token, baseURL string) (*GitLab, error) {
	if baseURL == "" {
		baseURL = DefaultGitLabURL
	}

	var opts []gitlab.ClientOptionFunc
	if baseURL != DefaultGitLabURL {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}

	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("new gitlab client: %w", err)
	}
	return &GitLab{client: client, base: strings.TrimSuffix(baseURL, "/")}, nil
}

func (g *GitLab) Name() string {
	return "gitlab"
}

func (g *GitLab) ListReleases(ctx context.Context, repo string, limit int) ([]Release, error) {
	if repo == "" {
		return nil, fmt.Errorf("invalid repo %q, expected a project path", repo)
	}
	if limit <= 0 {
		limit = 10
	}

	rels, _, err := g.client.Releases.ListReleases(repo, &gitlab.ListReleasesOptions{
		ListOptions: gitlab.ListOptions{PerPage: int64(limit)},
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(rels))
	for _, r := range rels {
		rel := Release{
			Tag:  r.TagName,
			Name: r.Name,
			URL:  g.releaseURL(repo, r.TagName),
		}
		if r.ReleasedAt != nil {
			rel.PublishedAt = *r.ReleasedAt
		}
		out = append(out, rel)
	}
	return out, nil
}

func (g *GitLab) releaseURL(path, tag string) string {
	if tag == "" {
		return ""
	}
	host := strings.TrimSuffix(g.base, "/api/v4")
	return fmt.Sprintf("%s/%s/-/releases/%s", host, path, url.PathEscape(tag))
}
