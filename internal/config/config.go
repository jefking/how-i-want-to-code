package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the v1 public contract for a harness run.
type Config struct {
	Version       string   `json:"version"`
	RepoURL       string   `json:"repo_url"`
	BaseBranch    string   `json:"base_branch"`
	TargetSubdir  string   `json:"target_subdir"`
	Prompt        string   `json:"prompt"`
	CommitMessage string   `json:"commit_message"`
	PRTitle       string   `json:"pr_title"`
	PRBody        string   `json:"pr_body"`
	Labels        []string `json:"labels"`
	Reviewers     []string `json:"reviewers"`
}

// Load reads and validates a JSON config from disk.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config json: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks required values and path safety.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if c.Version != "v1" {
		return fmt.Errorf("unsupported version %q", c.Version)
	}
	if strings.TrimSpace(c.RepoURL) == "" {
		return fmt.Errorf("repo_url is required")
	}
	if strings.TrimSpace(c.BaseBranch) == "" {
		return fmt.Errorf("base_branch is required")
	}
	if strings.TrimSpace(c.TargetSubdir) == "" {
		return fmt.Errorf("target_subdir is required")
	}
	if err := validateSubdir(c.TargetSubdir); err != nil {
		return err
	}
	if strings.TrimSpace(c.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(c.CommitMessage) == "" {
		return fmt.Errorf("commit_message is required")
	}
	if strings.TrimSpace(c.PRTitle) == "" {
		return fmt.Errorf("pr_title is required")
	}
	if strings.TrimSpace(c.PRBody) == "" {
		return fmt.Errorf("pr_body is required")
	}
	return nil
}

func validateSubdir(subdir string) error {
	clean := filepath.Clean(subdir)
	if clean == "." || clean == "" {
		return fmt.Errorf("target_subdir must be a non-root relative path")
	}
	if filepath.IsAbs(clean) {
		return fmt.Errorf("target_subdir must be relative")
	}
	if strings.HasPrefix(clean, "..") {
		return fmt.Errorf("target_subdir cannot escape repository root")
	}
	if strings.Contains(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target_subdir cannot contain parent traversals")
	}
	return nil
}
