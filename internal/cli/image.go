package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"relwatch/internal/store"
)

func newImageCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Track container images (OCI Distribution v2 registries)",
	}
	cmd.AddCommand(
		newImageAddCmd(opts),
		newImageListCmd(opts),
		newImageRemoveCmd(opts),
	)
	return cmd
}

func newImageAddCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add NAME HOST/REPO",
		Short: "Track a repository of container image tags (e.g. ghcr.io/octocat/hello)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			host, repo, ok := strings.Cut(args[1], "/")
			if !ok || repo == "" {
				return fmt.Errorf("image repo must be HOST/REPO, e.g. ghcr.io/octocat/hello")
			}
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			img, err := st.AddContainerImage(name, host, repo)
			if err != nil {
				return err
			}
			fmt.Printf("Added image %q (%s/%s, id %d)\n", img.Name, img.Registry, img.Repo, img.ID)
			return nil
		},
	}
	return cmd
}

func newImageListCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked images and their latest known digest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			summaries, err := st.ListImageSummaries()
			if err != nil {
				return err
			}
			if len(summaries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no images tracked in %s; add one with: relwatch image add <name> <host/repo>\n", opts.dbPath)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tREPO\tREGISTRY\tLATEST TAG\tLATEST DIGEST\tDATE")
			for _, s := range summaries {
				dateStr := ""
				if s.LatestObservedAt != nil {
					dateStr = s.LatestObservedAt.Local().Format("2006-01-02")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Name, s.Repo, s.Registry, s.LatestTag, s.LatestDigest, dateStr)
			}
			return w.Flush()
		},
	}
	return cmd
}

func newImageRemoveCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove ID|NAME|HOST/REPO",
		Short: "Delete a tracked image and its digest history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			images, err := st.ListContainerImages()
			if err != nil {
				return err
			}
			target := args[0]
			var match *store.ContainerImage
			if id, err := strconv.ParseInt(target, 10, 64); err == nil {
				for i := range images {
					if images[i].ID == id {
						match = &images[i]
						break
					}
				}
			} else {
				for i := range images {
					img := &images[i]
					full := img.Registry + "/" + img.Repo
					if img.Name == target || full == target {
						match = img
						break
					}
				}
			}
			if match == nil {
				return fmt.Errorf("no tracked image matching %q (see: relwatch image list)", target)
			}
			if err := st.DeleteContainerImage(match.ID); err != nil {
				return err
			}
			fmt.Printf("Removed image %q (%s/%s, id %d)\n", match.Name, match.Registry, match.Repo, match.ID)
			return nil
		},
	}
	return cmd
}
