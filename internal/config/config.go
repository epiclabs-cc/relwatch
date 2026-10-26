package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DB          string       `yaml:"db"`
	SMTP        SMTPConfig   `yaml:"smtp"`
	HTTP        HTTPConfig   `yaml:"http"`
	Update      UpdateConfig `yaml:"update"`
	Schedule    string       `yaml:"schedule"`
	Prereleases bool         `yaml:"prereleases"`
}

// UpdateConfig points the `version --check` and `upgrade` commands at an update
// server. Server defaults to the public download mirror when empty.
type UpdateConfig struct {
	Server string `yaml:"server"`
}

// HTTPConfig enables the HTTP listener served by `serve`: the web dashboard at
// `/` and the JSON API at `/api/v1`. It is disabled unless Enabled is true (or
// --http / RELWATCH_HTTP_ADDR override the listen address, which also
// enables it).
type HTTPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
	Token   string `yaml:"token"`
}

type SMTPConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
	TLS      string   `yaml:"tls"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
