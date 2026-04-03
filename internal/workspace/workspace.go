package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultRAMBase  = "/dev/shm"
	defaultDiskBase = "/tmp"
	defaultRunRoot  = "temp"
)

// Manager creates isolated run directories.
type Manager struct {
	PathExists func(string) bool
	MkdirAll   func(string, os.FileMode) error
	NewGUID    func() string
}

// NewManager returns a manager backed by os functions.
func NewManager() Manager {
	return Manager{
		PathExists: pathExists,
		MkdirAll:   os.MkdirAll,
		NewGUID:    newGUID,
	}
}

// SelectBase chooses /dev/shm when available, else /tmp.
func (m Manager) SelectBase() string {
	exists := m.PathExists
	if exists == nil {
		exists = pathExists
	}
	if exists(defaultRAMBase) {
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

	runDir := filepath.Join(m.SelectBase(), defaultRunRoot, guid)
	if err := mkdirAll(runDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create run dir: %w", err)
	}
	return runDir, guid, nil
}

func pathExists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
