package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"relwatch/internal/store"
	"relwatch/internal/update"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/v1/projects/{id}", s.handleGetProject)
	mux.HandleFunc("DELETE /api/v1/projects/{id}", s.handleDeleteProject)
	mux.HandleFunc("GET /api/v1/projects/{id}/releases", s.handleProjectReleases)
	mux.HandleFunc("GET /api/v1/images", s.handleListImages)
	mux.HandleFunc("POST /api/v1/images", s.handleCreateImage)
	mux.HandleFunc("DELETE /api/v1/images/{id}", s.handleDeleteImage)
	mux.HandleFunc("GET /api/v1/images/{id}/digests", s.handleImageDigests)
	mux.HandleFunc("GET /api/v1/feeds", s.handleListFeeds)
	mux.HandleFunc("POST /api/v1/feeds", s.handleCreateFeed)
	mux.HandleFunc("GET /api/v1/feeds/{id}", s.handleGetFeed)
	mux.HandleFunc("DELETE /api/v1/feeds/{id}", s.handleDeleteFeed)
	mux.HandleFunc("GET /api/v1/feeds/{id}/releases", s.handleFeedReleases)
	return s.auth(mux)
}

type statusResponse struct {
	Version       string         `json:"version"`
	Projects      int            `json:"projects"`
	Images        int            `json:"images"`
	Feeds         int            `json:"feeds"`
	LastCheckedAt *time.Time     `json:"last_checked_at,omitempty"`
	Update        *update.Status `json:"update,omitempty"`
}

type projectSummary struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Provider         string     `json:"provider"`
	Repo             string     `json:"repo"`
	InstanceURL      string     `json:"instance_url,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	LatestTag        string     `json:"latest_tag,omitempty"`
	LatestReleasedAt *time.Time `json:"latest_released_at,omitempty"`
	LatestURL        string     `json:"latest_url,omitempty"`
}

type release struct {
	ID           int64      `json:"id"`
	ProjectID    int64      `json:"project_id"`
	Tag          string     `json:"tag"`
	Name         string     `json:"name,omitempty"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	URL          string     `json:"url,omitempty"`
	IsLatest     bool       `json:"is_latest"`
	IsPrerelease bool       `json:"is_prerelease"`
	CreatedAt    time.Time  `json:"created_at"`
}

type imageSummary struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Registry         string     `json:"registry"`
	Repo             string     `json:"repo"`
	CreatedAt        time.Time  `json:"created_at"`
	LatestTag        string     `json:"latest_tag,omitempty"`
	LatestDigest     string     `json:"latest_digest,omitempty"`
	LatestObservedAt *time.Time `json:"latest_observed_at,omitempty"`
}

