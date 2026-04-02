package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestRunUsageMissingSubcommand(t *testing.T) {
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })
	os.Args = []string{"harness"}

	if code := run(); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

func TestRunUsageMissingConfigFlag(t *testing.T) {
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })
	os.Args = []string{"harness", "run"}

	if code := run(); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

func TestRunConfigLoadFailure(t *testing.T) {
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })

	missing := filepath.Join(t.TempDir(), "missing.json")
	os.Args = []string{"harness", "run", "--config", missing}

	if code := run(); code != 10 {
		t.Fatalf("run() = %d, want 10", code)
	}
}

func TestRunUsageUnknownSubcommand(t *testing.T) {
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })
	os.Args = []string{"harness", "unknown"}

	if code := run(); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

func TestRunMultiplexUsageMissingConfigFlag(t *testing.T) {
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })
	os.Args = []string{"harness", "multiplex"}

	if code := run(); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

func TestCollectConfigPathsFilesAndDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.json")
	fileB := filepath.Join(dir, "b.JSON")
	fileTxt := filepath.Join(dir, "notes.txt")
	nestedDir := filepath.Join(dir, "nested")
	fileC := filepath.Join(nestedDir, "c.json")

	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	for _, path := range []string{fileA, fileB, fileTxt, fileC} {
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	got, err := collectConfigPaths([]string{fileA, dir, fileA})
	if err != nil {
		t.Fatalf("collectConfigPaths() error = %v", err)
	}

	want := []string{fileA, fileB, fileC}
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectConfigPaths() = %v, want %v", got, want)
	}
}

func TestResolveHubInitConfigExplicitPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "init.json")
	if err := os.WriteFile(path, []byte(`{"bind_token":"bind_123"}`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

	cfg, err := resolveHubInitConfig(path)
	if err != nil {
		t.Fatalf("resolveHubInitConfig() error = %v", err)
	}
	if cfg.BindToken != "bind_123" {
		t.Fatalf("BindToken = %q", cfg.BindToken)
	}
}

func TestResolveHubInitConfigPrefersTemplatesInitJSON(t *testing.T) {
	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "init.json"), []byte(`{"bind_token":"bind_primary"}`), 0o644); err != nil {
		t.Fatalf("write init.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "init.example.json"), []byte(`{"bind_token":"bind_example"}`), 0o644); err != nil {
		t.Fatalf("write init.example.json: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	cfg, err := resolveHubInitConfig("")
	if err != nil {
		t.Fatalf("resolveHubInitConfig() error = %v", err)
	}
	if cfg.BindToken != "bind_primary" {
		t.Fatalf("BindToken = %q", cfg.BindToken)
	}
}

func TestResolveHubInitConfigFallsBackToExample(t *testing.T) {
	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "init.example.json"), []byte(`{"bind_token":"bind_example"}`), 0o644); err != nil {
		t.Fatalf("write init.example.json: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	cfg, err := resolveHubInitConfig("")
	if err != nil {
		t.Fatalf("resolveHubInitConfig() error = %v", err)
	}
	if cfg.BindToken != "bind_example" {
		t.Fatalf("BindToken = %q", cfg.BindToken)
	}
}

func TestResolveHubInitConfigUsesDefaultsWithoutTemplates(t *testing.T) {
	root := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	cfg, err := resolveHubInitConfig("")
	if err != nil {
		t.Fatalf("resolveHubInitConfig() error = %v", err)
	}
	if cfg.BaseURL != "https://na.hub.molten.bot/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.SessionKey != "main" {
		t.Fatalf("SessionKey = %q", cfg.SessionKey)
	}
	if cfg.BindToken != "" || cfg.AgentToken != "" {
		t.Fatalf("expected empty tokens, got bind=%q agent=%q", cfg.BindToken, cfg.AgentToken)
	}
}
