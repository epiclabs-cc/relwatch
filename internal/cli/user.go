package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"relwatch/internal/service"
)

func newUserCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage dashboard users",
	}
	cmd.AddCommand(newUserAddCmd(opts), newUserListCmd(opts))
	return cmd
}

func newUserAddCmd(opts *options) *cobra.Command {
	var password string
	cmd := &cobra.Command{
		Use:   "add EMAIL",
		Short: "Create a dashboard user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := args[0]
			if password == "" {
				if !stdinIsTTY() {
					return fmt.Errorf("password required: pass --password or run interactively")
				}
				pw, err := promptPassword(cmd)
				if err != nil {
					return err
				}
				password = pw
			}

			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			u, err := service.New(st).CreateUser(email, password)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created user %q (id %d)\n", u.Email, u.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&password, "password", "", "password (non-interactive; prefer the interactive prompt)")
	return cmd
}

func newUserListCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboard users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := opts.openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			users, err := service.New(st).ListUsers()
			if err != nil {
				return err
			}
			for _, u := range users {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\n", u.ID, u.Email)
			}
			return nil
		},
	}
	return cmd
}

func promptPassword(cmd *cobra.Command) (string, error) {
	out := cmd.OutOrStdout()
	fmt.Fprint(out, "Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	fmt.Fprint(out, "Confirm: ")
	confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	if string(pw) != string(confirm) {
		return "", fmt.Errorf("passwords do not match")
	}
	return string(pw), nil
}
