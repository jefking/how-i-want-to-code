package workspace

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultRAMBase  = "/dev/shm"
	defaultDiskBase = "/tmp"
	defaultRunRoot  = "temp"
	agentsSeedPath  = "library/AGENTS.md"
	agentsFileName  = "AGENTS.md"
	agentsSeedEnv   = "HARNESS_AGENTS_SEED_PATH"
)

// Manager creates isolated run directories.
type Manager struct {
	PathExists func(string) bool
	MkdirAll   func(string, os.FileMode) error
	NewGUID    func() string
	ReadFile   func(string) ([]byte, error)
	WriteFile  func(string, []byte, os.FileMode) error
	CanExec    func(string) bool
}

// NewManager returns a manager backed by os functions.
func NewManager() Manager {
	return Manager{
		PathExists: pathExists,
		MkdirAll:   os.MkdirAll,
		NewGUID:    newGUID,
		ReadFile:   os.ReadFile,
		WriteFile:  os.WriteFile,
		CanExec:    baseAllowsExec,
	}
}

// SelectBase chooses /dev/shm when available and executable, else /tmp.
func (m Manager) SelectBase() string {
	exists := m.PathExists
	if exists == nil {
		exists = pathExists
	}
	canExec := m.CanExec
	if canExec == nil {
		canExec = baseAllowsExec
	}
	if exists(defaultRAMBase) && canExec(defaultRAMBase) {
		return defaultRAMBase
	}
	return defaultDiskBase
}

// CreateRunDir creates a GUID-named run directory under <base>/temp.
func (m Manager) CreateRunDir() (string, string, error) {
	mkdirAll := m.MkdirAll
	if mkdirAll == nil {
		mkdirAll = os.MkdirAll
	}
	guidFn := m.NewGUID
	if guidFn == nil {
		guidFn = newGUID
	}

	guid := guidFn()
	if guid == "" {
		return "", "", fmt.Errorf("generated empty guid")
	}

	preferredBase := m.SelectBase()
	fallbackBase := defaultDiskBase
	if preferredBase == defaultDiskBase {
		fallbackBase = ""
	}

	candidates := []string{preferredBase}
	if fallbackBase != "" && fallbackBase != preferredBase {
		candidates = append(candidates, fallbackBase)
	}

	var lastErr error
	for _, base := range candidates {
		runDir := filepath.Join(base, defaultRunRoot, guid)
		if err := mkdirAll(runDir, 0o755); err != nil {
			lastErr = err
			continue
		}
		return runDir, guid, nil
	}

	if lastErr != nil {
		return "", "", fmt.Errorf("create run dir: %w", lastErr)
	}
	return "", "", fmt.Errorf("create run dir: no workspace base candidates")
}

// SeedAgentsFile copies the local library AGENTS seed into a run directory.
func (m Manager) SeedAgentsFile(runDir string) (string, error) {
	readFile := m.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	writeFile := m.WriteFile
	if writeFile == nil {
		writeFile = os.WriteFile
	}

	content, err := readFile(agentsSeedPath)
	if err != nil {
		fallbackPath := resolveAgentsSeedPath()
		if fallbackPath != "" && fallbackPath != agentsSeedPath {
			content, err = readFile(fallbackPath)
		}
		if err != nil {
			return "", fmt.Errorf("read agents seed: %w", err)
		}
	}
	dst := filepath.Join(runDir, agentsFileName)
	if err := writeFile(dst, content, 0o644); err != nil {
		return "", fmt.Errorf("write agents seed: %w", err)
	}
	return dst, nil
}

func pathExists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

func baseAllowsExec(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return true
	}
	if runtime.GOOS != "linux" {
		return true
	}
	opts, ok := mountOptionsForPath(path)
	if !ok {
		// If mount details are unavailable, keep existing behavior.
		return true
	}
	_, noExec := opts["noexec"]
	return !noExec
}

func mountOptionsForPath(targetPath string) (map[string]struct{}, bool) {
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if targetPath == "" {
		return nil, false
	}
	if opts, ok := mountOptionsFromMountInfo("/proc/self/mountinfo", targetPath); ok {
		return opts, true
	}
	if opts, ok := mountOptionsFromProcMounts("/proc/mounts", targetPath); ok {
		return opts, true
	}
	return nil, false
}

