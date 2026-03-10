package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultCodexModel = "gpt-5.3-Codex"
	DefaultTimeout    = 180
	ConfigFileName    = "config.json"
	SessionsDirName   = "sessions"
	XReviewDirName    = ".xreview"
)

// Config holds project-level xreview configuration from .xreview/config.json.
type Config struct {
	CodexModel     string   `json:"codex_model"`
	DefaultTimeout int      `json:"default_timeout"`
	DefaultContext string   `json:"default_context"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// Load reads .xreview/config.json from the given workdir.
// Returns default config if the file does not exist.
func Load(workdir string) (*Config, error) {
	cfg := &Config{
		CodexModel:     DefaultCodexModel,
		DefaultTimeout: DefaultTimeout,
		IgnorePatterns: []string{
			"**/*_test.go",
			"**/vendor/**",
			"**/*.generated.go",
		},
	}

	path := filepath.Join(workdir, XReviewDirName, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.CodexModel == "" {
		cfg.CodexModel = DefaultCodexModel
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultTimeout
	}

	return cfg, nil
}

// SessionsDir returns the path to .xreview/sessions/ under the given workdir.
func SessionsDir(workdir string) string {
	return filepath.Join(workdir, XReviewDirName, SessionsDirName)
}
