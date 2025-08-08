package internal

import (
	"log/slog"
	"path/filepath"
)

// Workspace is an interface that provides methods to interact with the workspace.
type Workspace interface {
	// GetGoBinPath returns the Go binary path.
	GetGoBinPath() string
	// GetInternalBasePath returns the internal base directory.
	GetInternalBasePath() string
	// GetInternalBinPath returns the internal binary directory.
	GetInternalBinPath() string
	// GetInternalTempPath returns the internal temporary directory.
	GetInternalTempPath() string
}

// defaultWorkspace is the default implementation of the Workspace interface.
type defaultWorkspace struct {
	goBinPath        string
	internalBasePath string
	internalBinPath  string
	internalTempPath string
	system           System
}

// NewWorkspace creates a new workspace. It returns an error if the workspace
// initialization fails.
func NewWorkspace(system System) (Workspace, error) {
	ws := &defaultWorkspace{system: system}

	if err := ws.init(); err != nil {
		return nil, err
	}

	return ws, nil
}

// GetGoBinPath returns the Go binary path.
func (w *defaultWorkspace) GetGoBinPath() string {
	return w.goBinPath
}

// GetBaseDir returns the base directory.
func (w *defaultWorkspace) GetInternalBasePath() string {
	return w.internalBasePath
}

// GetBinDir returns the binary directory.
func (w *defaultWorkspace) GetInternalBinPath() string {
	return w.internalBinPath
}

// GetTempPath returns the temporary directory.
func (w *defaultWorkspace) GetInternalTempPath() string {
	return w.internalTempPath
}

// init initializes the workspace. It creates the base, binary, and temporary
// directories. It returns an error if the directories cannot be created.
func (w *defaultWorkspace) init() error {
	homeDir, err := w.system.UserHomeDir()
	if err != nil {
		slog.Default().Error("failed to get user home directory", "err", err)
		return err
	}

	w.loadGoBinPath(homeDir)
	w.loadInternalPaths(homeDir)

	for _, dir := range []string{w.internalBasePath, w.internalBinPath, w.internalTempPath} {
		if err = w.system.MkdirAll(dir, dirPerm); err != nil {
			slog.Default().Error("failed to create directory", "dir", dir, "err", err)
			return err
		}
	}

	return nil
}

// loadGoBinPath loads the Go binary path.
func (w *defaultWorkspace) loadGoBinPath(homeDir string) {
	if gobin, ok := w.system.GetEnvVar("GOBIN"); ok {
		w.goBinPath = gobin
		return
	}

	if gopath, ok := w.system.GetEnvVar("GOPATH"); ok {
		w.goBinPath = filepath.Join(gopath, "bin")
		return
	}

	w.goBinPath = filepath.Join(homeDir, "go", "bin")
}

// loadInternalPaths loads the internal paths.
func (w *defaultWorkspace) loadInternalPaths(homeDir string) {
	var baseDir, binDir, tmpDir string
	switch w.system.RuntimeOS() {
	case windowsOS:
		baseDir = filepath.Join(homeDir, "AppData", "Local", "gobin")
		binDir = filepath.Join(baseDir, "bin")
		tmpDir = filepath.Join(baseDir, "tmp")
	default:
		baseDir = filepath.Join(homeDir, ".gobin")
		binDir = filepath.Join(baseDir, "bin")
		tmpDir = filepath.Join(baseDir, ".tmp")
	}

	w.internalBasePath = baseDir
	w.internalBinPath = binDir
	w.internalTempPath = tmpDir
}
