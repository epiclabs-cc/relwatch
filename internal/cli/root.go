package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"relwatch/internal/config"
	"relwatch/internal/scm"
	"relwatch/internal/store"
	"relwatch/internal/update"
)

type options struct {
	// dbFlag/configFlag hold the raw --db/--config values ("" when unset).
	dbFlag     string
	configFlag string

	// Resolved by resolvePaths before every command runs.
	dbPath     string
	configPath string
	cfg        config.Config
	configMode configMode

	githubToken string
	gitlabToken string
	gitlabURL   string
	giteaToken  string
	giteaURL    string

	registryToken  string
	registryScheme string

	updateServer string
}

func NewRootCmd() *cobra.Command {
	opts := &options{
		githubToken: os.Getenv("RELWATCH_GITHUB_TOKEN"),
		gitlabToken: os.Getenv("RELWATCH_GITLAB_TOKEN"),
		gitlabURL:   envOr("RELWATCH_GITLAB_URL", scm.DefaultGitLabURL),
		giteaToken:  os.Getenv("RELWATCH_GITEA_TOKEN"),
		giteaURL:    envOr("RELWATCH_GITEA_URL", scm.DefaultGiteaURL),

		registryToken:  os.Getenv("RELWATCH_REGISTRY_TOKEN"),
		registryScheme: envOr("RELWATCH_REGISTRY_SCHEME", "https"),

		updateServer: envOr("RELWATCH_UPDATE_SERVER", ""),
	}

	root := &cobra.Command{
		Use:           "relwatch",
		Short:         "Periodically check providers, feeds, and container images for new releases",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.resolvePaths()
		},
	}

	root.PersistentFlags().StringVar(&opts.dbFlag, "db", "", "path to the SQLite database (overrides config db: and RELWATCH_DB)")
	root.PersistentFlags().StringVar(&opts.configFlag, "config", "", "path to settings YAML config (overrides RELWATCH_CONFIG and the search path)")
	root.PersistentFlags().StringVar(&opts.githubToken, "github-token", opts.githubToken, "GitHub API token (env RELWATCH_GITHUB_TOKEN)")
	root.PersistentFlags().StringVar(&opts.gitlabToken, "gitlab-token", opts.gitlabToken, "GitLab API token (env RELWATCH_GITLAB_TOKEN)")
	root.PersistentFlags().StringVar(&opts.gitlabURL, "gitlab-url", opts.gitlabURL, fmt.Sprintf("GitLab API base URL (env RELWATCH_GITLAB_URL, default %q)", scm.DefaultGitLabURL))
	root.PersistentFlags().StringVar(&opts.giteaToken, "gitea-token", opts.giteaToken, "Gitea API token (env RELWATCH_GITEA_TOKEN)")
	root.PersistentFlags().StringVar(&opts.giteaURL, "gitea-url", opts.giteaURL, fmt.Sprintf("Gitea instance base URL (env RELWATCH_GITEA_URL, default %q)", scm.DefaultGiteaURL))
	root.PersistentFlags().StringVar(&opts.registryToken, "registry-token", opts.registryToken, "OCI registry bearer token (env RELWATCH_REGISTRY_TOKEN)")
	root.PersistentFlags().StringVar(&opts.registryScheme, "registry-scheme", opts.registryScheme, "OCI registry scheme (env RELWATCH_REGISTRY_SCHEME, default https)")
	root.PersistentFlags().StringVar(&opts.updateServer, "update-server", opts.updateServer, fmt.Sprintf("base URL of the update server (env RELWATCH_UPDATE_SERVER, default %q)", update.DefaultServer))

	root.AddCommand(
		newProjectCmd(opts),
		newFeedCmd(opts),
		newImageCmd(opts),
		newCheckCmd(opts),
		newServeCmd(opts),
		newConfigCmd(opts),
		newInitCmd(opts),
		newUserCmd(opts),
		newVersionCmd(opts),
	)
	return root
}

// resolvePaths determines the settings.conf and database paths for the current
// invocation. It is called once (via PersistentPreRunE) before any command
// runs, so every subcommand shares one resolved config and one database.
func (o *options) resolvePaths() error {
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	path, mode := resolveConfig(o.configFlag, os.Getenv("RELWATCH_CONFIG"), exists)
	if path == "" {
		// Nothing found: adopt the per-user config location (created lazily on
		// first use) rather than a bare working-directory file.
		if u := userConfigPath(); u != "" {
			path, mode = u, modeUser
		}
	}

	var cfg config.Config
	if path != "" {
		c, err := config.Load(path)
		if err != nil {
			return fmt.Errorf("loading config %q: %w", path, err)
		}
		cfg = c
	}

	o.configPath = path
	o.configMode = mode
	o.cfg = cfg
	o.dbPath = resolveDBPath(o.dbFlag, os.Getenv("RELWATCH_DB"), cfg, mode, path)
	return nil
}

// ensureConfig creates the per-user settings.conf on first use, recording the
// resolved database path. It is a no-op for system/local installs and when the
// file already exists.
func (o *options) ensureConfig() error {
	if o.configMode != modeUser || o.configPath == "" {
		return nil
	}
	if _, err := os.Stat(o.configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(o.configPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(o.configPath, []byte(defaultUserConfig(o.dbPath)), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "relwatch: created %s (db: %s)\n", o.configPath, o.dbPath)
	return nil
}

// openStore ensures the config/db directories exist and opens the database.
// All DB-touching commands go through it so first-run setup is consistent.
func (o *options) openStore() (*store.Store, error) {
	if err := o.ensureConfig(); err != nil {
		return nil, err
	}
	if dir := filepath.Dir(o.dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return store.Open(o.dbPath)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
