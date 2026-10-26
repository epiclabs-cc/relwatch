package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"relwatch/internal/config"
	"relwatch/internal/notify"
	"relwatch/internal/registry"
	"relwatch/internal/scm"
	"relwatch/internal/store"
)

func newCheckCmd(opts *options) *cobra.Command {
	var target string
	var provider string
	var prereleases bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check tracked projects for new releases",
		RunE: func(cmd *cobra.Command, args []string) error {
			includePrereleases, err := resolvePrereleases(prereleases, opts.cfg)
			if err != nil {
				return err
			}
			if _, err := runCheck(cmd.Context(), opts, target, provider, includePrereleases, cmd.ErrOrStderr()); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "project", "", "project ID, name, or owner/repo (default: all)")
	cmd.Flags().StringVar(&provider, "provider", "", "only check projects of this provider (github, gitlab, gitea)")
	cmd.Flags().BoolVar(&prereleases, "prereleases", false, "include pre-release versions (opt-in; flag > env RELWATCH_PRERELEASES > settings.prereleases > default skip)")
	return cmd
}

func resolvePrereleases(flagVal bool, cfg config.Config) (bool, error) {
	if flagVal {
		return true, nil
	}
	if v := os.Getenv("RELWATCH_PRERELEASES"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("RELWATCH_PRERELEASES: %v", err)
		}
		return b, nil
	}
	return cfg.Prereleases, nil
}

func resolveProjects(projects []store.Project, target, provider string) ([]store.Project, error) {
	matches := make([]store.Project, 0, len(projects))
	for _, p := range projects {
		if provider != "" && !strings.EqualFold(p.Provider, provider) {
			continue
		}
		matches = append(matches, p)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no tracked projects with provider %q (supported: github, gitlab, gitea)", provider)
	}
	if target == "" {
		return matches, nil
	}

	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		for _, p := range matches {
			if p.ID == id {
				return []store.Project{p}, nil
			}
		}
		return nil, fmt.Errorf("no tracked project with id %d", id)
	}

	var found []store.Project
	for _, p := range matches {
		if p.Name == target || p.Repo == target {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no tracked project named or referenced %q", target)
	case 1:
		return found, nil
	default:
		var ids []string
		for _, p := range found {
			ids = append(ids, fmt.Sprintf("%d (%s/%s)", p.ID, p.Provider, p.Repo))
		}
		return nil, fmt.Errorf("project %q matches multiple projects: %s; disambiguate with --provider or use --project <id>",
			target, strings.Join(ids, ", "))
	}
}
func runCheck(ctx context.Context, opts *options, target, provider string, includePrereleases bool, errOut io.Writer) ([]notify.NewRelease, error) {
	st, err := opts.openStore()
	if err != nil {
		return nil, err
	}
	defer st.Close()
	return checkAll(ctx, opts, st, target, provider, includePrereleases, errOut)
}

