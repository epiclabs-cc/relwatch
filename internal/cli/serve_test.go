package cli

import (
	"testing"

	"relwatch/internal/config"
)

func TestResolveSchedulePrecedence(t *testing.T) {
	tests := []struct {
		name      string
		flag, env string
		cfg, def  string
		want      string
	}{
		{"flag beats env", "0 * * * *", "30 * * * *", "0 0 * * *", "0 0 0 0 0", "0 * * * *"},
		{"env beats config", "", "30 * * * *", "0 0 * * *", "0 0 0 0 0", "30 * * * *"},
		{"config beats default", "", "", "0 0 * * *", "0 0 0 0 0", "0 0 * * *"},
		{"default when all empty", "", "", "", "0 0 0 0 0", "0 0 0 0 0"},
		{"flag empty falls through", "", "30 * * * *", "", "0 0 0 0 0", "30 * * * *"},
		{"env empty falls through", "0 * * * *", "", "0 0 * * *", "0 0 0 0 0", "0 * * * *"},
		{"config empty falls through", "0 * * * *", "30 * * * *", "", "0 0 0 0 0", "0 * * * *"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSchedule(tc.flag, tc.env, tc.cfg, tc.def); got != tc.want {
				t.Fatalf("resolveSchedule(%q,%q,%q,%q) = %q, want %q",
					tc.flag, tc.env, tc.cfg, tc.def, got, tc.want)
			}
		})
	}
}

func TestResolveHTTPPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		flag, env   string
		cfg         config.HTTPConfig
		wantEnabled bool
		wantAddr    string
	}{
		{"disabled by default", "", "", config.HTTPConfig{}, false, "127.0.0.1:8080"},
		{"config enabled uses default addr", "", "", config.HTTPConfig{Enabled: true}, true, "127.0.0.1:8080"},
		{"config enabled + listen", "", "", config.HTTPConfig{Enabled: true, Listen: "127.0.0.1:9000"}, true, "127.0.0.1:9000"},
		{"config disabled with listen stays off", "", "", config.HTTPConfig{Listen: "127.0.0.1:9000"}, false, "127.0.0.1:9000"},
		{"env enables and sets addr", "", ":9090", config.HTTPConfig{}, true, ":9090"},
		{"flag enables and sets addr", ":8080", "", config.HTTPConfig{}, true, ":8080"},
		{"flag beats env", ":8080", ":9090", config.HTTPConfig{}, true, ":8080"},
		{"flag beats config", ":8080", "", config.HTTPConfig{Enabled: true, Listen: "127.0.0.1:9000"}, true, ":8080"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enabled, addr := resolveHTTP(tc.flag, tc.env, tc.cfg)
			if enabled != tc.wantEnabled || addr != tc.wantAddr {
				t.Fatalf("resolveHTTP(%q,%q,%+v) = (%v,%q), want (%v,%q)",
					tc.flag, tc.env, tc.cfg, enabled, addr, tc.wantEnabled, tc.wantAddr)
			}
		})
	}
}
