package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"relwatch/internal/update"
)

// updateServer resolves the update server in precedence order:
//
//	--update-server flag / RELWATCH_UPDATE_SERVER  >  settings update.server:  >  default.
//
// The flag default already folds in the environment variable, so flag beats env
// beats config beats the built-in mirror.
func updateServer(opts *options) string {
	if opts.updateServer != "" {
		return opts.updateServer
	}
	if s := opts.cfg.Update.Server; s != "" {
		return s
	}
	return update.DefaultServer
}

func newVersionCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the running version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "relwatch %s\n", cmd.Root().Version)
			return nil
		},
	}
}
