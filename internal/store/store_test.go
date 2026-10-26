package store

import (
	"errors"
	"testing"
	"time"
)

func TestAddProject(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p, err := s.AddProject("svelte", "github", "sveltejs/svelte", "")
	if err != nil {
		t.Fatal(err)
	}
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "svelte" || projects[0].Repo != "sveltejs/svelte" {
		t.Fatalf("unexpected projects: %+v", projects)
	}
	if projects[0].CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	t.Logf("ok id=%d created_at=%s", p.ID, projects[0].CreatedAt)
}

func TestListProjectSummaries(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.AddProject("svelte", "github", "sveltejs/svelte", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject("vitest", "github", "vitest-dev/vitest", ""); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.ListProjectSummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	for _, sm := range summaries {
		if sm.LatestTag != "" {
			t.Fatalf("expected no latest for %s, got %q", sm.Name, sm.LatestTag)
		}
	}

	published := time.Date(2026, 8, 28, 23, 31, 40, 0, time.UTC)
	if _, err := s.UpsertRelease(Release{
		ProjectID:   1,
		Tag:         "svelte@5.57.0",
		PublishedAt: &published,
		URL:         "https://github.com/sveltejs/svelte/releases/tag/svelte%405.57.0",
	}); err != nil {
		t.Fatal(err)
	}

	summaries, err = s.ListProjectSummaries()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ProjectSummary{}
	for _, sm := range summaries {
		byName[sm.Name] = sm
	}

	sv := byName["svelte"]
	if sv.LatestTag != "svelte@5.57.0" || sv.LatestReleasedAt == nil || !sv.LatestReleasedAt.Equal(published) {
		t.Fatalf("unexpected svelte summary: %+v", sv)
	}
	if sv.LatestURL == "" {
		t.Fatal("expected latest URL")
	}
	if v := byName["vitest"]; v.LatestTag != "" {
		t.Fatalf("expected no latest for vitest, got %q", v.LatestTag)
	}
}

func TestListReleasesAndGetProjectSummary(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.AddProject("svelte", "github", "sveltejs/svelte", ""); err != nil {
		t.Fatal(err)
	}

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for tag, when := range map[string]time.Time{"v1.0.0": older, "v2.0.0": newer} {
		if _, err := s.UpsertRelease(Release{ProjectID: 1, Tag: tag, PublishedAt: &when}); err != nil {
			t.Fatal(err)
		}
	}

	releases, err := s.ListReleases(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 || releases[0].Tag != "v2.0.0" || releases[1].Tag != "v1.0.0" {
		t.Fatalf("expected newest-first [v2.0.0 v1.0.0], got %+v", releases)
	}

	sum, err := s.GetProjectSummary(1)
	if err != nil {
		t.Fatal(err)
	}
	if sum.LatestTag != "v2.0.0" {
		t.Fatalf("expected latest v2.0.0, got %q", sum.LatestTag)
	}

	if _, err := s.GetProjectSummary(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFeedLifecycle(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	f, err := s.AddFeed("alpine", "https://alpinelinux.org/atom.xml")
	if err != nil {
		t.Fatal(err)
	}

	feeds, err := s.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].Name != "alpine" || feeds[0].URL != "https://alpinelinux.org/atom.xml" {
		t.Fatalf("unexpected feeds: %+v", feeds)
	}

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	inserted, err := s.UpsertFeedRelease(FeedRelease{FeedID: f.ID, Tag: "v1", PublishedAt: &older, IsLatest: true})
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}
	if _, err := s.UpsertFeedRelease(FeedRelease{FeedID: f.ID, Tag: "v1", PublishedAt: &older, IsLatest: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertFeedRelease(FeedRelease{FeedID: f.ID, Tag: "v2", PublishedAt: &newer, IsLatest: true}); err != nil {
		t.Fatal(err)
	}

	if n, err := s.CountFeedReleases(f.ID); err != nil || n != 2 {
		t.Fatalf("expected 2 feed releases, got %d (err=%v)", n, err)
	}

	releases, err := s.ListFeedReleases(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 || releases[0].Tag != "v2" || releases[1].Tag != "v1" {
		t.Fatalf("expected newest-first [v2 v1], got %+v", releases)
	}

	summaries, err := s.ListFeedSummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].LatestTag != "v2" {
		t.Fatalf("unexpected feed summaries: %+v", summaries)
	}

	if err := s.DeleteFeed(f.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CountFeedReleases(f.ID); err != nil || n != 0 {
		t.Fatalf("expected 0 feed releases after delete, got %d (err=%v)", n, err)
	}
}

func TestListImageDigestsAndGetImageSummary(t *testing.T) {
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	img, err := s.AddContainerImage("caddy", "docker.io", "library/caddy")
	if err != nil {
		t.Fatal(err)
	}

	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for tag, d := range map[string]struct {
		digest string
		at     time.Time
	}{
		"2.7": {"sha256:aaa", t1},
		"2.8": {"sha256:bbb", t2},
	} {
		if _, err := s.UpsertImageDigest(ImageDigest{ImageID: img.ID, Tag: tag, Digest: d.digest, ObservedAt: &d.at}); err != nil {
			t.Fatal(err)
		}
	}

	digests, err := s.ListImageDigests(img.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(digests) != 2 || digests[0].Tag != "2.8" || digests[1].Tag != "2.7" {
		t.Fatalf("expected newest-first [2.8 2.7], got %+v", digests)
	}

	sum, err := s.GetImageSummary(img.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.LatestDigest != "sha256:bbb" {
		t.Fatalf("expected latest sha256:bbb, got %q", sum.LatestDigest)
	}

	if _, err := s.GetImageSummary(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
