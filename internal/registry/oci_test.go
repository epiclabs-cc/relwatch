package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCanonicalHost(t *testing.T) {
	cases := map[string]string{
		"docker.io":          "registry-1.docker.io",
		"index.docker.io":    "registry-1.docker.io",
		"registry.docker.io": "registry-1.docker.io",
		"ghcr.io":            "ghcr.io",
		"quay.io":            "quay.io",
		"127.0.0.1:8080":     "127.0.0.1:8080",
	}
	for in, want := range cases {
		if got := canonicalHost(in); got != want {
			t.Errorf("canonicalHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestListImagesAnonymousBearer(t *testing.T) {
	var ua string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"test-token"}`))
	}))
	defer tokenSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenSrv.URL+`",service="test",scope="repository:repo:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":["v1","v2"]}`))
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.TrimPrefix(r.URL.Path, "/v2/repo/manifests/"))
		w.WriteHeader(http.StatusOK)
	}))
	defer regSrv.Close()

	host := strings.TrimPrefix(regSrv.URL, "http://")
	client := NewFactory(Config{Scheme: "http"}).For(host)

	images, err := client.ListImages(context.Background(), "repo")
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2", len(images))
	}
	if images[0].Tag != "v1" || images[0].Digest != "sha256:v1" {
		t.Fatalf("unexpected images[0]: %+v", images[0])
	}
	if images[1].Tag != "v2" || images[1].Digest != "sha256:v2" {
		t.Fatalf("unexpected images[1]: %+v", images[1])
	}
	if ua != "relwatch" {
		t.Errorf("User-Agent = %q, want relwatch", ua)
	}
}

func TestListImagesTokenEndpointEmptyToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer tokenSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenSrv.URL+`"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer regSrv.Close()

	host := strings.TrimPrefix(regSrv.URL, "http://")
	client := NewFactory(Config{Scheme: "http"}).For(host)

	_, err := client.ListImages(context.Background(), "repo")
	if err == nil || !strings.Contains(err.Error(), "returned no token") {
		t.Fatalf("got %v, want 'returned no token' error", err)
	}
}

func TestListImagesTokenEndpointForbidden(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"code":"DENIED","message":"denied"}]}`))
	}))
	defer tokenSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+tokenSrv.URL+`"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer regSrv.Close()

	host := strings.TrimPrefix(regSrv.URL, "http://")
	client := NewFactory(Config{Scheme: "http"}).For(host)

	_, err := client.ListImages(context.Background(), "repo")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("got %v, want status error", err)
	}
}
