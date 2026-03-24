package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultCodexModel = "gpt-5.3-codex"
	DefaultTimeout    = 600
	ConfigFileName    = "config.json"
	XReviewDirName    = ".xreview"
)

// SessionsDirOverride allows tests to redirect session storage to a temp directory.
// Production code should never set this.
var SessionsDirOverride string

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

// SessionsDir returns the path to the sessions directory.
// Sessions are stored in /tmp/xreview/sessions/ (ephemeral by design).
func SessionsDir() string {
	if SessionsDirOverride != "" {
		return SessionsDirOverride
	}
	return filepath.Join(os.TempDir(), "xreview", "sessions")
}