// checkAll runs one poll cycle against the given store. `check` opens a store
// per invocation; `serve` shares one long-lived store with the HTTP API.
func checkAll(ctx context.Context, opts *options, st *store.Store, target, provider string, includePrereleases bool, errOut io.Writer) ([]notify.NewRelease, error) {
	newReleases := []notify.NewRelease{}

	projects, err := st.ListProjects()
	if err != nil {
		return nil, err
	}

	factory := scm.NewFactory(scm.Config{
		GitHubToken: opts.githubToken,
		GitLabToken: opts.gitlabToken,
		GitLabURL:   opts.gitlabURL,
		GiteaToken:  opts.giteaToken,
		GiteaURL:    opts.giteaURL,
	})

	checked := 0
	selected := []store.Project{}
	if len(projects) > 0 {
		selected, err = resolveProjects(projects, target, provider)
		if err != nil {
			return newReleases, err
		}
		for _, p := range selected {
			prov, err := factory.For(p.Provider, p.InstanceURL)
			if err != nil {
				fmt.Fprintf(errOut, "project %q: %v\n", p.Name, err)
				continue
			}

			releases, err := prov.ListReleases(ctx, p.Repo, 10)
			if err != nil {
				fmt.Fprintf(errOut, "project %q: %v\n", p.Name, err)
				continue
			}

			established, err := st.CountReleases(p.ID)
			if err != nil {
				fmt.Fprintf(errOut, "project %q: %v\n", p.Name, err)
				continue
			}
			// The first check of a project backfills history as a baseline;
			// new-release reporting only kicks in from the next check onward.
			baseline := established == 0

			for i, rel := range releases {
				if rel.Prerelease && !includePrereleases {
					continue
				}
				var publishedAt *time.Time
				if !rel.PublishedAt.IsZero() {
					publishedAt = &rel.PublishedAt
				}
				inserted, err := st.UpsertRelease(store.Release{
					ProjectID:    p.ID,
					Tag:          rel.Tag,
					Name:         rel.Name,
					PublishedAt:  publishedAt,
					URL:          rel.URL,
					IsLatest:     i == 0,
					IsPrerelease: rel.Prerelease,
				})
				if err != nil {
					fmt.Fprintf(errOut, "project %q: %v\n", p.Name, err)
					break
				}
				if inserted && !baseline {
					when := ""
					if !rel.PublishedAt.IsZero() {
						when = fmt.Sprintf(" (%s)", rel.PublishedAt.Format(time.RFC3339))
					}
					fmt.Printf("new release: %s %s%s %s\n", p.Name, rel.Tag, when, rel.URL)
					newReleases = append(newReleases, notify.NewRelease{
						ProjectID:   p.ID,
						ProjectName: p.Name,
						Provider:    p.Provider,
						Repo:        p.Repo,
						Tag:         rel.Tag,
						Name:        rel.Name,
						URL:         rel.URL,
						PublishedAt: rel.PublishedAt,
					})
				}
			}

			checked++
		}
	} else {
		fmt.Fprintf(errOut, "no projects tracked in %s; add one with: relwatch project add <name> <owner/repo>\n", opts.dbPath)
	}

	// v0.4.0 Feature 3: poll tracked container images through the single
	// generic OCI Distribution v2 client (ghcr/gcr/quay/gitlab/docker/self-hosted).
	images, err := st.ListImageSummaries()
	if err != nil {
		return newReleases, err
	}
	if len(images) > 0 {
		regFactory := registry.NewFactory(registry.Config{
			Scheme: opts.registryScheme,
			Token:  opts.registryToken,
		})
		for _, img := range images {
			client := regFactory.For(img.Registry)
			entries, err := client.ListImages(ctx, img.Repo)
			if err != nil {
				fmt.Fprintf(errOut, "image %q: %v\n", img.Name, err)
				continue
			}
			established, err := st.CountImageDigests(img.ID)
			if err != nil {
				fmt.Fprintf(errOut, "image %q: %v\n", img.Name, err)
				continue
			}
			baseline := established == 0
			for _, e := range entries {
				moved, err := st.UpsertImageDigest(store.ImageDigest{
					ImageID:    img.ID,
					Tag:        e.Tag,
					Digest:     e.Digest,
					IsLatest:   e.Tag == img.LatestTag,
					ObservedAt: &e.PublishedAt,
				})
				if err != nil {
					fmt.Fprintf(errOut, "image %q: %v\n", img.Name, err)
					break
				}
				if moved && !baseline {
					fmt.Printf("new image release: %s %s (%s) %s\n",
						img.Name, e.Tag, e.Digest, e.URL)
					newReleases = append(newReleases, notify.NewRelease{
						ProjectID:   img.ID,
						ProjectName: img.Name,
						Provider:    "registry",
						Repo:        img.Repo,
						Tag:         e.Tag,
						Name:        e.Tag,
						URL:         e.URL,
						PublishedAt: e.PublishedAt,
					})
				}
			}
		}
	}

	// Feeds (RSS/Atom) are polled like projects but tracked separately.
	feeds, err := st.ListFeeds()
	if err != nil {
		return newReleases, err
	}
	for _, f := range feeds {
		prov, err := factory.For("rss", "")
		if err != nil {
			fmt.Fprintf(errOut, "feed %q: %v\n", f.Name, err)
			continue
		}
		releases, err := prov.ListReleases(ctx, f.URL, 10)
		if err != nil {
			fmt.Fprintf(errOut, "feed %q: %v\n", f.Name, err)
			continue
		}
		established, err := st.CountFeedReleases(f.ID)
		if err != nil {
			fmt.Fprintf(errOut, "feed %q: %v\n", f.Name, err)
			continue
		}
		baseline := established == 0
		for i, rel := range releases {
			if rel.Prerelease && !includePrereleases {
				continue
			}
			var publishedAt *time.Time
			if !rel.PublishedAt.IsZero() {
				publishedAt = &rel.PublishedAt
			}
			inserted, err := st.UpsertFeedRelease(store.FeedRelease{
				FeedID:      f.ID,
				Tag:         rel.Tag,
				Name:        rel.Name,
				PublishedAt: publishedAt,
				URL:         rel.URL,
				IsLatest:    i == 0,
			})
			if err != nil {
				fmt.Fprintf(errOut, "feed %q: %v\n", f.Name, err)
				break
			}
			if inserted && !baseline {
				when := ""
				if !rel.PublishedAt.IsZero() {
					when = fmt.Sprintf(" (%s)", rel.PublishedAt.Format(time.RFC3339))
				}
				fmt.Printf("new release: %s %s%s %s\n", f.Name, rel.Tag, when, rel.URL)
				newReleases = append(newReleases, notify.NewRelease{
					ProjectID:   f.ID,
					ProjectName: f.Name,
					Provider:    "feed",
					Repo:        f.URL,
					Tag:         rel.Tag,
					Name:        rel.Name,
					URL:         rel.URL,
					PublishedAt: rel.PublishedAt,
				})
			}
		}
	}

	fmt.Printf("Checked %d of %d project(s)\n", checked, len(selected))
	if err := st.SetMeta("last_checked_at", strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		fmt.Fprintf(errOut, "relwatch: failed to record check time: %v\n", err)
	}
	return newReleases, nil
}
