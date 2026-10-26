package cli

import (
	"os"
	"path/filepath"
	"strings"

	"relwatch/internal/config"
)

// Fixed locations for a system-wide ("service") install. Config lives under
// /etc, state (the SQLite database) under /var/lib, and the binary under
// --prefix (default /opt/relwatch).
const (
	systemConfigFile = "/etc/relwatch/settings.conf"
	systemDBFile     = "/var/lib/relwatch/relwatch.db"
)

const (
	appDirName = "relwatch"
	configName = "settings.conf"
	dbName     = "relwatch.db"
	localDBRel = "relwatch.db"
)

// configMode describes where the resolved settings.conf came from; it selects
// the default database path when the config does not set one explicitly.
type configMode int

const (
	modeNone configMode = iota
	modeSystem
	modeUser
	modeLocal
)

func (m configMode) String() string {
	switch m {
	case modeSystem:
		return "system"
	case modeUser:
		return "user"
	case modeLocal:
		return "local"
	default:
		return "none"
	}
}

// configCandidate pairs a search path with the mode it represents.
type configCandidate struct {
	path string
	mode configMode
}

// configCandidates is the search order used when neither --config nor
// RELWATCH_CONFIG is set: an installed service (/etc), then a
// working-directory config (dev/local), then the per-user config.
func configCandidates() []configCandidate {
	cands := []configCandidate{{systemConfigFile, modeSystem}}
	cands = append(cands, configCandidate{configName, modeLocal})
	if u := userConfigPath(); u != "" {
		cands = append(cands, configCandidate{u, modeUser})
	}
	return cands
}

func userConfigDir() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config")
}

func userDataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// userConfigPath is $XDG_CONFIG_HOME/relwatch/settings.conf, or "" when no
// home directory can be determined.
func userConfigPath() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, appDirName, configName)
}

// userDBPath is $XDG_DATA_HOME/relwatch/relwatch.db, or "" when no home
// directory can be determined.
func userDBPath() string {
	dir := userDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, appDirName, dbName)
}

// modeFor classifies an explicitly chosen config path.
func modeFor(path string) configMode {
	if path == systemConfigFile {
		return modeSystem
	}
	if u := userConfigPath(); u != "" && path == u {
		return modeUser
	}
	return modeLocal
}

// resolveConfig picks the settings.conf to load, in precedence order:
//
//	--config  >  RELWATCH_CONFIG  >  /etc  >  ./settings.conf  >  $XDG_CONFIG_HOME
//
// A blank return means nothing was found; the caller then falls back to a
// per-mode default (see resolvePaths).
func resolveConfig(flag, env string, exists func(string) bool) (string, configMode) {
	if flag != "" {
		return flag, modeFor(flag)
	}
	if env != "" {
		return env, modeFor(env)
	}
	for _, c := range configCandidates() {
		if exists(c.path) {
			return c.path, c.mode
		}
	}
	return "", modeNone
}

// resolveDBPath picks the SQLite path, in precedence order:
//
//	--db  >  RELWATCH_DB  >  config db:  >  per-mode default.
func resolveDBPath(flag, env string, cfg config.Config, mode configMode, configPath string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	if cfg.DB != "" {
		return expandPath(cfg.DB, configBaseDir(configPath))
	}
	return defaultDBForMode(mode, configPath)
}

func defaultDBForMode(mode configMode, configPath string) string {
	switch mode {
	case modeSystem:
		return systemDBFile
	case modeUser:
		if p := userDBPath(); p != "" {
			return p
		}
	}
	return filepath.Join(configBaseDir(configPath), localDBRel)
}

// configBaseDir is the directory relative config/db paths resolve against.
func configBaseDir(configPath string) string {
	if configPath == "" {
		return "."
	}
	if d := filepath.Dir(configPath); d != "" {
		return d
	}
	return "."
}

// expandPath resolves a leading ~ and makes a relative path absolute against
// baseDir.
func expandPath(p, baseDir string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			rest := strings.TrimPrefix(p, "~")
			rest = strings.TrimPrefix(rest, string(os.PathSeparator))
			p = filepath.Join(home, rest)
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	return p
}
