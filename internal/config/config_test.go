package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	json := `{
  "version": "v1",
  "repo_url": "git@github.com:acme/repo.git",
  "base_branch": "main",
  "target_subdir": "services/api",
  "prompt": "fix tests",
  "commit_message": "feat: update api",
  "pr_title": "feat: update api",
  "pr_body": "Automated update"
}`
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TargetSubdir != "services/api" {
		t.Fatalf("TargetSubdir = %q", cfg.TargetSubdir)
	}
}

func TestLoadRejectsMissingRequiredField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	json := `{
  "version": "v1",
  "base_branch": "main",
  "target_subdir": "services/api",
  "prompt": "fix tests",
  "commit_message": "feat: update api",
  "pr_title": "feat: update api",
  "pr_body": "Automated update"
}`
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "repo_url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsUnsafeSubdir(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Version:       "v1",
		RepoURL:       "git@github.com:acme/repo.git",
		BaseBranch:    "main",
		TargetSubdir:  "../escape",
		Prompt:        "fix tests",
		CommitMessage: "commit",
		PRTitle:       "title",
		PRBody:        "body",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}
