package hub

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncProfileUsesOpenAPICompatiblePayloads(t *testing.T) {
	t.Parallel()

	type captured struct {
		Path string
		Body map[string]any
	}
	var calls []captured

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		calls = append(calls, captured{Path: r.URL.Path, Body: body})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer ts.Close()

	client := NewAPIClient(ts.URL + "/v1")
	cfg := InitConfig{
		Handle: "codex-beast",
		Profile: ProfileConfig{
			DisplayName: "Codex Harness",
			Emoji:       "🎮",
			Bio:         "Automation worker",
		},
		Skill: SkillConfig{Name: "codex_harness_run"},
	}

	if err := client.SyncProfile(context.Background(), "token", cfg); err != nil {
		t.Fatalf("SyncProfile() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}

	if calls[0].Path != "/v1/agents/me/metadata" {
		t.Fatalf("first path = %q", calls[0].Path)
	}
	if _, ok := calls[0].Body["handle"]; !ok {
		t.Fatalf("first body missing handle: %#v", calls[0].Body)
	}

	if calls[1].Path != "/v1/agents/me/metadata" {
		t.Fatalf("second path = %q", calls[1].Path)
	}
	metaRaw, ok := calls[1].Body["metadata"]
	if !ok {
		t.Fatalf("second body missing metadata wrapper: %#v", calls[1].Body)
	}
	meta, ok := metaRaw.(map[string]any)
	if !ok {
		t.Fatalf("metadata has wrong type: %#v", metaRaw)
	}
	skillsRaw, ok := meta["skills"]
	if !ok {
		t.Fatalf("metadata missing skills: %#v", meta)
	}
	skills, ok := skillsRaw.([]any)
	if !ok || len(skills) == 0 {
		t.Fatalf("skills has wrong type/value: %#v", skillsRaw)
	}
	skill, ok := skills[0].(map[string]any)
	if !ok {
		t.Fatalf("skill has wrong type: %#v", skills[0])
	}
	if _, ok := skill["name"]; !ok {
		t.Fatalf("skill missing name: %#v", skill)
	}
	if _, ok := skill["description"]; !ok {
		t.Fatalf("skill missing description: %#v", skill)
	}
	if _, ok := skill["dispatch_type"]; ok {
		t.Fatalf("skill should not include dispatch_type: %#v", skill)
	}
}))

func TestNormalizeSkillName(t *testing.T) {
	t.Parallel()

	if got := normalizeSkillName("CODEx Harness RUN!!"); got != "codex-harness-run" {
		t.Fatalf("normalizeSkillName() = %q", got)
	}
	if got := normalizeSkillName("@"); got != "codex_harness_run" {
		t.Fatalf("normalizeSkillName() fallback = %q", got)
	}
}
