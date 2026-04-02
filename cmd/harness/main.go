package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jef/how-i-want-to-code/internal/config"
	"github.com/jef/how-i-want-to-code/internal/execx"
	"github.com/jef/how-i-want-to-code/internal/harness"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "usage: harness run --config <path-to-json>")
		return harness.ExitUsage
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "Path to run configuration JSON")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return harness.ExitUsage
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "missing required --config flag")
		return harness.ExitUsage
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return harness.ExitConfig
	}

	logger := log.New(os.Stderr, "", 0)
	h := harness.New(execx.OSRunner{})
	h.Logf = logger.Printf

	result := h.Run(context.Background(), cfg)
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Err)
		if result.WorkspaceDir != "" {
			fmt.Fprintf(os.Stderr, "workspace: %s\n", result.WorkspaceDir)
		}
		return result.ExitCode
	}

	if result.NoChanges {
		fmt.Printf("status=no_changes workspace=%s branch=%s\n", result.WorkspaceDir, result.Branch)
		return harness.ExitSuccess
	}
	fmt.Printf("status=ok workspace=%s branch=%s", result.WorkspaceDir, result.Branch)
	if result.PRURL != "" {
		fmt.Printf(" pr_url=%s", result.PRURL)
	}
	fmt.Println()

	return harness.ExitSuccess
}
