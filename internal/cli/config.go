package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"relwatch/internal/config"
	"relwatch/internal/notify"
)

func newConfigCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the resolved configuration",
	}
	cmd.AddCommand(newConfigPathCmd(opts), newConfigTestSMTPCmd(opts))
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

func newConfigTestSMTPCmd(opts *options) *cobra.Command {
	var to []string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "test-smtp",
		Short: "Send a test email using the resolved SMTP settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return errors.New("--timeout must be greater than zero")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			return sendTestEmail(ctx, opts.cfg.SMTP, to, opts.configPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), newSMTPSender)
		},
	}
	cmd.Flags().StringSliceVar(&to, "to", nil, "recipient address (repeatable; overrides smtp.to)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "give up if the SMTP exchange takes longer than this")
	return cmd
}

func newSMTPSender(cfg config.SMTPConfig) (notify.Sender, error) {
	return notify.NewSMTP(cfg)
}

func sendTestEmail(ctx context.Context, cfg config.SMTPConfig, to []string, configPath string, out, errOut io.Writer, newSender func(config.SMTPConfig) (notify.Sender, error)) error {
	to = cleanRecipients(to)
	if len(to) == 0 {
		to = cleanRecipients(cfg.To)
	}
	if len(to) == 0 {
		return errors.New("no recipients: set smtp.to or pass --to")
	}
	if !cfg.Enabled {
		fmt.Fprintln(errOut, "note: smtp.enabled is false; serve will not send digests")
	}
	sender, err := newSender(cfg)
	if err != nil {
		return fmt.Errorf("smtp settings: %w", err)
	}

	server, tlsMode, err := notify.Transport(cfg)
	if err != nil {
		return fmt.Errorf("smtp settings: %w", err)
	}
	if tlsMode == "starttls" {
		tlsMode += " (opportunistic)"
	}
	source := configPath
	if source == "" {
		source = "(none)"
	}
	body := fmt.Sprintf("This is a test email from relwatch.\n\nIf you received it, your SMTP settings work.\n\nconfig: %s\nserver: %s\ntls:    %s\nsent:   %s\n",
		source, server, tlsMode, time.Now().Format(time.RFC1123Z))
	msg := notify.Message{Subject: "relwatch: SMTP test", Body: body, To: to}
	if err := sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("sending test email: %w", err)
	}
	fmt.Fprintf(out, "sent test email to %s via %s using %s (config: %s)\n", strings.Join(to, ", "), server, tlsMode, source)
	return nil
}

func cleanRecipients(recipients []string) []string {
	var result []string
	for _, recipient := range recipients {
		if trimmed := strings.TrimSpace(recipient); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
