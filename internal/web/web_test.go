package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"relwatch/internal/service"
	"relwatch/internal/store"
)

func seedStore(t *testing.T, st *store.Store) {
	t.Helper()
	pub := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	proj, err := st.AddProject("svelte", "github", "sveltejs/svelte", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertRelease(store.Release{ProjectID: proj.ID, Tag: "svelte@5.0.0", PublishedAt: &pub, URL: "https://example.test/r"}); err != nil {
		t.Fatal(err)
	}

	img, err := st.AddContainerImage("caddy", "docker.io", "library/caddy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertImageDigest(store.ImageDigest{ImageID: img.ID, Tag: "2.8", Digest: "sha256:bbb", ObservedAt: &pub}); err != nil {
		t.Fatal(err)
	}

	fd, err := st.AddFeed("alpine", "https://example.test/atom.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertFeedRelease(store.FeedRelease{FeedID: fd.ID, Tag: "v1", Name: "v1", PublishedAt: &pub}); err != nil {
		t.Fatal(err)
	}
}

func newTestWeb(t *testing.T) (*Web, *store.Store, *httptest.Server) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	seedStore(t, st)

	w, err := New(service.New(st), "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	t.Cleanup(srv.Close)
	return w, st, srv
}

func doAuthed(t *testing.T, method, url, token string, form url.Values) *http.Response {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPagesRequireAuth(t *testing.T) {
	_, st, srv := newTestWeb(t)

	u, err := service.New(st).CreateUser("admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.New(st).CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		want string
	}{
		{"/", "Overview"},
		{"/projects", "svelte"},
		{"/projects/1", "svelte@5.0.0"},
		{"/images", "caddy"},
		{"/images/1", "sha256:bbb"},
		{"/feeds", "alpine"},
		{"/feeds/1", "v1"},
	}
	for _, tc := range cases {
		t.Run("authed"+tc.path, func(t *testing.T) {
			resp := doAuthed(t, http.MethodGet, srv.URL+tc.path, token, nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tc.path, resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tc.want) {
				t.Fatalf("GET %s missing %q", tc.path, tc.want)
			}
		})
	}

	t.Run("unauthed", func(t *testing.T) {
		resp := doAuthed(t, http.MethodGet, srv.URL+"/", "", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("unauthenticated / = %d, want 303", resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Fatalf("redirect = %q, want /login", loc)
		}
	})
}

func TestLoginAndLogout(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	svc := service.New(st)
	if _, err := svc.CreateUser("admin@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	w, err := New(svc, "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Wrong password re-renders the form (200) with an error.
	resp, err := client.PostForm(srv.URL+"/login", url.Values{"email": {"admin@example.com"}, "password": {"wrong"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("wrong-password login = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Correct credentials set a session cookie and redirect home.
	resp, err = client.PostForm(srv.URL+"/login", url.Values{"email": {"admin@example.com"}, "password": {"password123"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login = %d, want 303", resp.StatusCode)
	}
	var token string
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("no session cookie set on login")
	}

	// The token grants access to a protected page.
	authed := doAuthed(t, http.MethodGet, srv.URL+"/", token, nil)
	authed.Body.Close()
	if authed.StatusCode != http.StatusOK {
		t.Fatalf("authed / = %d, want 200", authed.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	svc := service.New(st)
	if _, err := svc.CreateUser("admin@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	token, err := svc.CreateSession(1)
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := svc.CSRFToken(token)
	if err != nil {
		t.Fatal(err)
	}

	w, err := New(svc, "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	defer srv.Close()

	resp := doAuthed(t, http.MethodPost, srv.URL+"/logout", token, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("logout without csrf = %d, want 403", resp.StatusCode)
	}

	resp = doAuthed(t, http.MethodPost, srv.URL+"/logout", token, url.Values{"csrf": {csrf}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout = %d, want 303", resp.StatusCode)
	}

	authed := doAuthed(t, http.MethodGet, srv.URL+"/", token, nil)
	authed.Body.Close()
	if authed.StatusCode != http.StatusSeeOther {
		t.Fatalf("after logout / = %d, want 303 (redirect to login)", authed.StatusCode)
	}
}

func TestCRUD(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	svc := service.New(st)
	if _, err := svc.CreateUser("admin@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	token, err := svc.CreateSession(1)
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := svc.CSRFToken(token)
	if err != nil {
		t.Fatal(err)
	}

	w, err := New(svc, "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	defer srv.Close()

	resp := doAuthed(t, http.MethodPost, srv.URL+"/projects", token, url.Values{
		"csrf": {csrf}, "name": {"svelte"}, "provider": {"github"}, "repo": {"sveltejs/svelte"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("add project = %d, want 303", resp.StatusCode)
	}

	resp = doAuthed(t, http.MethodGet, srv.URL+"/projects", token, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "svelte") {
		t.Fatal("added project not shown in list")
	}

	resp = doAuthed(t, http.MethodPost, srv.URL+"/projects/1/delete", token, url.Values{"csrf": {csrf}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete project = %d, want 303", resp.StatusCode)
	}

	resp = doAuthed(t, http.MethodGet, srv.URL+"/projects", token, nil)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), "svelte") {
		t.Fatal("deleted project still shown")
	}
}

func TestCSRFRejected(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	svc := service.New(st)
	if _, err := svc.CreateUser("admin@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	token, err := svc.CreateSession(1)
	if err != nil {
		t.Fatal(err)
	}

	w, err := New(svc, "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	defer srv.Close()

	resp := doAuthed(t, http.MethodPost, srv.URL+"/projects", token, url.Values{
		"name": {"svelte"}, "provider": {"github"}, "repo": {"sveltejs/svelte"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing csrf = %d, want 403", resp.StatusCode)
	}

	resp = doAuthed(t, http.MethodPost, srv.URL+"/projects", token, url.Values{
		"csrf": {"bogus"}, "name": {"svelte"}, "provider": {"github"}, "repo": {"sveltejs/svelte"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong csrf = %d, want 403", resp.StatusCode)
	}
}

func TestHeaderUserMenu(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	svc := service.New(st)
	if _, err := svc.CreateUser("admin@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	token, err := svc.CreateSession(1)
	if err != nil {
		t.Fatal(err)
	}

	w, err := New(svc, "dev")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(w.Routes())
	defer srv.Close()

	resp := doAuthed(t, http.MethodGet, srv.URL+"/", token, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "admin@example.com") {
		t.Fatal("header missing user email")
	}
	if !strings.Contains(string(body), "Log out") {
		t.Fatal("header missing log out")
	}
	if !strings.Contains(string(body), `action="/logout"`) {
		t.Fatal("header missing logout form action")
	}

	resp = doAuthed(t, http.MethodGet, srv.URL+"/login", "", nil)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), "Log out") {
		t.Fatal("login page should not show log out")
	}
}

func TestNotFoundBadIDAndStatic(t *testing.T) {
	_, st, srv := newTestWeb(t)
	u, err := service.New(st).CreateUser("admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.New(st).CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]int{
		"/projects/999": http.StatusNotFound,
		"/projects/abc": http.StatusBadRequest,
	} {
		resp := doAuthed(t, http.MethodGet, srv.URL+path, token, nil)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
	}

	// Static assets are public (no session required).
	resp, err := http.Get(srv.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("style.css = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("style.css content-type = %q, want text/css", ct)
	}
}
