// Package web renders the dashboard served by `relwatch serve` at the HTTP
// listener's root, alongside the JSON API. It is server-rendered with
// html/template and reads and writes through the service layer only.
package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"relwatch/internal/service"
	"relwatch/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Web struct {
	svc     *service.Service
	version string
	pages   map[string]*template.Template
}

func New(svc *service.Service, version string) (*Web, error) {
	base, err := template.New("").Funcs(funcMap()).ParseFS(templatesFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}

	w := &Web{svc: svc, version: version, pages: map[string]*template.Template{}}
	for _, name := range []string{"index", "projects", "project", "images", "image", "feeds", "feed", "login"} {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if t, err = t.ParseFS(templatesFS, "templates/"+name+".html"); err != nil {
			return nil, err
		}
		w.pages[name] = t
	}
	return w, nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"date": func(t *time.Time) template.HTML {
			if t == nil {
				return ""
			}
			u := t.UTC()
			return template.HTML(fmt.Sprintf(`<time datetime="%s">%s</time>`,
				u.Format(time.RFC3339), u.Format("2006-01-02 15:04")))
		},
		"short": func(s string) string {
			if len(s) > 16 {
				return s[:16] + "…"
			}
			return s
		},
		"initial": func(s string) string {
			for _, r := range s {
				return strings.ToUpper(string(r))
			}
			return "?"
		},
	}
}

type sortCol struct {
	Key   string
	Label string
	Date  bool
}

var projectCols = []sortCol{
	{"name", "Name", false},
	{"provider", "Provider", false},
	{"repo", "Repository", false},
	{"instance", "Instance", false},
	{"latest", "Latest release", false},
	{"released", "Released", true},
}

var imageCols = []sortCol{
	{"name", "Name", false},
	{"registry", "Registry", false},
	{"repo", "Repository", false},
	{"latest", "Latest tag", false},
	{"digest", "Digest", false},
	{"released", "Released", true},
}

var feedCols = []sortCol{
	{"name", "Name", false},
	{"url", "URL", false},
	{"latest", "Latest release", false},
	{"released", "Released", true},
}

// tableSort parses the sort/dir query params, defaulting to newest-first by the
// "released" column and ignoring unknown columns.
func tableSort(r *http.Request, cols []sortCol) (string, string) {
	key := r.URL.Query().Get("sort")
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "desc"
	}
	if !hasCol(cols, key) {
		key = "released"
		dir = "desc"
	}
	return key, dir
}

func hasCol(cols []sortCol, key string) bool {
	for _, c := range cols {
		if c.Key == key {
			return true
		}
	}
	return false
}

// sortHeaders builds the clickable header cells for a list page. The active
// column carries an arrow and toggles direction on click; date columns default
// to descending, text columns to ascending.
func sortHeaders(path, key, dir string, cols []sortCol) []header {
	headers := make([]header, 0, len(cols))
	for _, c := range cols {
		h := header{Label: c.Label}
		if c.Key == key {
			h.Arrow = "▼"
			next := "asc"
			if dir == "asc" {
				h.Arrow = "▲"
				next = "desc"
			}
			h.Href = path + "?sort=" + c.Key + "&dir=" + next
		} else {
			d := "asc"
			if c.Date {
				d = "desc"
			}
			h.Href = path + "?sort=" + c.Key + "&dir=" + d
		}
		headers = append(headers, h)
	}
	return headers
}

func timeLess(a, b *time.Time, desc bool) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if desc {
		return a.After(*b)
	}
	return a.Before(*b)
}

func sortProjects(list []store.ProjectSummary, key, dir string) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if key == "released" {
			return timeLess(a.LatestReleasedAt, b.LatestReleasedAt, dir == "desc")
		}
		var cmp int
		switch key {
		case "name":
			cmp = strings.Compare(a.Name, b.Name)
		case "provider":
			cmp = strings.Compare(a.Provider, b.Provider)
		case "repo":
			cmp = strings.Compare(a.Repo, b.Repo)
		case "instance":
			cmp = strings.Compare(a.InstanceURL, b.InstanceURL)
		case "latest":
			cmp = strings.Compare(a.LatestTag, b.LatestTag)
		default:
			cmp = strings.Compare(a.Name, b.Name)
		}
		if dir == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
}

