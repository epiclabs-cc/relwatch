package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newConfigCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the resolved configuration",
	}
	cmd.AddCommand(newConfigPathCmd(opts))
	return cmd
}

func newConfigPathCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved settings.conf and database paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg := opts.configPath
			note := ""
			switch {
			case cfg == "":
				cfg = "(none — built-in defaults)"
			default:
				if _, err := os.Stat(opts.configPath); os.IsNotExist(err) && opts.configMode == modeUser {
					note = " (will be created on first use)"
				}
			}
			fmt.Fprintf(out, "config: %s [%s]%s\n", cfg, opts.configMode, note)
			fmt.Fprintf(out, "db:     %s\n", opts.dbPath)
			return nil
		},
	}
}
