package hub

import (
	"strings"
	"testing"
)

func TestParseSkillDispatchFromPayloadConfig(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "skill_request",
		"skill": defaultSkillName,
		"id":    "req-1",
		"from":  "agent-alpha",
		"payload": map[string]any{
			"config": map[string]any{
				"repo":          "git@github.com:acme/repo.git",
				"base_branch":   "main",
				"target_subdir": ".",
				"prompt":        "update tests",
			},
		},
	}

	dispatch, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if err != nil {
		t.Fatalf("ParseSkillDispatch() error = %v", err)
	}
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if dispatch.RequestID != "req-1" {
		t.Fatalf("RequestID = %q", dispatch.RequestID)
	}
	if dispatch.ReplyTo != "agent-alpha" {
		t.Fatalf("ReplyTo = %q", dispatch.ReplyTo)
	}
	if dispatch.Config.RepoURL != "git@github.com:acme/repo.git" {
		t.Fatalf("RepoURL = %q", dispatch.Config.RepoURL)
	}
	if dispatch.Config.Prompt != "update tests" {
		t.Fatalf("Prompt = %q", dispatch.Config.Prompt)
	}
}

func TestParseSkillDispatchIgnoresDifferentSkill(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "skill_request",
		"skill": "other_skill",
		"config": map[string]any{
			"repo":   "git@github.com:acme/repo.git",
			"prompt": "x",
		},
	}

	_, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if err != nil {
		t.Fatalf("ParseSkillDispatch() error = %v", err)
	}
	if matched {
		t.Fatal("matched = true, want false")
	}
}

func TestParseSkillDispatchRequiresInlineConfigObject(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "skill_request",
		"skill": defaultSkillName,
		"config": map[string]any{
			"path": "/tmp/run.json",
		},
	}

	dispatch, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dispatch.RequestID != "" {
		t.Fatalf("RequestID = %q", dispatch.RequestID)
	}
}

func TestParseSkillDispatchRejectsUnknownConfigFields(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "skill_request",
		"skill": defaultSkillName,
		"config": map[string]any{
			"repo":        "git@github.com:acme/repo.git",
			"prompt":      "make change",
			"unknown_key": true,
		},
	}

	_, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSkillDispatchMissingPayloadIsValidationError(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "skill_request",
		"skill": defaultSkillName,
		"id":    "req-2",
	}

	dispatch, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dispatch.RequestID != "req-2" {
		t.Fatalf("RequestID = %q", dispatch.RequestID)
	}
}

func TestParseSkillDispatchWrongTypeIsValidationError(t *testing.T) {
	t.Parallel()

	msg := map[string]any{
		"type":  "not_skill_request",
		"skill": defaultSkillName,
		"id":    "req-3",
		"config": map[string]any{
			"repo":   "git@github.com:acme/repo.git",
			"prompt": "x",
		},
	}

	dispatch, matched, err := ParseSkillDispatch(msg, "skill_request", defaultSkillName)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected dispatch type") {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatch.RequestID != "req-3" {
		t.Fatalf("RequestID = %q", dispatch.RequestID)
	}
}
