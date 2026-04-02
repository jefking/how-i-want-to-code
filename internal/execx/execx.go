package execx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Command describes a subprocess execution.
type Command struct {
	Dir  string
	Name string
	Args []string
}

// Result is subprocess output.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes subprocesses.
type Runner interface {
	Run(ctx context.Context, cmd Command) (Result, error)
}

// OSRunner executes commands via os/exec.
type OSRunner struct{}

// Run executes cmd and captures output.
func (OSRunner) Run(ctx context.Context, cmd Command) (Result, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	if cmd.Dir != "" {
		c.Dir = cmd.Dir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		return res, fmt.Errorf("run %s %v: %w", cmd.Name, cmd.Args, err)
	}
	return res, nil
}
