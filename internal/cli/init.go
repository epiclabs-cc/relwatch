package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"relwatch/internal/config"
	"relwatch/internal/store"
)

func newInitCmd(opts *options) *cobra.Command {
	var dbPath string
	var nonInteractive bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a user settings.conf and initialize the database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := opts.configFlag
			if target == "" {
				target = userConfigPath()
			}
			if target == "" {
				return fmt.Errorf("cannot determine a config location (is $HOME set?); pass --config PATH")
			}
			if _, err := os.Stat(target); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "config already exists: %s\n", target)
				return nil
			}

			db := dbPath
			if db == "" && opts.configFlag == "" {
				db = userDBPath()
			}
			if db == "" {
				db = opts.dbPath
			}

			var smtp config.SMTPConfig
			if !nonInteractive && stdinIsTTY() {
				r := bufio.NewReader(cmd.InOrStdin())
				out := cmd.OutOrStdout()
				db = prompt(r, out, "Database path", db)
				if yes(prompt(r, out, "Configure the email digest now? [y/N]", "")) {
					promptSMTP(r, out, &smtp)
				}
			}

			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(renderConfig(db, smtp)), 0o600); err != nil {
				return err
			}

			if dir := filepath.Dir(db); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					return err
				}
			}
			st, err := store.Open(db)
			if err != nil {
				return err
			}
			st.Close()

			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\ninitialized database %s\n", target, db)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "database path to record in settings.conf (default: per-user data dir)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "never prompt; use flags/defaults only")
	return cmd
}

// defaultUserConfig is the settings.conf written on first use, without prompts.
func defaultUserConfig(db string) string {
	return renderConfig(db, config.SMTPConfig{})
}

func renderConfig(db string, smtp config.SMTPConfig) string {
	var b strings.Builder
	b.WriteString("# relwatch settings — see docs/configuration.md.\n")
	b.WriteString("# Keep this file out of version control (it may hold the SMTP password).\n\n")
	fmt.Fprintf(&b, "db: \"%s\"\n\n", yamlEscape(db))
	b.WriteString("# Poll schedule for `serve` as a 5-field cron expression.\n")
	b.WriteString("schedule: \"0 * * * *\"\n\n")

	b.WriteString("smtp:\n")
	fmt.Fprintf(&b, "  enabled: %t\n", smtp.Enabled)
	fmt.Fprintf(&b, "  host: \"%s\"\n", yamlEscape(orDefault(smtp.Host, "smtp.example.com")))
	port := smtp.Port
	if port == 0 {
		port = 587
	}
	fmt.Fprintf(&b, "  port: %d\n", port)
	b.WriteString("  # tls: auto (default), starttls, implicit, none\n")
	fmt.Fprintf(&b, "  tls: \"%s\"\n", yamlEscape(orDefault(smtp.TLS, "auto")))
	if smtp.Username != "" {
		fmt.Fprintf(&b, "  username: \"%s\"\n", yamlEscape(smtp.Username))
	}
	if smtp.Password != "" {
		fmt.Fprintf(&b, "  password: \"%s\"\n", yamlEscape(smtp.Password))
	}
	fmt.Fprintf(&b, "  from: \"%s\"\n", yamlEscape(orDefault(smtp.From, orDefault(smtp.Username, "relwatch@example.com"))))
	b.WriteString("  to:\n")
	if len(smtp.To) == 0 {
		b.WriteString("    - \"you@example.com\"\n")
	} else {
		for _, r := range smtp.To {
			fmt.Fprintf(&b, "    - \"%s\"\n", yamlEscape(r))
		}
	}
	return b.String()
}

func promptSMTP(r *bufio.Reader, out io.Writer, smtp *config.SMTPConfig) {
	host := prompt(r, out, "  SMTP host (blank to skip email)", "")
	if host == "" {
		return
	}
	smtp.Enabled = true
	smtp.Host = host
	smtp.Port = atoiOr(prompt(r, out, "  SMTP port", "587"), 587)
	smtp.Username = prompt(r, out, "  SMTP username (blank for none)", "")
	if smtp.Username != "" {
		smtp.Password = prompt(r, out, "  SMTP password", "")
	}
	smtp.From = prompt(r, out, "  From address", orDefault(smtp.Username, "relwatch@example.com"))
	if to := prompt(r, out, "  Digest recipient(s), comma-separated", ""); to != "" {
		for _, part := range strings.FieldsFunc(to, func(c rune) bool { return c == ',' || c == ' ' }) {
			smtp.To = append(smtp.To, part)
		}
	}
}

func prompt(r *bufio.Reader, out io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func yes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func atoiOr(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}

func yamlEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
