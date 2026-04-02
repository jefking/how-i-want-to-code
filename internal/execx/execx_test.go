package execx

import (
	"context"
	"strings"
	"testing"
)

func TestOSRunnerRunSuccess(t *testing.T) {
	t.Parallel()

	r := OSRunner{}
	res, err := r.Run(context.Background(), Command{
		Name: "bash",
		Args: []string{"-lc", "printf hello"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Stdout != "hello" {
		t.Fatalf("stdout = %q, want hello", res.Stdout)
	}
}

func TestOSRunnerRunFailure(t *testing.T) {
	t.Parallel()

	r := OSRunner{}
	res, err := r.Run(context.Background(), Command{
		Name: "bash",
		Args: []string{"-lc", "echo boom 1>&2; exit 7"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(res.Stderr, "boom") {
		t.Fatalf("stderr = %q, want to contain boom", res.Stderr)
	}
}