func sortImages(list []store.ImageSummary, key, dir string) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if key == "released" {
			return timeLess(a.LatestObservedAt, b.LatestObservedAt, dir == "desc")
		}
		var cmp int
		switch key {
		case "name":
			cmp = strings.Compare(a.Name, b.Name)
		case "registry":
			cmp = strings.Compare(a.Registry, b.Registry)
		case "repo":
			cmp = strings.Compare(a.Repo, b.Repo)
		case "latest":
			cmp = strings.Compare(a.LatestTag, b.LatestTag)
		case "digest":
			cmp = strings.Compare(a.LatestDigest, b.LatestDigest)
		default:
			cmp = strings.Compare(a.Name, b.Name)
		}
		if dir == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
}

func sortFeeds(list []store.FeedSummary, key, dir string) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if key == "released" {
			return timeLess(a.LatestReleasedAt, b.LatestReleasedAt, dir == "desc")
		}
		var cmp int
		switch key {
		case "name":
			cmp = strings.Compare(a.Name, b.Name)
		case "url":
			cmp = strings.Compare(a.URL, b.URL)
		case "latest":
			cmp = strings.Compare(a.LatestTag, b.LatestTag)
		default:
			cmp = strings.Compare(a.Name, b.Name)
		}
		if dir == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
}

func (w *Web) Routes() http.Handler {
	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", w.handleLoginForm)
	mux.HandleFunc("POST /login", w.handleLogin)
	mux.HandleFunc("POST /logout", w.handleLogout)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticRoot)))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /{$}", w.handleIndex)
	protected.HandleFunc("GET /projects", w.handleProjects)
	protected.HandleFunc("POST /projects", w.handleAddProject)
	protected.HandleFunc("GET /projects/{id}", w.handleProject)
	protected.HandleFunc("POST /projects/{id}/delete", w.handleDeleteProject)
	protected.HandleFunc("GET /images", w.handleImages)
	protected.HandleFunc("POST /images", w.handleAddImage)
	protected.HandleFunc("GET /images/{id}", w.handleImage)
	protected.HandleFunc("POST /images/{id}/delete", w.handleDeleteImage)
	protected.HandleFunc("GET /feeds", w.handleFeeds)
	protected.HandleFunc("POST /feeds", w.handleAddFeed)
	protected.HandleFunc("GET /feeds/{id}", w.handleFeed)
	protected.HandleFunc("POST /feeds/{id}/delete", w.handleDeleteFeed)
	mux.Handle("/", w.requireAuth(protected))
	return mux
}

type baseData struct {
	Title   string
	Version string
	CSRF    string
	Email   string
}

type indexData struct {
	baseData
	ProjectCount int
	ImageCount   int
	FeedCount    int
	LastChecked  *time.Time
}

type loginData struct {
	baseData
	Error string
}

// tableData holds the sortable-column state for a list page.
type tableData struct {
	Sort    string
	Dir     string
	Headers []header
}

type header struct {
	Label string
	Href  string
	Arrow string
}

type projectsData struct {
	baseData
	tableData
	Projects []store.ProjectSummary
	Error    string
}

type projectData struct {
	baseData
	Project  store.ProjectSummary
	Releases []store.Release
}

type imagesData struct {
	baseData
	tableData
	Images []store.ImageSummary
	Error  string
}

type imageData struct {
	baseData
	Image   store.ImageSummary
	Digests []store.ImageDigest
}

type feedsData struct {
	baseData
	tableData
	Feeds []store.FeedSummary
	Error string
}