func mountOptionsFromMountInfo(sourcePath, targetPath string) (map[string]struct{}, bool) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	bestMatchLen := -1
	var bestOptions map[string]struct{}

	for scanner.Scan() {
		mountPoint, optionsCSV, ok := parseMountInfoLine(scanner.Text())
		if !ok || !pathWithinMount(targetPath, mountPoint) {
			continue
		}
		if len(mountPoint) > bestMatchLen {
			bestMatchLen = len(mountPoint)
			bestOptions = parseMountOptions(optionsCSV)
		}
	}
	if bestMatchLen >= 0 {
		return bestOptions, true
	}
	return nil, false
}

func mountOptionsFromProcMounts(sourcePath, targetPath string) (map[string]struct{}, bool) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	bestMatchLen := -1
	var bestOptions map[string]struct{}

	for scanner.Scan() {
		mountPoint, optionsCSV, ok := parseProcMountsLine(scanner.Text())
		if !ok || !pathWithinMount(targetPath, mountPoint) {
			continue
		}
		if len(mountPoint) > bestMatchLen {
			bestMatchLen = len(mountPoint)
			bestOptions = parseMountOptions(optionsCSV)
		}
	}
	if bestMatchLen >= 0 {
		return bestOptions, true
	}
	return nil, false
}

func parseMountInfoLine(line string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), " - ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	left := strings.Fields(parts[0])
	right := strings.Fields(parts[1])
	if len(left) < 6 || len(right) < 3 {
		return "", "", false
	}

	mountPoint := unescapeMountField(left[4])
	options := strings.TrimSpace(left[5])
	superOptions := strings.TrimSpace(right[2])
	if superOptions != "" {
		if options != "" {
			options += ","
		}
		options += superOptions
	}
	if mountPoint == "" {
		return "", "", false
	}
	return mountPoint, options, true
}

func parseProcMountsLine(line string) (string, string, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 {
		return "", "", false
	}
	mountPoint := unescapeMountField(fields[1])
	if mountPoint == "" {
		return "", "", false
	}
	return mountPoint, strings.TrimSpace(fields[3]), true
}

func parseMountOptions(csv string) map[string]struct{} {
	opts := make(map[string]struct{})
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		opts[part] = struct{}{}
	}
	return opts
}

func pathWithinMount(path, mountPoint string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	mountPoint = filepath.Clean(strings.TrimSpace(mountPoint))
	if path == "" || mountPoint == "" {
		return false
	}
	if mountPoint == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == mountPoint || strings.HasPrefix(path, mountPoint+"/")
}

func unescapeMountField(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || !strings.Contains(v, `\`) {
		return v
	}
	var b strings.Builder
	b.Grow(len(v))
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' || i+3 >= len(v) || !isOctal(v[i+1]) || !isOctal(v[i+2]) || !isOctal(v[i+3]) {
			b.WriteByte(v[i])
			continue
		}
		decoded := (v[i+1]-'0')*64 + (v[i+2]-'0')*8 + (v[i+3] - '0')
		b.WriteByte(decoded)
		i += 3
	}
	return b.String()
}

func isOctal(b byte) bool {
	return b >= '0' && b <= '7'
}

func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func resolveAgentsSeedPath() string {
	if configured := strings.TrimSpace(os.Getenv(agentsSeedEnv)); configured != "" {
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if path, ok := findPathUpward(wd, agentsSeedPath); ok {
			return path
		}
	}
	if exePath, err := os.Executable(); err == nil {
		if path, ok := findPathUpward(filepath.Dir(exePath), agentsSeedPath); ok {
			return path
		}
	}
	return ""
}

func findPathUpward(startDir, relPath string) (string, bool) {
	startDir = strings.TrimSpace(startDir)
	relPath = strings.TrimSpace(relPath)
	if startDir == "" || relPath == "" {
		return "", false
	}

	current := filepath.Clean(startDir)
	for {
		candidate := filepath.Join(current, relPath)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}
