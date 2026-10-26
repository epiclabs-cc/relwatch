package cli

import (
	"strings"
	"testing"

	"relwatch/internal/store"
)

func projects() []store.Project {
	return []store.Project{
		{ID: 1, Name: "gh", Provider: "github", Repo: "cli/cli"},
		{ID: 2, Name: "gl", Provider: "gitlab", Repo: "3hg/nakedeb"},
		{ID: 3, Name: "same", Provider: "github", Repo: "org/proj"},
		{ID: 4, Name: "same", Provider: "gitlab", Repo: "org/proj"},
	}
}

func TestResolveProjectsAll(t *testing.T) {
	got, err := resolveProjects(projects(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d projects, want 4", len(got))
	}
}

func TestResolveProjectsByID(t *testing.T) {
	got, err := resolveProjects(projects(), "2", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("got %+v, want project id 2", got)
	}
}

func TestResolveProjectsByIDMissing(t *testing.T) {
	_, err := resolveProjects(projects(), "99", "")
	if err == nil || !strings.Contains(err.Error(), "no tracked project with id 99") {
		t.Fatalf("got %v, want id-missing error", err)
	}
}

func TestResolveProjectsByName(t *testing.T) {
	got, err := resolveProjects(projects(), "gl", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "gl" {
		t.Fatalf("got %+v, want project gl", got)
	}
}

func TestResolveProjectsByRepo(t *testing.T) {
	got, err := resolveProjects(projects(), "cli/cli", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Repo != "cli/cli" {
		t.Fatalf("got %+v, want repo cli/cli", got)
	}
}

func TestResolveProjectsNotFound(t *testing.T) {
	_, err := resolveProjects(projects(), "nope", "")
	if err == nil || !strings.Contains(err.Error(), "no tracked project") {
		t.Fatalf("got %v, want not-found error", err)
	}
}

func TestResolveProjectsAmbiguous(t *testing.T) {
	_, err := resolveProjects(projects(), "same", "")
	if err == nil {
		t.Fatal("want ambiguous-match error, got nil")
	}
	var wantAmbiguous bool
	for _, msg := range []string{"matches multiple projects", "--provider", "--project <id>"} {
		if strings.Contains(err.Error(), msg) {
			wantAmbiguous = true
		}
	}
	if !wantAmbiguous {
		t.Fatalf("ambiguous error missing guidance: %v", err)
	}
}

func TestResolveProjectsAmbiguousDisambiguatedByProvider(t *testing.T) {
	got, err := resolveProjects(projects(), "same", "gitlab")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 4 {
		t.Fatalf("got %+v, want gitlab project id 4", got)
	}
}

func TestResolveProjectsProviderUnknown(t *testing.T) {
	_, err := resolveProjects(projects(), "", "forges")
	if err == nil || !strings.Contains(err.Error(), "no tracked projects with provider") {
		t.Fatalf("got %v, want provider-not-found error", err)
	}
}

func TestResolveProjectsProviderCaseInsensitive(t *testing.T) {
	got, err := resolveProjects(projects(), "gl", "GITLAB")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("got %+v, want gitlab project", got)
	}
}
