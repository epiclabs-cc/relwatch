package scm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Release struct {
	Tag         string
	Name        string
	PublishedAt time.Time
	URL         string
	Prerelease  bool
}

type Provider interface {
	Name() string
	ListReleases(ctx context.Context, repo string, limit int) ([]Release, error)
}

type Config struct {
	GitHubToken string
	GitLabToken string
	GitLabURL   string
	GiteaToken  string
	GiteaURL    string
}

type Factory struct {
	cfg Config
}

func NewFactory(cfg Config) *Factory {
	return &Factory{cfg: cfg}
}

const (
	DefaultGitLabURL = "https://gitlab.com/api/v4"
	DefaultGiteaURL  = "https://gitea.com"
)

// For returns a provider for the given name. instanceURL overrides the global
// provider URL (GitLab/Gitea) for a single project when non-empty.
func (f *Factory) For(name, instanceURL string) (Provider, error) {
	switch strings.ToLower(name) {
	case "github":
		return NewGitHub(f.cfg.GitHubToken), nil
	case "gitlab":
		url := f.cfg.GitLabURL
		if instanceURL != "" {
			url = instanceURL
		}
		return NewGitLab(f.cfg.GitLabToken, url)
	case "gitea":
		url := f.cfg.GiteaURL
		if instanceURL != "" {
			url = instanceURL
		}
		return NewGitea(f.cfg.GiteaToken, url)
	case "rss":
		return NewRSS(), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
}