type digest struct {
	ID         int64      `json:"id"`
	ImageID    int64      `json:"image_id"`
	Tag        string     `json:"tag"`
	Digest     string     `json:"digest"`
	IsLatest   bool       `json:"is_latest"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type feedSummary struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	URL              string     `json:"url"`
	CreatedAt        time.Time  `json:"created_at"`
	LatestTag        string     `json:"latest_tag,omitempty"`
	LatestReleasedAt *time.Time `json:"latest_released_at,omitempty"`
	LatestURL        string     `json:"latest_url,omitempty"`
}

type feedRelease struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feed_id"`
	Tag         string     `json:"tag"`
	Name        string     `json:"name,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	URL         string     `json:"url,omitempty"`
	IsLatest    bool       `json:"is_latest"`
	CreatedAt   time.Time  `json:"created_at"`
}

type projectInput struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Repo        string `json:"repo"`
	InstanceURL string `json:"instance_url"`
}

type imageInput struct {
	Name     string `json:"name"`
	Registry string `json:"registry"`
	Repo     string `json:"repo"`
}

type feedInput struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	projects, err := s.svc.ListProjectSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	images, err := s.svc.ListImageSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	feeds, err := s.svc.ListFeedSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	lastChecked, err := s.svc.LastChecked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := statusResponse{Version: s.version, Projects: len(projects), Images: len(images), Feeds: len(feeds), LastCheckedAt: lastChecked}
	if s.update != nil {
		resp.Update = s.update()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.svc.ListProjectSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProjects(summaries))
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sum, err := s.svc.GetProjectSummary(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProject(sum))
}

func (s *Server) handleProjectReleases(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.requireProject(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rels, err := s.svc.ListReleases(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toReleases(rels))
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.svc.ListImageSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toImages(summaries))
}

func (s *Server) handleImageDigests(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.svc.GetImageSummary(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	digests, err := s.svc.ListImageDigests(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDigests(digests))
}

func (s *Server) requireProject(id int64) error {
	_, err := s.svc.GetProjectSummary(id)
	return err
}

func (s *Server) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.svc.ListFeedSummaries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toFeeds(summaries))
}

func (s *Server) handleGetFeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sum, err := s.svc.GetFeedSummary(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toFeed(sum))
}

func (s *Server) handleFeedReleases(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.svc.GetFeedSummary(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rels, err := s.svc.ListFeedReleases(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toFeedReleases(rels))
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var in projectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := s.svc.AddProject(in.Name, in.Provider, in.Repo, in.InstanceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sum, err := s.svc.GetProjectSummary(p.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toProject(sum))
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteProject(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateImage(w http.ResponseWriter, r *http.Request) {
	var in imageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	img, err := s.svc.AddImage(in.Name, in.Registry, in.Repo)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sum, err := s.svc.GetImageSummary(img.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toImage(sum))
}

func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteImage(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateFeed(w http.ResponseWriter, r *http.Request) {
	var in feedInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	f, err := s.svc.AddFeed(in.Name, in.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sum, err := s.svc.GetFeedSummary(f.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toFeed(sum))
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteFeed(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func toProject(s store.ProjectSummary) projectSummary {
	return projectSummary{
		ID:               s.ID,
		Name:             s.Name,
		Provider:         s.Provider,
		Repo:             s.Repo,
		InstanceURL:      s.InstanceURL,
		CreatedAt:        s.CreatedAt,
		LatestTag:        s.LatestTag,
		LatestReleasedAt: s.LatestReleasedAt,
		LatestURL:        s.LatestURL,
	}
}

func toProjects(in []store.ProjectSummary) []projectSummary {
	out := make([]projectSummary, 0, len(in))
	for _, s := range in {
		out = append(out, toProject(s))
	}
	return out
}

func toRelease(r store.Release) release {
	return release{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Tag:          r.Tag,
		Name:         r.Name,
		PublishedAt:  r.PublishedAt,
		URL:          r.URL,
		IsLatest:     r.IsLatest,
		IsPrerelease: r.IsPrerelease,
		CreatedAt:    r.CreatedAt,
	}
}

func toReleases(in []store.Release) []release {
	out := make([]release, 0, len(in))
	for _, r := range in {
		out = append(out, toRelease(r))
	}
	return out
}

func toImage(s store.ImageSummary) imageSummary {
	return imageSummary{
		ID:               s.ID,
		Name:             s.Name,
		Registry:         s.Registry,
		Repo:             s.Repo,
		CreatedAt:        s.CreatedAt,
		LatestTag:        s.LatestTag,
		LatestDigest:     s.LatestDigest,
		LatestObservedAt: s.LatestObservedAt,
	}
}

func toImages(in []store.ImageSummary) []imageSummary {
	out := make([]imageSummary, 0, len(in))
	for _, s := range in {
		out = append(out, toImage(s))
	}
	return out
}

func toDigest(d store.ImageDigest) digest {
	return digest{
		ID:         d.ID,
		ImageID:    d.ImageID,
		Tag:        d.Tag,
		Digest:     d.Digest,
		IsLatest:   d.IsLatest,
		ObservedAt: d.ObservedAt,
		CreatedAt:  d.CreatedAt,
	}
}

func toDigests(in []store.ImageDigest) []digest {
	out := make([]digest, 0, len(in))
	for _, d := range in {
		out = append(out, toDigest(d))
	}
	return out
}

func toFeed(s store.FeedSummary) feedSummary {
	return feedSummary{
		ID:               s.ID,
		Name:             s.Name,
		URL:              s.URL,
		CreatedAt:        s.CreatedAt,
		LatestTag:        s.LatestTag,
		LatestReleasedAt: s.LatestReleasedAt,
		LatestURL:        s.LatestURL,
	}
}

func toFeeds(in []store.FeedSummary) []feedSummary {
	out := make([]feedSummary, 0, len(in))
	for _, s := range in {
		out = append(out, toFeed(s))
	}
	return out
}

func toFeedRelease(r store.FeedRelease) feedRelease {
	return feedRelease{
		ID:          r.ID,
		FeedID:      r.FeedID,
		Tag:         r.Tag,
		Name:        r.Name,
		PublishedAt: r.PublishedAt,
		URL:         r.URL,
		IsLatest:    r.IsLatest,
		CreatedAt:   r.CreatedAt,
	}
}

func toFeedReleases(in []store.FeedRelease) []feedRelease {
	out := make([]feedRelease, 0, len(in))
	for _, r := range in {
		out = append(out, toFeedRelease(r))
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}
