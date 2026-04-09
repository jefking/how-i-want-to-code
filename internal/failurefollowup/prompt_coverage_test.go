package failurefollowup

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestTaskLogDirAndIdentifierValidationBranches(t *testing.T) {
	t.Parallel()

	if got, ok := TaskLogDir("/tmp/logs", " - "); !ok || got != filepath.Join("/tmp/logs", fallbackLogSubdir) {
		t.Fatalf("TaskLogDir(fallback) = (%q, %v), want fallback path", got, ok)
	}
	if got, ok := TaskLogDir("/tmp/logs", "."); !ok || got != filepath.Join("/tmp/logs", fallbackLogSubdir) {
		t.Fatalf("TaskLogDir(dot fallback) = (%q, %v), want fallback path", got, ok)
	}

	if got, ok := identifierSubdir("one---two"); !ok || got != filepath.Join("one", "two") {
		t.Fatalf("identifierSubdir(collapsed separators) = (%q, %v)", got, ok)
	}
}

func TestNonRemediableRepoAccessReasonFallsBackToEmpty(t *testing.T) {
	t.Parallel()

	if got := NonRemediableRepoAccessReason(nil); got != "" {
		t.Fatalf("NonRemediableRepoAccessReason(nil) = %q, want empty", got)
	}
	if got := NonRemediableRepoAccessReason(errors.New("unrelated failure")); got != "" {
		t.Fatalf("NonRemediableRepoAccessReason(unrelated) = %q, want empty", got)
	}
}
