package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the tool's configuration, loaded from ~/.config/showroom-waiting/config.json.
type Config struct {
	// OutputDir is the base directory for recordings. The actual recording is saved under a url_key directory beneath it.
	OutputDir string `json:"output_dir"`
}

// configPath returns the config file path. It prefers $XDG_CONFIG_HOME and falls back to ~/.config.
func configPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "showroom-waiting", "config.json"), nil
}

// LoadConfig loads the config file. Configuration is optional: a missing file yields a zero value, not an error.
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("cannot read config file (%s): %w", path, err)
	}
	// Treat an empty or whitespace-only file as "no config" (same default behavior as a missing file).
	if len(bytes.TrimSpace(data)) == 0 {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("cannot parse config file (%s): %w", path, err)
	}
	return cfg, nil
}

// resolveBaseDir decides the base output directory. Priority: -o flag > config > current directory.
func resolveBaseDir(flagDir, cfgDir string) string {
	switch {
	case flagDir != "":
		return flagDir
	case cfgDir != "":
		return cfgDir
	default:
		return "."
	}
}

// expandHome expands a leading ~ to the home directory.
// Config values come from JSON and are not shell-expanded, so paths like ~/recordings are resolved here.
func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
