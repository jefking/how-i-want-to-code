package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jef/moltenhub-code/internal/agentruntime"
	"github.com/jef/moltenhub-code/internal/hub"
	"github.com/jef/moltenhub-code/internal/hubui"
)

const claudeAuthDocsURL = "https://code.claude.com/docs/en/authentication"
const claudeGitHubConfigureCommand = "gh auth token"
const claudeGitHubConfigurePlaceholder = "ghp_xxx"

type claudeAuthGate struct {
	mu sync.Mutex

	command           string
	runtimeConfigPath string
	initCfg           hub.InitConfig
}

func newClaudeAuthGate(command string) *claudeAuthGate {
	return newClaudeAuthGateWithConfig(command, "", hub.InitConfig{})
}

func newClaudeAuthGateWithConfig(command, runtimeConfigPath string, initCfg hub.InitConfig) *claudeAuthGate {
	command = strings.TrimSpace(command)
	if command == "" {
		command = agentruntime.HarnessClaude
	}
	return &claudeAuthGate{
		command:           command,
		runtimeConfigPath: strings.TrimSpace(runtimeConfigPath),
		initCfg:           initCfg,
	}
}

func (g *claudeAuthGate) Status(_ context.Context) (hubui.AgentAuthState, error) {
	return g.currentState(), nil
}

func (g *claudeAuthGate) StartDeviceAuth(_ context.Context) (hubui.AgentAuthState, error) {
	return g.currentState(), nil
}

func (g *claudeAuthGate) Verify(_ context.Context) (hubui.AgentAuthState, error) {
	return g.currentState(), nil
}

func (g *claudeAuthGate) Configure(_ context.Context, rawInput string) (hubui.AgentAuthState, error) {
	token := strings.TrimSpace(rawInput)
	if token == "" {
		state := g.currentState()
		state.Ready = false
		state.State = "needs_configure"
		state.Message = "GitHub token is required. Paste a GitHub token below, then click Done."
		state.ConfigureCommand = claudeGitHubConfigureCommand
		state.ConfigurePlaceholder = claudeGitHubConfigurePlaceholder
		return state, fmt.Errorf("github token is required")
	}

	g.mu.Lock()
	runtimeConfigPath := g.runtimeConfigPath
	initCfg := g.initCfg
	g.mu.Unlock()

	if err := hub.SaveRuntimeConfigGitHubToken(runtimeConfigPath, initCfg, token); err != nil {
		state := g.currentState()
		state.Ready = false
		state.State = "needs_configure"
		state.Message = fmt.Sprintf("save github token: %v", err)
		state.ConfigureCommand = claudeGitHubConfigureCommand
		state.ConfigurePlaceholder = claudeGitHubConfigurePlaceholder
		return state, err
	}
	if err := setGitHubTokenEnvironment(token); err != nil {
		state := g.currentState()
		state.Ready = false
		state.State = "needs_configure"
		state.Message = fmt.Sprintf("set github token env: %v", err)
		state.ConfigureCommand = claudeGitHubConfigureCommand
		state.ConfigurePlaceholder = claudeGitHubConfigurePlaceholder
		return state, err
	}

	g.mu.Lock()
	g.initCfg.GitHubToken = token
	g.mu.Unlock()

	return g.currentState(), nil
}

func (g *claudeAuthGate) currentState() hubui.AgentAuthState {
	g.mu.Lock()
	runtimeConfigPath := g.runtimeConfigPath
	initCfg := g.initCfg
	g.mu.Unlock()

	githubToken, _ := firstConfiguredGitHubToken(runtimeConfigPath, initCfg)
	if githubToken != "" {
		if err := setGitHubTokenEnvironment(githubToken); err != nil {
			return hubui.AgentAuthState{
				Harness:   agentruntime.HarnessClaude,
				Required:  true,
				Ready:     false,
				State:     "needs_configure",
				Message:   fmt.Sprintf("set github token env: %v", err),
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
	}

	claudeReady, claudeMessage := g.probeClaude()
	if strings.TrimSpace(githubToken) == "" {
		return hubui.AgentAuthState{
			Harness:              agentruntime.HarnessClaude,
			Required:             true,
			Ready:                false,
			State:                "needs_configure",
			Message:              "GitHub token is required. Set GITHUB_TOKEN/GH_TOKEN or paste a token below, then click Done.",
			ConfigureCommand:     claudeGitHubConfigureCommand,
			ConfigurePlaceholder: claudeGitHubConfigurePlaceholder,
			UpdatedAt:            time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	if !claudeReady {
		return hubui.AgentAuthState{
			Harness:   agentruntime.HarnessClaude,
			Required:  true,
			Ready:     false,
			State:     "needs_browser_login",
			Message:   claudeMessage,
			AuthURL:   claudeAuthDocsURL,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}

	return hubui.AgentAuthState{
		Harness:   agentruntime.HarnessClaude,
		Required:  true,
		Ready:     true,
		State:     "ready",
		Message:   "Claude Code and GitHub token are ready.",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (g *claudeAuthGate) probeClaude() (bool, string) {
	if enabled := firstClaudeEnabledProvider(); enabled != "" {
		return true, fmt.Sprintf("Claude Code is configured for %s credentials.", enabled)
	}
	if strings.TrimSpace(os.Getenv("ANTHROPIC_AUTH_TOKEN")) != "" {
		return true, "Claude Code is configured via ANTHROPIC_AUTH_TOKEN."
	}
	if strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != "" {
		return true, "Claude Code is configured via ANTHROPIC_API_KEY."
	}
	if path := claudeCredentialsPath(); path != "" {
		return true, fmt.Sprintf("Claude Code credentials were found at %s.", path)
	}

	return false, "Claude Code login is required. Run `claude`, complete the browser sign-in, then click Done."
}

func firstConfiguredGitHubToken(runtimeConfigPath string, initCfg hub.InitConfig) (value string, source string) {
	if env := strings.TrimSpace(os.Getenv("GH_TOKEN")); env != "" {
		return env, "environment"
	}
	if env := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); env != "" {
		return env, "environment"
	}
	if init := strings.TrimSpace(initCfg.GitHubToken); init != "" {
		return init, "init config"
	}
	if persisted := loadPersistedGitHubToken(runtimeConfigPath); persisted != "" {
		return persisted, "runtime config"
	}
	return "", ""
}

func loadPersistedGitHubToken(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return firstNonEmptyString(
		stringValue(doc["github_token"]),
		stringValue(doc["githubToken"]),
		stringValue(doc["GITHUB_TOKEN"]),
	)
}

func setGitHubTokenEnvironment(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("github token is required")
	}
	if err := os.Setenv("GITHUB_TOKEN", token); err != nil {
		return err
	}
	if err := os.Setenv("GH_TOKEN", token); err != nil {
		return err
	}
	return nil
}

func firstClaudeEnabledProvider() string {
	providers := []struct {
		flag string
		name string
	}{
		{flag: "CLAUDE_CODE_USE_BEDROCK", name: "Amazon Bedrock"},
		{flag: "CLAUDE_CODE_USE_VERTEX", name: "Google Vertex AI"},
		{flag: "CLAUDE_CODE_USE_FOUNDRY", name: "Microsoft Foundry"},
	}
	for _, provider := range providers {
		if envEnabled(provider.flag) {
			return provider.name
		}
	}
	return ""
}

func envEnabled(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func claudeCredentialsPath() string {
	base := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".claude")
	}
	path := filepath.Join(base, ".credentials.json")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ""
	}
	return path
}
