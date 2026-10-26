package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"relwatch/internal/service"
	"relwatch/internal/store"
	"relwatch/internal/update"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func getJSON(t *testing.T, url string, out any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func seed(t *testing.T, st *store.Store) {
	t.Helper()
	pub := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	p, err := st.AddProject("svelte", "github", "sveltejs/svelte", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertRelease(store.Release{ProjectID: p.ID, Tag: "svelte@5.0.0", PublishedAt: &pub, URL: "https://example.test/r"}); err != nil {
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
	if _, err := st.UpsertFeedRelease(store.FeedRelease{FeedID: fd.ID, Tag: "v1", Name: "v1", PublishedAt: &pub, URL: "https://example.test/f"}); err != nil {
		t.Fatal(err)
	}
}

func TestReadEndpoints(t *testing.T) {
	st := newTestStore(t)
	seed(t, st)
	srv := httptest.NewServer(New(service.New(st), "v9.9.9", "").Handler())
	defer srv.Close()

	t.Run("status", func(t *testing.T) {
		var got statusResponse
		if code := getJSON(t, srv.URL+"/api/v1/status", &got); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if got.Version != "v9.9.9" || got.Projects != 1 || got.Images != 1 || got.Feeds != 1 {
			t.Fatalf("unexpected status: %+v", got)
		}
	})

	t.Run("projects", func(t *testing.T) {
		var got []projectSummary
		if code := getJSON(t, srv.URL+"/api/v1/projects", &got); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(got) != 1 || got[0].Name != "svelte" || got[0].LatestTag != "svelte@5.0.0" {
			t.Fatalf("unexpected projects: %+v", got)
		}
	})

	t.Run("project and releases", func(t *testing.T) {
		var proj projectSummary
		if code := getJSON(t, srv.URL+"/api/v1/projects/1", &proj); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if proj.ID != 1 {
			t.Fatalf("unexpected project: %+v", proj)
		}
		var rels []release
		if code := getJSON(t, srv.URL+"/api/v1/projects/1/releases", &rels); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(rels) != 1 || rels[0].Tag != "svelte@5.0.0" {
			t.Fatalf("unexpected releases: %+v", rels)
		}
	})

	t.Run("images and digests", func(t *testing.T) {
		var imgs []imageSummary
		if code := getJSON(t, srv.URL+"/api/v1/images", &imgs); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(imgs) != 1 || imgs[0].LatestDigest != "sha256:bbb" {
			t.Fatalf("unexpected images: %+v", imgs)
		}
		var digs []digest
		if code := getJSON(t, srv.URL+"/api/v1/images/1/digests", &digs); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(digs) != 1 || digs[0].Tag != "2.8" {
			t.Fatalf("unexpected digests: %+v", digs)
		}
	})

	t.Run("feeds and releases", func(t *testing.T) {
		var fds []feedSummary
		if code := getJSON(t, srv.URL+"/api/v1/feeds", &fds); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(fds) != 1 || fds[0].Name != "alpine" || fds[0].LatestTag != "v1" {
			t.Fatalf("unexpected feeds: %+v", fds)
		}
		var fd feedSummary
		if code := getJSON(t, srv.URL+"/api/v1/feeds/1", &fd); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if fd.ID != 1 {
			t.Fatalf("unexpected feed: %+v", fd)
		}
		var rels []feedRelease
		if code := getJSON(t, srv.URL+"/api/v1/feeds/1/releases", &rels); code != 200 {
			t.Fatalf("status code %d", code)
		}
		if len(rels) != 1 || rels[0].Tag != "v1" {
			t.Fatalf("unexpected feed releases: %+v", rels)
		}
	})
}

func TestReadEndpointsErrors(t *testing.T) {
	st := newTestStore(t)
	seed(t, st)
	srv := httptest.NewServer(New(service.New(st), "dev", "").Handler())
	defer srv.Close()

	cases := []struct {
		name, path string
		want       int
	}{
		{"unknown id", "/api/v1/projects/999", http.StatusNotFound},
		{"unknown image", "/api/v1/images/999/digests", http.StatusNotFound},
		{"unknown feed", "/api/v1/feeds/999", http.StatusNotFound},
		{"bad id", "/api/v1/projects/abc", http.StatusBadRequest},
		{"unknown path", "/api/v1/nope", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("GET %s = %d, want %d", tc.path, resp.StatusCode, tc.want)
			}
		})
	}

	t.Run("method not allowed", func(t *testing.T) {
		resp, err := http.Post(srv.URL+"/api/v1/status", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("POST /api/v1/status = %d, want 405", resp.StatusCode)
		}
	})
}

func TestAuth(t *testing.T) {
	st := newTestStore(t)
	seed(t, st)
	srv := httptest.NewServer(New(service.New(st), "dev", "s3cret").Handler())
	defer srv.Close()

	t.Run("missing token", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/status")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
		if resp.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("missing WWW-Authenticate header")
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer nope")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer s3cret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
	})
}

func sendJSON(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestWriteEndpoints(t *testing.T) {
	st := newTestStore(t)
	svc := service.New(st)
	srv := httptest.NewServer(New(svc, "dev", "").Handler())
	defer srv.Close()

	resp := sendJSON(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"name":"svelte","provider":"github","repo":"sveltejs/svelte"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST projects = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	var projects []projectSummary
	if code := getJSON(t, srv.URL+"/api/v1/projects", &projects); code != 200 || len(projects) != 1 || projects[0].Name != "svelte" {
		t.Fatalf("projects = %d, %+v", code, projects)
	}

	resp = sendJSON(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"name":"","provider":"github","repo":"x/y"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid POST = %d, want 400", resp.StatusCode)
	}

	resp = sendJSON(t, http.MethodPost, srv.URL+"/api/v1/feeds", `{"name":"alpine","url":"https://example.com/atom.xml"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST feeds = %d, want 201", resp.StatusCode)
	}

	resp = sendJSON(t, http.MethodDelete, srv.URL+"/api/v1/projects/1", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE project = %d, want 204", resp.StatusCode)
	}

	resp = sendJSON(t, http.MethodDelete, srv.URL+"/api/v1/projects/999", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE missing = %d, want 404", resp.StatusCode)
	}
}

func TestStatusUpdate(t *testing.T) {
	st := newTestStore(t)
	svc := service.New(st)

	apiSrv := New(svc, "v1.0.0", "")
	apiSrv.SetUpdateStatus(func() *update.Status {
		return &update.Status{Current: "v1.0.0", Latest: "v1.1.0", Available: true, Comparable: true}
	})
	srv := httptest.NewServer(apiSrv.Handler())
	defer srv.Close()

	var got statusResponse
	if code := getJSON(t, srv.URL+"/api/v1/status", &got); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if got.Update == nil || !got.Update.Available || got.Update.Latest != "v1.1.0" {
		t.Fatalf("unexpected update: %+v", got.Update)
	}
}
