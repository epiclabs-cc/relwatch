package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SMTP.Enabled {
		t.Fatal("want smtp disabled for missing file")
	}
}

func TestLoadParsesSMTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	content := "smtp:\n" +
		"  enabled: true\n" +
		"  host: smtp.example.com\n" +
		"  port: 587\n" +
		"  username: bot\n" +
		"  password: secret\n" +
		"  from: updates@example.com\n" +
		"  tls: starttls\n" +
		"  to:\n" +
		"    - me@example.com\n" +
		"    - fam@example.com\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.SMTP
	if !s.Enabled || s.Host != "smtp.example.com" || s.Port != 587 || s.Username != "bot" || s.Password != "secret" || s.From != "updates@example.com" {
		t.Fatalf("unexpected smtp config: %+v", s)
	}
	if s.TLS != "starttls" {
		t.Fatalf("unexpected tls mode: %q", s.TLS)
	}
	if len(s.To) != 2 || s.To[0] != "me@example.com" || s.To[1] != "fam@example.com" {
		t.Fatalf("unexpected recipients: %v", s.To)
	}
}

func TestLoadPartialSMTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(path, []byte("smtp:\n  host: smtp.example.com\n  from: a@b.c\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SMTP.Enabled || cfg.SMTP.Port != 0 || cfg.SMTP.Host != "smtp.example.com" {
		t.Fatalf("unexpected smtp config: %+v", cfg.SMTP)
	}
}

func TestLoadParsesDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(path, []byte("db: \"/var/lib/relwatch/relwatch.db\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB != "/var/lib/relwatch/relwatch.db" {
		t.Fatalf("unexpected db path: %q", cfg.DB)
	}
}

func TestLoadParsesHTTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	content := "http:\n" +
		"  enabled: true\n" +
		"  listen: 127.0.0.1:9000\n" +
		"  token: s3cret\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.HTTP
	if !h.Enabled || h.Listen != "127.0.0.1:9000" || h.Token != "s3cret" {
		t.Fatalf("unexpected http config: %+v", h)
	}
}

func TestLoadParsesUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(path, []byte("update:\n  server: https://mirror.example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Update.Server != "https://mirror.example.com" {
		t.Fatalf("unexpected update config: %+v", cfg.Update)
	}
}

func TestLoadMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(path, []byte(":\tbroken: [\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("want parse error")
	}
}
