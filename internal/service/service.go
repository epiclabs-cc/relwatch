// Package service is the domain layer between persistence and the HTTP
// surfaces (the JSON API and the web UI). It holds business rules and, later,
// authentication and authorization; the handlers stay thin.
package service

import (
	"relwatch/internal/store"
)

// Service wraps the store and exposes domain operations. Reads are pass-through
// today; writes and auth checks land here in later phases so the API, the web
// UI, and (optionally) the CLI share one implementation.
type Service struct {
	st *store.Store
}

func New(st *store.Store) *Service {
	return &Service{st: st}
}

func (s *Service) ListProjectSummaries() ([]store.ProjectSummary, error) {
	return s.st.ListProjectSummaries()
}

func (s *Service) GetProjectSummary(id int64) (store.ProjectSummary, error) {
	return s.st.GetProjectSummary(id)
}

func (s *Service) ListReleases(projectID int64) ([]store.Release, error) {
	return s.st.ListReleases(projectID)
}

func (s *Service) ListImageSummaries() ([]store.ImageSummary, error) {
	return s.st.ListImageSummaries()
}

func (s *Service) GetImageSummary(id int64) (store.ImageSummary, error) {
	return s.st.GetImageSummary(id)
}

func (s *Service) ListImageDigests(imageID int64) ([]store.ImageDigest, error) {
	return s.st.ListImageDigests(imageID)
}

func (s *Service) ListFeedSummaries() ([]store.FeedSummary, error) {
	return s.st.ListFeedSummaries()
}

func (s *Service) GetFeedSummary(id int64) (store.FeedSummary, error) {
	return s.st.GetFeedSummary(id)
}

func (s *Service) ListFeedReleases(feedID int64) ([]store.FeedRelease, error) {
	return s.st.ListFeedReleases(feedID)
}
