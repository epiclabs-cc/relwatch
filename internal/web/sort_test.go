package web

import (
	"net/http/httptest"
	"testing"
	"time"

	"relwatch/internal/store"
)

func TestSortProjectsByReleased(t *testing.T) {
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	list := []store.ProjectSummary{
		{Project: store.Project{Name: "old"}, LatestReleasedAt: &older},
		{Project: store.Project{Name: "new"}, LatestReleasedAt: &newer},
		{Project: store.Project{Name: "never"}},
	}

	sortProjects(list, "released", "desc")
	if list[0].Name != "new" || list[1].Name != "old" || list[2].Name != "never" {
		t.Fatalf("desc order = %s,%s,%s, want new,old,never", list[0].Name, list[1].Name, list[2].Name)
	}

	sortProjects(list, "released", "asc")
	if list[0].Name != "old" || list[1].Name != "new" || list[2].Name != "never" {
		t.Fatalf("asc order = %s,%s,%s, want old,new,never", list[0].Name, list[1].Name, list[2].Name)
	}
}

func TestSortProjectsByName(t *testing.T) {
	list := []store.ProjectSummary{
		{Project: store.Project{Name: "zeta"}},
		{Project: store.Project{Name: "alpha"}},
		{Project: store.Project{Name: "mike"}},
	}
	sortProjects(list, "name", "asc")
	if list[0].Name != "alpha" || list[1].Name != "mike" || list[2].Name != "zeta" {
		t.Fatalf("name asc = %s,%s,%s", list[0].Name, list[1].Name, list[2].Name)
	}
	sortProjects(list, "name", "desc")
	if list[0].Name != "zeta" || list[1].Name != "mike" || list[2].Name != "alpha" {
		t.Fatalf("name desc = %s,%s,%s", list[0].Name, list[1].Name, list[2].Name)
	}
}

func TestSortHeaders(t *testing.T) {
	headers := sortHeaders("/projects", "released", "desc", projectCols)

	rel := headers[len(headers)-1]
	if rel.Arrow != "▼" {
		t.Fatalf("released arrow = %q, want ▼", rel.Arrow)
	}
	if rel.Href != "/projects?sort=released&dir=asc" {
		t.Fatalf("released href = %q", rel.Href)
	}

	name := headers[0]
	if name.Href != "/projects?sort=name&dir=asc" || name.Arrow != "" {
		t.Fatalf("name header = %+v", name)
	}
}

func TestTableSort(t *testing.T) {
	r := httptest.NewRequest("GET", "/projects?sort=bogus&dir=sideways", nil)
	key, dir := tableSort(r, projectCols)
	if key != "released" || dir != "desc" {
		t.Fatalf("tableSort() = %q,%q, want released,desc", key, dir)
	}

	r = httptest.NewRequest("GET", "/projects?sort=name&dir=asc", nil)
	key, dir = tableSort(r, projectCols)
	if key != "name" || dir != "asc" {
		t.Fatalf("tableSort() = %q,%q, want name,asc", key, dir)
	}
}
