package service

import (
	"errors"
	"fmt"
	"strings"

	"relwatch/internal/store"
)

func (s *Service) AddProject(name, provider, repo, instanceURL string) (store.Project, error) {
	name = strings.TrimSpace(name)
	provider = strings.ToLower(strings.TrimSpace(provider))
	repo = strings.TrimSpace(repo)
	instanceURL = strings.TrimSpace(instanceURL)
	if name == "" {
		return store.Project{}, errors.New("name is required")
	}
	switch provider {
	case "github", "gitlab", "gitea":
	default:
		return store.Project{}, fmt.Errorf("unsupported provider %q", provider)
	}
	if repo == "" {
		return store.Project{}, errors.New("repository is required")
	}
	return s.st.AddProject(name, provider, repo, instanceURL)
}

func (s *Service) DeleteProject(id int64) error {
	return s.st.DeleteProject(id)
}

func (s *Service) AddImage(name, registry, repo string) (store.ContainerImage, error) {
	name = strings.TrimSpace(name)
	registry = strings.TrimSpace(registry)
	repo = strings.TrimSpace(repo)
	if name == "" {
		return store.ContainerImage{}, errors.New("name is required")
	}
	if registry == "" {
		return store.ContainerImage{}, errors.New("registry is required")
	}
	if repo == "" {
		return store.ContainerImage{}, errors.New("repository is required")
	}
	return s.st.AddContainerImage(name, registry, repo)
}

func (s *Service) DeleteImage(id int64) error {
	return s.st.DeleteContainerImage(id)
}

func (s *Service) AddFeed(name, url string) (store.Feed, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return store.Feed{}, errors.New("name is required")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return store.Feed{}, errors.New("feed URL must start with http:// or https://")
	}
	return s.st.AddFeed(name, url)
}

func (s *Service) DeleteFeed(id int64) error {
	return s.st.DeleteFeed(id)
}
