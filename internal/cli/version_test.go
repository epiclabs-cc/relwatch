package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"relwatch/internal/config"
	"relwatch/internal/update"
)

func TestUpdateServerPrecedence(t *testing.T) {
	tests := []struct {
		name string
		flag string
		cfg  string
		want string
	}{
		{"flag wins", "https://flag.example", "https://cfg.example", "https://flag.example"},
		{"config when no flag", "", "https://cfg.example", "https://cfg.example"},
		{"default when neither", "", "", update.DefaultServer},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := &options{updateServer: tc.flag}
			opts.cfg = config.Config{Update: config.UpdateConfig{Server: tc.cfg}}
			if got := updateServer(opts); got != tc.want {
				t.Fatalf("updateServer() = %q, want %q", got, tc.want)
			}
		})
	}
}

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	root.Version = "0.5.0"
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestVersionLocal(t *testing.T) {
	out, err := runRoot(t, "version", "--config", filepath.Join(t.TempDir(), "none.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "relwatch 0.5.0" {
		t.Fatalf("output = %q", out)
	}
}