type feedData struct {
	baseData
	Feed     store.FeedSummary
	Releases []store.FeedRelease
}

func (w *Web) render(wr http.ResponseWriter, name string, data any) {
	if err := w.pages[name].ExecuteTemplate(wr, "layout", data); err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
	}
}

type ctxKey int

const emailCtxKey ctxKey = iota

// base builds the shared page data for authenticated pages, including the
// signed-in user's email (for the header) and their CSRF token (for forms).
func (w *Web) base(r *http.Request, title string) baseData {
	return baseData{
		Title:   title,
		Version: w.version,
		CSRF:    w.csrf(r),
		Email:   currentEmail(r),
	}
}

func currentEmail(r *http.Request) string {
	if e, ok := r.Context().Value(emailCtxKey).(string); ok {
		return e
	}
	return ""
}

func (w *Web) handleIndex(wr http.ResponseWriter, r *http.Request) {
	projects, err := w.svc.ListProjectSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	images, err := w.svc.ListImageSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	feeds, err := w.svc.ListFeedSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	lastChecked, err := w.svc.LastChecked()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	w.render(wr, "index", indexData{
		baseData:     w.base(r, "Overview"),
		ProjectCount: len(projects),
		ImageCount:   len(images),
		FeedCount:    len(feeds),
		LastChecked:  lastChecked,
	})
}

func (w *Web) renderProjects(wr http.ResponseWriter, r *http.Request, errMsg string) {
	projects, err := w.svc.ListProjectSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	sort, dir := tableSort(r, projectCols)
	sortProjects(projects, sort, dir)
	w.render(wr, "projects", projectsData{
		baseData:  w.base(r, "Projects"),
		tableData: tableData{Sort: sort, Dir: dir, Headers: sortHeaders("/projects", sort, dir, projectCols)},
		Projects:  projects,
		Error:     errMsg,
	})
}

func (w *Web) handleProjects(wr http.ResponseWriter, r *http.Request) {
	w.renderProjects(wr, r, "")
}

