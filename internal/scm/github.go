package scm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v69/github"
)

type GitHub struct {
	client *github.Client
}

func NewGitHub(token string) *GitHub {
	client := github.NewClient(nil)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	return &GitHub{client: client}
}

func (g *GitHub) Name() string {
	return "github"
}

func (g *GitHub) ListReleases(ctx context.Context, repo string, limit int) ([]Release, error) {
	if limit <= 0 {
		limit = 10
	}
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("invalid repo %q, expected owner/name", repo)
	}

	rels, _, err := g.client.Repositories.ListReleases(ctx, owner, name, &github.ListOptions{PerPage: limit})
	if err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(rels))
	for _, r := range rels {
		rel := Release{
			Tag:        r.GetTagName(),
			Name:       r.GetName(),
			URL:        r.GetHTMLURL(),
			Prerelease: r.GetPrerelease(),
		}
		if r.PublishedAt != nil {
			rel.PublishedAt = r.PublishedAt.Time
		}
		out = append(out, rel)
	}
	return out, nil
}
