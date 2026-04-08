package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jef/moltenhub-code/internal/agentruntime"
	"github.com/jef/moltenhub-code/internal/hubui"
)

const claudeAuthDocsURL = "https://code.claude.com/docs/en/authentication"

var claudeAuthURLPattern = regexp.MustCompile(`https?://[^\s"'<>()]+`)

type claudeAuthGate struct {
	mu sync.Mutex

	baseCtx context.Context
	logf    func(string, ...any)

	command string

	required bool
	ready    bool
	state    string
	message  string

	authURL   string
	updatedAt time.Time

	procRunning       bool
	procCancel        context.CancelFunc
	procInput         io.WriteCloser
	lastPromptAdvance time.Time
}

func newClaudeAuthGate(ctx context.Context, command string, logf func(string, ...any)) *claudeAuthGate {
	if ctx == nil {
		ctx = context.Background()
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	command = strings.TrimSpace(command)
	if command == "" {
		command = agentruntime.HarnessClaude
	}

	g := &claudeAuthGate{
		baseCtx:   ctx,
		logf:      logf,
		command:   command,
		required:  true,
		state:     "needs_browser_login",
		message:   "Claude Code login is required. Run `claude login`, complete browser sign-in, then click Done.",
		updatedAt: time.Now().UTC(),
	}

	ready, probeMessage := g.probe()
	g.mu.Lock()
	if ready {
		g.ready = true
		g.state = "ready"
		g.message = firstNonEmptyString(probeMessage, "Claude Code authorization is ready.")
	} else if strings.TrimSpace(probeMessage) != "" {
		g.message = probeMessage
	}
	g.updatedAt = time.Now().UTC()
	g.mu.Unlock()

	return g
}

func (g *claudeAuthGate) Status(_ context.Context) (hubui.AgentAuthState, error) {
	if g == nil {
		return hubui.AgentAuthState{
			Required: false,
			Ready:    true,
			State:    "ready",
			Message:  "Agent auth is ready.",
		}, nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	return g.snapshotLocked(), nil
}

func (g *claudeAuthGate) StartDeviceAuth(_ context.Context) (hubui.AgentAuthState, error) {
	if g == nil {
		return hubui.AgentAuthState{
			Required: false,
			Ready:    true,
			State:    "ready",
			Message:  "Agent auth is ready.",
		}, nil
	}

	g.mu.Lock()
	if g.ready {
		snap := g.snapshotLocked()
		g.mu.Unlock()
		return snap, nil
	}
	if g.procRunning {
		g.state = "pending_browser_login"
		g.message = "Waiting for Claude browser sign-in. Complete auth, then click Done."
		g.updatedAt = time.Now().UTC()
		snap := g.snapshotLocked()
		g.mu.Unlock()
		return snap, nil
	}
	g.mu.Unlock()

	tmpDir, err := os.MkdirTemp("", "moltenhub-claude-auth-*")
	if err != nil {
		snap, _ := g.Status(context.Background())
		return snap, fmt.Errorf("create claude login temp dir: %w", err)
	}

	procCtx, cancel := context.WithCancel(g.baseCtx)
	cmd := exec.CommandContext(procCtx, g.command, "login")
	cmd.Dir = tmpDir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		cancel()
		snap, _ := g.Status(context.Background())
		return snap, fmt.Errorf("open claude login stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		cancel()
		snap, _ := g.Status(context.Background())
		return snap, fmt.Errorf("open claude login stderr: %w", err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		cancel()
		snap, _ := g.Status(context.Background())
		return snap, fmt.Errorf("open claude login stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tmpDir)
		cancel()
		_ = stdinPipe.Close()
		g.mu.Lock()
		g.state = "error"
		g.message = fmt.Sprintf("start claude login: %v", err)
		g.updatedAt = time.Now().UTC()
		snap := g.snapshotLocked()
		g.mu.Unlock()
		return snap, err
	}

	g.mu.Lock()
	g.procRunning = true
	g.procCancel = cancel
	g.procInput = stdinPipe
	g.state = "pending_browser_login"
	g.message = "Starting Claude login. Follow prompts, complete browser sign-in, then click Done."
	g.updatedAt = time.Now().UTC()
	snap := g.snapshotLocked()
	g.mu.Unlock()

	go g.readLoginStream(stdoutPipe)
	go g.readLoginStream(stderrPipe)
	go g.waitLogin(cmd, tmpDir)

	return snap, nil
}

func (g *claudeAuthGate) Verify(ctx context.Context) (hubui.AgentAuthState, error) {
	if g == nil {
		return hubui.AgentAuthState{
			Required: false,
			Ready:    true,
			State:    "ready",
			Message:  "Agent auth is ready.",
		}, nil
	}

	ready, probeMessage := g.probe()
	shouldStartLogin := false

	g.mu.Lock()
	if ready {
		g.ready = true
		g.state = "ready"
		g.message = firstNonEmptyString(probeMessage, "Claude Code authorization is ready.")
		if g.procRunning && g.procCancel != nil {
			g.procCancel()
		}
		g.updatedAt = time.Now().UTC()
		snap := g.snapshotLocked()
		g.mu.Unlock()
		return snap, nil
	}

	if g.procRunning {
		g.ready = false
		g.state = "pending_browser_login"
		g.message = "Still waiting for Claude browser sign-in. Complete auth, then click Done."
		g.updatedAt = time.Now().UTC()
		snap := g.snapshotLocked()
		g.mu.Unlock()
		return snap, nil
	}

	g.ready = false
	g.state = "needs_browser_login"
	g.message = firstNonEmptyString(
		probeMessage,
		"Claude Code login is required. Run `claude login`, complete browser sign-in, then click Done.",
	)
	g.updatedAt = time.Now().UTC()
	shouldStartLogin = true
	g.mu.Unlock()

	if shouldStartLogin {
		return g.StartDeviceAuth(ctx)
	}
	snap, _ := g.Status(context.Background())
	return snap, nil
}

func (g *claudeAuthGate) Configure(_ context.Context, _ string) (hubui.AgentAuthState, error) {
	status, _ := g.Status(context.Background())
	return status, fmt.Errorf("claude auth does not support manual config submission")
}

func (g *claudeAuthGate) snapshotLocked() hubui.AgentAuthState {
	state := strings.TrimSpace(g.state)
	if state == "" {
		if g.ready {
			state = "ready"
		} else {
			state = "needs_browser_login"
		}
	}
	authURL := strings.TrimSpace(g.authURL)
	if authURL == "" && !g.ready {
		authURL = claudeAuthDocsURL
	}
	return hubui.AgentAuthState{
		Harness:   agentruntime.HarnessClaude,
		Required:  g.required,
		Ready:     g.ready,
		State:     state,
		Message:   strings.TrimSpace(g.message),
		AuthURL:   authURL,
		UpdatedAt: g.updatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (g *claudeAuthGate) probe() (bool, string) {
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

	return false, "Claude Code login is required. Run `claude login`, complete browser sign-in, then click Done."
}

func (g *claudeAuthGate) readLoginStream(r io.ReadCloser) {
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		g.ingestLoginLine(scanner.Text())
	}
}

func (g *claudeAuthGate) ingestLoginLine(line string) {
	if g == nil {
		return
	}

	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return
	}

	authURL := extractClaudeAuthURL(line)
	promptAdvance := shouldAdvanceClaudeLoginPrompt(line)

	var input io.WriteCloser
	var update bool

	g.mu.Lock()
	if authURL != "" {
		g.authURL = authURL
		if !g.ready {
			g.state = "pending_browser_login"
			g.message = "Open the Claude login URL, complete sign-in, then click Done."
		}
		update = true
	}
	if promptAdvance && g.procRunning && g.procInput != nil {
		now := time.Now().UTC()
		if now.Sub(g.lastPromptAdvance) >= 250*time.Millisecond {
			g.lastPromptAdvance = now
			input = g.procInput
			update = true
		}
	}
	if update {
		g.updatedAt = time.Now().UTC()
	}
	g.mu.Unlock()

	if input != nil {
		if _, err := io.WriteString(input, "\n"); err != nil {
			g.logf("hub.auth status=warn harness=claude action=advance_login_prompt err=%q", err)
		}
	}
}

func (g *claudeAuthGate) waitLogin(cmd *exec.Cmd, tempDir string) {
	err := cmd.Wait()
	_ = os.RemoveAll(strings.TrimSpace(tempDir))

	g.mu.Lock()
	procInput := g.procInput
	g.procRunning = false
	g.procCancel = nil
	g.procInput = nil
	if procInput != nil {
		_ = procInput.Close()
	}

	if g.ready {
		g.mu.Unlock()
		return
	}
	if g.baseCtx != nil && g.baseCtx.Err() != nil {
		g.mu.Unlock()
		return
	}

	if err == nil {
		ready, probeMessage := g.probe()
		if ready {
			g.ready = true
			g.state = "ready"
			g.message = firstNonEmptyString(probeMessage, "Claude Code authorization is ready.")
		} else {
			g.ready = false
			g.state = "needs_browser_login"
			g.message = firstNonEmptyString(
				probeMessage,
				"Claude login process ended before authorization was detected. Click Done to retry.",
			)
		}
		g.updatedAt = time.Now().UTC()
		g.mu.Unlock()
		return
	}

	if strings.TrimSpace(g.authURL) == "" {
		g.state = "needs_browser_login"
		g.message = "Claude login did not provide a browser URL. Click Done to retry."
	} else {
		g.state = "pending_browser_login"
		g.message = "Complete Claude browser sign-in, then click Done."
	}
	g.updatedAt = time.Now().UTC()
	g.mu.Unlock()
}

func extractClaudeAuthURL(line string) string {
	match := claudeAuthURLPattern.FindString(strings.TrimSpace(stripANSI(line)))
	if strings.TrimSpace(match) == "" {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(match), ".,);]}>")
}

func shouldAdvanceClaudeLoginPrompt(line string) bool {
	text := strings.ToLower(strings.TrimSpace(stripANSI(line)))
	if text == "" {
		return false
	}

	for _, marker := range []string{
		"select",
		"choose",
		"press enter",
		"enter to continue",
		"which account",
		"which option",
		"pick an option",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
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
