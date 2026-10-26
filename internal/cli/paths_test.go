package cli

import (
	"testing"

	"relwatch/internal/config"
)

func TestResolveConfigPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	sysPath := "/etc/relwatch/settings.conf"
	userPath := "/xdg/config/relwatch/settings.conf"

	all := func(string) bool { return true }
	none := func(string) bool { return false }

	tests := []struct {
		name     string
		flag     string
		env      string
		exists   func(string) bool
		wantPath string
		wantMode configMode
	}{
		{"flag wins", "/custom/conf", "/env/conf", all, "/custom/conf", modeLocal},
		{"env wins", "", "/env/conf", all, "/env/conf", modeLocal},
		{"system when present", "", "", func(p string) bool { return p == sysPath || p == userPath }, sysPath, modeSystem},
		{"local beats user", "", "", func(p string) bool { return p == "settings.conf" || p == userPath }, "settings.conf", modeLocal},
		{"user when only user", "", "", func(p string) bool { return p == userPath }, userPath, modeUser},
		{"local when only cwd", "", "", func(p string) bool { return p == "settings.conf" }, "settings.conf", modeLocal},
		{"none", "", "", none, "", modeNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotMode := resolveConfig(tc.flag, tc.env, tc.exists)
			if gotPath != tc.wantPath || gotMode != tc.wantMode {
				t.Fatalf("resolveConfig(%q,%q) = (%q,%v), want (%q,%v)",
					tc.flag, tc.env, gotPath, gotMode, tc.wantPath, tc.wantMode)
			}
		})
	}
}

func TestModeFor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	if m := modeFor("/etc/relwatch/settings.conf"); m != modeSystem {
		t.Fatalf("system path mode = %v, want system", m)
	}
	if m := modeFor("/xdg/config/relwatch/settings.conf"); m != modeUser {
		t.Fatalf("user path mode = %v, want user", m)
	}
	if m := modeFor("/somewhere/else.conf"); m != modeLocal {
		t.Fatalf("other path mode = %v, want local", m)
	}
}

func TestResolveDBPathPrecedence(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	cfg := config.Config{DB: "/from/config.db"}

	tests := []struct {
		name       string
		flag       string
		env        string
		cfg        config.Config
		mode       configMode
		configPath string
		want       string
	}{
		{"flag wins", "/from/flag.db", "/from/env.db", cfg, modeSystem, "/etc/relwatch/settings.conf", "/from/flag.db"},
		{"env wins", "", "/from/env.db", cfg, modeSystem, "/etc/relwatch/settings.conf", "/from/env.db"},
		{"config db wins", "", "", cfg, modeSystem, "/etc/relwatch/settings.conf", "/from/config.db"},
		{"system default", "", "", config.Config{}, modeSystem, "/etc/relwatch/settings.conf", "/var/lib/relwatch/relwatch.db"},
		{"user default", "", "", config.Config{}, modeUser, "/xdg/config/relwatch/settings.conf", "/xdg/data/relwatch/relwatch.db"},
		{"local default", "", "", config.Config{}, modeLocal, "settings.conf", "relwatch.db"},
		{"none falls back to local", "", "", config.Config{}, modeNone, "", "relwatch.db"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDBPath(tc.flag, tc.env, tc.cfg, tc.mode, tc.configPath); got != tc.want {
				t.Fatalf("resolveDBPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveDBPathRelativeToConfigDir(t *testing.T) {
	cfg := config.Config{DB: "data/local.db"}
	got := resolveDBPath("", "", cfg, modeLocal, "/etc/relwatch/settings.conf")
	if want := "/etc/relwatch/data/local.db"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	tests := []struct {
		p, base, want string
	}{
		{"~", ".", "/home/tester"},
		{"~/x.db", ".", "/home/tester/x.db"},
		{"/abs/x.db", "/base", "/abs/x.db"},
		{"rel/x.db", "/base", "/base/rel/x.db"},
	}
	for _, tc := range tests {
		if got := expandPath(tc.p, tc.base); got != tc.want {
			t.Fatalf("expandPath(%q,%q) = %q, want %q", tc.p, tc.base, got, tc.want)
		}
	}
}

func TestUserPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	if got := userConfigPath(); got != "/xdg/config/relwatch/settings.conf" {
		t.Fatalf("userConfigPath = %q", got)
	}
	if got := userDBPath(); got != "/xdg/data/relwatch/relwatch.db" {
		t.Fatalf("userDBPath = %q", got)
	}
}

func TestUserPathsFallBackToHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	if got := userConfigPath(); got != "/home/tester/.config/relwatch/settings.conf" {
		t.Fatalf("userConfigPath = %q", got)
	}
	if got := userDBPath(); got != "/home/tester/.local/share/relwatch/relwatch.db" {
		t.Fatalf("userDBPath = %q", got)
	}
}
