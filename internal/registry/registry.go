package registry

import (
	"context"
	"net/http"
	"time"
)

// Image is a single tag of a container image repository resolved to its OCI
// digest. A moved tag (digest change) is treated as a new release.
type Image struct {
	Repo        string
	Tag         string
	Digest      string
	URL         string
	PublishedAt time.Time
}

// Client is a generic OCI Distribution v2 client. ghcr.io, gcr.io, quay.io,
// registry.gitlab.com, docker.io and self-hosted OCI registries all speak this
// one protocol, so a single implementation covers every registry (Q1).
type Client interface {
	Name() string
	ListImages(ctx context.Context, repo string) ([]Image, error)
}

type Config struct {
	Token      string // optional bearer token for private registries
	Scheme     string // http or https (default https)
	HttpClient *http.Client
}

type Factory struct {
	cfg        Config
	httpClient *http.Client
}

func NewFactory(cfg Config) *Factory {
	if cfg.Scheme == "" {
		cfg.Scheme = "https"
	}
	hc := cfg.HttpClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Factory{cfg: cfg, httpClient: hc}
}

// For returns a client bound to the given registry host.
func (f *Factory) For(host string) Client {
	return &ociClient{factory: f, host: host, http: f.httpClient}
}
