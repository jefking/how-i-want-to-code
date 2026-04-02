package slug

import (
	"testing"
	"time"
)

func TestFromPromptSanitizes(t *testing.T) {
	t.Parallel()

	got := FromPrompt("  Build API: Add OAuth + tests!!!  ")
	want := "build-api-add-oauth-tests"
	if got != want {
		t.Fatalf("FromPrompt() = %q, want %q", got, want)
	}
}

func TestFromPromptFallback(t *testing.T) {
	t.Parallel()

	got := FromPrompt("###")
	if got != "task" {
		t.Fatalf("FromPrompt() = %q, want task", got)
	}
}

func TestBranchNameIncludesSlugTimeAndGuid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 2, 15, 4, 5, 0, time.UTC)
	got := BranchName("Build API", now, "abcdef123456")
	want := "codex/build-api-20260402-150405-abcdef12"
	if got != want {
		t.Fatalf("BranchName() = %q, want %q", got, want)
	}
}
