package scm

import (
	"context"
	"fmt"
	"strings"

	gitea "code.gitea.io/sdk/gitea"
)

type Gitea struct {
	client *gitea.Client
}

func NewGitea(token, baseURL string) (*Gitea, error) {
	if baseURL == "" {
		baseURL = DefaultGiteaURL
	}

	var opts []gitea.ClientOption
	if token != "" {
		opts = append(opts, gitea.SetToken(token))
	}

	client, err := gitea.NewClient(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("new gitea client: %w", err)
	}
	return &Gitea{client: client}, nil
}

func (g *Gitea) Name() string {
	return "gitea"
}

func (g *Gitea) ListReleases(ctx context.Context, repo string, limit int) ([]Release, error) {
	_ = ctx

	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	if limit <= 0 {
		limit = 10
	}

	rels, _, err := g.client.ListReleases(owner, name, gitea.ListReleasesOptions{
		ListOptions: gitea.ListOptions{
			PageSize: limit,
		},
	})
	if err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(rels))
	for _, r := range rels {
		out = append(out, Release{
			Tag:         r.TagName,
			Name:        r.Title,
			PublishedAt: r.PublishedAt,
			URL:         r.HTMLURL,
			Prerelease:  r.IsPrerelease,
		})
	}
	return out, nil
}
