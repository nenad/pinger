package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProbeMode string

const (
	ProbeModeICMP ProbeMode = "ICMP"
	ProbeModeHTTP ProbeMode = "HTTP"
)

type Config struct {
	Target    string    `json:"target"`
	ProbeMode ProbeMode `json:"probe_mode"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Target:    "1.1.1.1",
		ProbeMode: ProbeModeICMP,
	}
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".config", "pinger")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// Load loads the configuration from disk. If the file doesn't exist, returns default config.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), nil
	}

	// Validate and set defaults for missing fields
	if cfg.Target == "" {
		cfg.Target = "1.1.1.1"
	}
	if cfg.ProbeMode != ProbeModeICMP && cfg.ProbeMode != ProbeModeHTTP {
		cfg.ProbeMode = ProbeModeICMP
	}

	return &cfg, nil
}

// Save saves the configuration to disk.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
