package system

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

// workspace is the default implementation of the Workspace interface.
type workspace struct {
	goBinPath        string
	internalBasePath string
	internalBinPath  string
	internalTempPath string

	env     Environment
	fs      FileSystem
	runtime Runtime
}

// NewWorkspace creates a new workspace. It returns an error if the workspace
// initialization fails.
func NewWorkspace(
	env Environment,
	fs FileSystem,
	runtime Runtime,
) (Workspace, error) {
	ws := &workspace{
		env:     env,
		fs:      fs,
		runtime: runtime,
	}

	if err := ws.init(); err != nil {
		return nil, err
	}

	return ws, nil
}

// GetGoBinPath returns the Go binary path.
func (w *workspace) GetGoBinPath() string {
	return w.goBinPath
}

// GetBaseDir returns the base directory.
func (w *workspace) GetInternalBasePath() string {
	return w.internalBasePath
}

// GetBinDir returns the binary directory.
func (w *workspace) GetInternalBinPath() string {
	return w.internalBinPath
}

// GetTempPath returns the temporary directory.
func (w *workspace) GetInternalTempPath() string {
	return w.internalTempPath
}

// init initializes the workspace. It creates the base, binary, and temporary
// directories. It returns an error if the directories cannot be created.
func (w *workspace) init() error {
	homeDir, err := w.env.UserHomeDir()
	if err != nil {
		slog.Default().Error("failed to get user home directory", "err", err)
		return err
	}

	w.loadGoBinPath(homeDir)
	w.loadInternalPaths(homeDir)

	for _, dir := range []string{w.internalBasePath, w.internalBinPath, w.internalTempPath} {
		//nolint:mnd // owner only permissions
		if err = w.fs.CreateDir(dir, 0700); err != nil {
			slog.Default().Error("failed to create directory", "dir", dir, "err", err)
			return err
		}
	}

	return nil
}

// loadGoBinPath loads the Go binary path.
func (w *workspace) loadGoBinPath(homeDir string) {
	if gobin, ok := w.env.Get("GOBIN"); ok {
		w.goBinPath = gobin
		return
	}

	if gopath, ok := w.env.Get("GOPATH"); ok {
		w.goBinPath = filepath.Join(gopath, "bin")
		return
	}

	w.goBinPath = filepath.Join(homeDir, "go", "bin")
}

// loadInternalPaths loads the internal paths.
func (w *workspace) loadInternalPaths(homeDir string) {
	var baseDir, binDir, tmpDir string
	switch w.runtime.OS() {
	case "windows": //nolint:goconst // windows is a valid OS
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
