// Package config loads and validates scenario-server configuration.
// Values may be set via a YAML file, environment variables (prefixed SCENARIO_),
// or command-line flags.  Environment variables take precedence over the file.
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all tunable parameters for the scenario server.
type Config struct {
	// SliverConfig is the path to the Sliver operator .cfg file.
	SliverConfig string `yaml:"sliver_config"`

	// AtomicsDir is the directory containing technique YAML files.
	AtomicsDir string `yaml:"atomics_dir"`

	// DBPath is the SQLite database file path.
	DBPath string `yaml:"db_path"`

	// ListenAddr is the HTTP listen address (default ":8080").
	ListenAddr string `yaml:"listen"`

	// AllowOrigin sets the Access-Control-Allow-Origin header (default "*").
	AllowOrigin string `yaml:"allow_origin"`

	// C2Host is the IP or hostname that generated beacon implants call back to.
	// Falls back to C2_HOST env var or 172.20.0.10 when empty.
	C2Host string `yaml:"c2_host"`

	// LogLevel controls verbosity: "debug", "info", "warn", "error".
	LogLevel string `yaml:"log_level"`
}

// Defaults returns a Config populated with sensible defaults.
func Defaults() *Config {
	return &Config{
		ListenAddr:  ":8080",
		DBPath:      "./scenario.db",
		AtomicsDir:  "./atomics",
		AllowOrigin: "*",
		LogLevel:    "info",
	}
}

// Load reads a YAML config file and overlays environment variables.
// A missing file is not an error; environment variables alone are sufficient.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config %q: %w", path, err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing config %q: %w", path, err)
			}
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// applyEnv overlays SCENARIO_* environment variables onto cfg.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SCENARIO_SLIVER_CONFIG"); v != "" {
		cfg.SliverConfig = v
	}
	if v := os.Getenv("SCENARIO_ATOMICS_DIR"); v != "" {
		cfg.AtomicsDir = v
	}
	if v := os.Getenv("SCENARIO_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("SCENARIO_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("SCENARIO_ALLOW_ORIGIN"); v != "" {
		cfg.AllowOrigin = v
	}
	if v := os.Getenv("SCENARIO_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("SCENARIO_C2_HOST"); v != "" {
		cfg.C2Host = v
	} else if v := os.Getenv("C2_HOST"); v != "" {
		cfg.C2Host = v
	}
}

// Validate checks that required fields are present.
func (c *Config) Validate() error {
	if c.SliverConfig == "" {
		return fmt.Errorf("sliver_config (or --config flag) is required")
	}
	if _, err := os.Stat(c.SliverConfig); err != nil {
		return fmt.Errorf("sliver_config %q: %w", c.SliverConfig, err)
	}
	return nil
}

// String returns a human-readable summary (hides sensitive paths).
func (c *Config) String() string {
	return fmt.Sprintf(
		"listen=%s db=%s atomics=%s sliver_cfg=%s c2_host=%s",
		c.ListenAddr, c.DBPath, c.AtomicsDir, c.SliverConfig, c.C2Host,
	)
}

// ParsePort extracts the port number from a listen address like ":8080".
func ParsePort(addr string) (int, error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return strconv.Atoi(addr[i+1:])
		}
	}
	return 0, fmt.Errorf("no port found in %q", addr)
}
