package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		want    string
		wantErr error
	}{
		{"plain tag", `{"tag_name":"0.6.0"}`, 200, "0.6.0", nil},
		{"leading v", `{"tag_name":"v0.6.0"}`, 200, "0.6.0", nil},
		{"no releases", "", 404, "", ErrNoRelease},
		{"empty tag", `{"tag_name":""}`, 200, "", ErrNoRelease},
		{"server error", "boom", 500, "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			got, err := NewClient(srv.URL, srv.Client()).Latest(context.Background())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Latest() err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.status >= 400 && tc.status != 404 {
				if err == nil {
					t.Fatal("want error for non-200 status")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Latest() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLatestInvalidVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"not-a-version"}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, srv.Client()).Latest(context.Background()); err == nil {
		t.Fatal("want error for invalid version")
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		current, latest string
		available       bool
		comparable      bool
	}{
		{"0.5.0", "0.6.0", true, true},
		{"0.6.0", "0.6.0", false, true},
		{"0.7.0", "0.6.0", false, true},
		{"v0.5.0", "v0.6.0", true, true},
		{"0.6.0", "0.6.1", true, true},
		{"dev", "0.6.0", false, false},
		{"", "0.6.0", false, false},
	}
	for _, tc := range tests {
		got, comparable, err := Compare(tc.current, tc.latest)
		if err != nil {
			t.Fatalf("Compare(%q,%q): %v", tc.current, tc.latest, err)
		}
		if got != tc.available || comparable != tc.comparable {
			t.Fatalf("Compare(%q,%q) = (%v,%v), want (%v,%v)",
				tc.current, tc.latest, got, comparable, tc.available, tc.comparable)
		}
	}
}

func TestCompareInvalidLatest(t *testing.T) {
	if _, _, err := Compare("0.6.0", "garbage"); err == nil {
		t.Fatal("want error for invalid latest")
	}
}
