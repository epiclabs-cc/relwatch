package cli

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"relwatch/internal/store"
)

func newFeedCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed",
		Short: "Track RSS/Atom feeds",
	}
	cmd.AddCommand(
		newFeedAddCmd(opts),
		newFeedListCmd(opts),
		newFeedRemoveCmd(opts),
	)
	return cmd
}

func newFeedAddCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add NAME URL",
		Short: "Add an RSS/Atom feed to track",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			url := args[1]

			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			f, err := st.AddFeed(name, url)
			if err != nil {
				return err
			}
			fmt.Printf("Added feed %q (%s, id %d)\n", f.Name, f.URL, f.ID)
			return nil
		},
	}
	return cmd
}

func newFeedListCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked feeds and their latest known release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			summaries, err := st.ListFeedSummaries()
			if err != nil {
				return err
			}
			if len(summaries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no feeds tracked in %s; add one with: relwatch feed add <name> <url>\n", opts.dbPath)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tURL\tADDED\tLATEST RELEASE\tDATE")
			for _, s := range summaries {
				dateStr := ""
				if s.LatestReleasedAt != nil {
					dateStr = s.LatestReleasedAt.Local().Format("2006-01-02")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Name, s.URL,
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

func newFeedRemoveCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove NAME|ID",
		Short: "Stop tracking a feed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			target := args[0]
			feeds, err := st.ListFeeds()
			if err != nil {
				return err
			}

			var match *store.Feed
			if id, perr := strconv.ParseInt(target, 10, 64); perr == nil {
				for i := range feeds {
					if feeds[i].ID == id {
						match = &feeds[i]
						break
					}
				}
			} else {
				for i := range feeds {
					if feeds[i].Name == target || feeds[i].URL == target {
						match = &feeds[i]
						break
					}
				}
			}
			if match == nil {
				return fmt.Errorf("no tracked feed matching %q (see: relwatch feed list)", target)
			}

			if err := st.DeleteFeed(match.ID); err != nil {
				return err
			}
			fmt.Printf("Removed feed %q (%s, id %d)\n", match.Name, match.URL, match.ID)
			return nil
		},
	}
	return cmd
}