func (w *Web) handleAddProject(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	if _, err := w.svc.AddProject(r.FormValue("name"), r.FormValue("provider"), r.FormValue("repo"), r.FormValue("instance_url")); err != nil {
		w.renderProjects(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/projects", http.StatusSeeOther)
}

func (w *Web) handleDeleteProject(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	if err := w.svc.DeleteProject(id); err != nil {
		w.renderProjects(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/projects", http.StatusSeeOther)
}

func (w *Web) handleProject(wr http.ResponseWriter, r *http.Request) {
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	project, err := w.svc.GetProjectSummary(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(wr, r)
		return
	}
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	releases, err := w.svc.ListReleases(id)
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	w.render(wr, "project", projectData{
		baseData: w.base(r, project.Name),
		Project:  project,
		Releases: releases,
	})
}

func (w *Web) renderImages(wr http.ResponseWriter, r *http.Request, errMsg string) {
	images, err := w.svc.ListImageSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	sort, dir := tableSort(r, imageCols)
	sortImages(images, sort, dir)
	w.render(wr, "images", imagesData{
		baseData:  w.base(r, "Images"),
		tableData: tableData{Sort: sort, Dir: dir, Headers: sortHeaders("/images", sort, dir, imageCols)},
		Images:    images,
		Error:     errMsg,
	})
}

func (w *Web) handleImages(wr http.ResponseWriter, r *http.Request) {
	w.renderImages(wr, r, "")
}

func (w *Web) handleAddImage(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	if _, err := w.svc.AddImage(r.FormValue("name"), r.FormValue("registry"), r.FormValue("repo")); err != nil {
		w.renderImages(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/images", http.StatusSeeOther)
}

func (w *Web) handleDeleteImage(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	if err := w.svc.DeleteImage(id); err != nil {
		w.renderImages(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/images", http.StatusSeeOther)
}

func (w *Web) handleImage(wr http.ResponseWriter, r *http.Request) {
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	image, err := w.svc.GetImageSummary(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(wr, r)
		return
	}
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	digests, err := w.svc.ListImageDigests(id)
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	w.render(wr, "image", imageData{
		baseData: w.base(r, image.Name),
		Image:    image,
		Digests:  digests,
	})
}

func (w *Web) renderFeeds(wr http.ResponseWriter, r *http.Request, errMsg string) {
	feeds, err := w.svc.ListFeedSummaries()
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	sort, dir := tableSort(r, feedCols)
	sortFeeds(feeds, sort, dir)
	w.render(wr, "feeds", feedsData{
		baseData:  w.base(r, "Feeds"),
		tableData: tableData{Sort: sort, Dir: dir, Headers: sortHeaders("/feeds", sort, dir, feedCols)},
		Feeds:     feeds,
		Error:     errMsg,
	})
}

func (w *Web) handleFeeds(wr http.ResponseWriter, r *http.Request) {
	w.renderFeeds(wr, r, "")
}

func (w *Web) handleAddFeed(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	if _, err := w.svc.AddFeed(r.FormValue("name"), r.FormValue("url")); err != nil {
		w.renderFeeds(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/feeds", http.StatusSeeOther)
}

func (w *Web) handleDeleteFeed(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	if err := w.svc.DeleteFeed(id); err != nil {
		w.renderFeeds(wr, r, err.Error())
		return
	}
	http.Redirect(wr, r, "/feeds", http.StatusSeeOther)
}

func (w *Web) handleFeed(wr http.ResponseWriter, r *http.Request) {
	id, ok := pathID(wr, r)
	if !ok {
		return
	}
	feed, err := w.svc.GetFeedSummary(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(wr, r)
		return
	}
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	releases, err := w.svc.ListFeedReleases(id)
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	w.render(wr, "feed", feedData{
		baseData: w.base(r, feed.Name),
		Feed:     feed,
		Releases: releases,
	})
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

const (
	sessionCookieName = "relwatch_session"
	sessionMaxAgeSec  = 30 * 24 * 60 * 60
)

func (w *Web) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(wr, r, "/login", http.StatusSeeOther)
			return
		}
		u, err := w.svc.UserForSession(cookie.Value)
		if err != nil {
			http.Redirect(wr, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), emailCtxKey, u.Email)
		next.ServeHTTP(wr, r.WithContext(ctx))
	})
}

func (w *Web) csrf(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	t, err := w.svc.CSRFToken(cookie.Value)
	if err != nil {
		return ""
	}
	return t
}

func (w *Web) validCSRF(wr http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		http.Error(wr, "invalid csrf token", http.StatusForbidden)
		return false
	}
	want, err := w.svc.CSRFToken(cookie.Value)
	if err != nil || subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(want)) != 1 {
		http.Error(wr, "invalid csrf token", http.StatusForbidden)
		return false
	}
	return true
}

func (w *Web) handleLoginForm(wr http.ResponseWriter, r *http.Request) {
	w.render(wr, "login", loginData{baseData: baseData{Title: "Sign in", Version: w.version}})
}

func (w *Web) handleLogin(wr http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	u, err := w.svc.Authenticate(r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		w.render(wr, "login", loginData{
			baseData: baseData{Title: "Sign in", Version: w.version},
			Error:    "Invalid email or password.",
		})
		return
	}
	token, err := w.svc.CreateSession(u.ID)
	if err != nil {
		http.Error(wr, "internal error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(wr, token)
	http.Redirect(wr, r, "/", http.StatusSeeOther)
}

func (w *Web) handleLogout(wr http.ResponseWriter, r *http.Request) {
	if !w.validCSRF(wr, r) {
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = w.svc.DeleteSession(cookie.Value)
	}
	clearSessionCookie(wr)
	http.Redirect(wr, r, "/login", http.StatusSeeOther)
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAgeSec,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
