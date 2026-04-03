package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestFormatTerminalLogLineDecodesBase64CommandOutput(t *testing.T) {
	t.Parallel()

	line := "cmd phase=clone name=git stream=stdout b64=" + base64.StdEncoding.EncodeToString([]byte("Cloning into 'repo'..."))

	got, mode := formatTerminalLogLine(line)
	if mode != terminalLogModeNormal {
		t.Fatalf("mode = %v, want normal", mode)
	}
	if strings.Contains(got, "b64=") {
		t.Fatalf("expected decoded line without b64 field: %q", got)
	}
	if !strings.Contains(got, `text="Cloning into 'repo'..."`) {
		t.Fatalf("expected decoded text in output: %q", got)
	}
}

func TestFormatTerminalLogLineSuppressesLowSignalCodexOutput(t *testing.T) {
	t.Parallel()

	line := "cmd phase=codex name=codex stream=stderr b64=" + base64.StdEncoding.EncodeToString([]byte(`  it("times out waiting for websocket response deadline after non-response payloads", async () => {`))

	got, mode := formatTerminalLogLine(line)
	if mode != terminalLogModeDrop {
		t.Fatalf("mode = %v, want drop", mode)
	}
	if got != "" {
		t.Fatalf("got %q, want empty output", got)
	}
}

func TestFormatTerminalLogLineKeepsImportantCodexOutput(t *testing.T) {
	t.Parallel()

	line := "cmd phase=codex name=codex stream=stderr b64=" + base64.StdEncoding.EncodeToString([]byte("ERROR: failed to apply patch"))

	got, mode := formatTerminalLogLine(line)
	if mode != terminalLogModeNormal {
		t.Fatalf("mode = %v, want normal", mode)
	}
	if !strings.Contains(got, `text="ERROR: failed to apply patch"`) {
		t.Fatalf("expected important codex output to be kept: %q", got)
	}
}

func TestFormatTerminalLogLineCompactsSingleRunCodexHeartbeat(t *testing.T) {
	t.Parallel()

	got, mode := formatTerminalLogLine("stage=codex status=running elapsed_s=45")
	if mode != terminalLogModeProgress {
		t.Fatalf("mode = %v, want progress", mode)
	}
	if got != "codex running... 45s" {
		t.Fatalf("got %q, want compact progress line", got)
	}
}

func TestFormatTerminalLogLineLeavesHubHeartbeatUntouched(t *testing.T) {
	t.Parallel()

	line := "dispatch request_id=req-9 stage=codex status=running elapsed_s=30"
	got, mode := formatTerminalLogLine(line)
	if mode != terminalLogModeNormal {
		t.Fatalf("mode = %v, want normal", mode)
	}
	if got != line {
		t.Fatalf("got %q, want original line", got)
	}
}
