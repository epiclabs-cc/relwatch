package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"relwatch/internal/store"
)

func newProjectCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage tracked projects",
	}

	cmd.AddCommand(
		newProjectAddCmd(opts),
		newProjectListCmd(opts),
		newProjectRemoveCmd(opts),
	)
	return cmd
}

func newProjectRemoveCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove ID|NAME|OWNER/REPO",
		Short: "Delete a tracked project and its release history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			projects, err := st.ListProjects()
			if err != nil {
				return err
			}

			target := args[0]
			var match *store.Project
			if id, err := strconv.ParseInt(target, 10, 64); err == nil {
				for i := range projects {
					if projects[i].ID == id {
						match = &projects[i]
						break
					}
				}
			} else {
				for i := range projects {
					p := &projects[i]
					if p.Name == target || p.Repo == target {
						match = p
						break
					}
				}
			}
			if match == nil {
				return fmt.Errorf("no tracked project matching %q (see: relwatch project list)", target)
			}

			if err := st.DeleteProject(match.ID); err != nil {
				return err
			}
			fmt.Printf("Removed project %q (%s/%s, id %d)\n", match.Name, match.Provider, match.Repo, match.ID)
			return nil
		},
	}
	return cmd
}

func newProjectAddCmd(opts *options) *cobra.Command {
	var provider string
	var instanceURL string

	cmd := &cobra.Command{
		Use:   "add NAME REPO",
		Short: "Add a project to track",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(provider) {
			case "github", "gitlab", "gitea":
			default:
				return fmt.Errorf("unsupported provider %q (supported: github, gitlab, gitea)", provider)
			}

			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			p, err := st.AddProject(args[0], provider, args[1], instanceURL)
			if err != nil {
				return err
			}
			fmt.Printf("Added project %q (%s/%s, id %d)\n", p.Name, p.Provider, p.Repo, p.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "github", "provider to query (github, gitlab, gitea) (rss feeds use 'feed add')")
	cmd.Flags().StringVar(&instanceURL, "instance-url", "", "provider instance base URL for self-hosted GitLab/Gitea (overrides the global URL)")
	return cmd
}

func newProjectListCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked projects and their latest known release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			summaries, err := st.ListProjectSummaries()
			if err != nil {
				return err
			}
			if len(summaries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no projects tracked in %s; add one with: relwatch project add <name> <owner/repo>\n", opts.dbPath)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tREPO\tPROVIDER\tINSTANCE\tADDED\tLATEST RELEASE\tDATE")
			for _, s := range summaries {
				dateStr := ""
				if s.LatestReleasedAt != nil {
					dateStr = s.LatestReleasedAt.Local().Format("2006-01-02")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Name, s.Repo, s.Provider, s.InstanceURL,
					s.CreatedAt.Local().Format("2006-01-02 15:04:05"),
					s.LatestTag,
					dateStr,
				)
			}
			return w.Flush()
		},
	}

	return cmd
}
