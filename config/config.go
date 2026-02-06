package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds user-facing settings persisted to disk as JSON.
type Config struct {
	// MpvPath is an explicit path to the mpv binary.
	// When empty, the player package falls back to exec.LookPath.
	MpvPath string `json:"mpv_path,omitempty"`
}

// configDir returns the platform-appropriate config directory:
//
//	Linux/macOS: ~/.config/just-stream
//	Windows:     %APPDATA%\just-stream
func configDir() (string, error) {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "just-stream"), nil
	}

	// XDG on Linux / macOS.
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "just-stream"), nil
}

// Path returns the full path to the config JSON file.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config from disk. Returns a zero Config (not an error)
// if the file does not exist yet.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return &Config{}, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(p, data, 0o644)
}
