package internal

import (
	"debug/buildinfo"
	"os"
	"os/exec"
	"runtime"
)

// System is an interface that provides methods to interact with the system.
type System interface {
	GetEnvVar(key string) (string, bool)
	LookPath(file string) (string, error)
	PathListSeparator() rune
	ReadBuildInfo(path string) (*buildinfo.BuildInfo, error)
	ReadDir(dirname string) ([]os.DirEntry, error)
	Remove(name string) error
	RuntimeARCH() string
	RuntimeOS() string
	RuntimeVersion() string
	Stat(name string) (os.FileInfo, error)
	UserHomeDir() (string, error)
}

// defaultSystem is the default implementation of the System interface.
type defaultSystem struct{}

// NewSystem creates a new System to interact with the system.
func NewSystem() System {
	return &defaultSystem{}
}

// GetEnvVar returns the value of an environment variable.
func (s *defaultSystem) GetEnvVar(key string) (string, bool) {
	return os.LookupEnv(key)
}

// LookPath returns the path to an executable file in the PATH environment variable.
func (s *defaultSystem) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// PathListSeparator returns the OS-specific path list separator for the PATH
// environment variable.
func (s *defaultSystem) PathListSeparator() rune {
	return os.PathListSeparator
}

// ReadBuildInfo returns build information embedded in a Go binary file at the given path.
func (s *defaultSystem) ReadBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	return buildinfo.ReadFile(path)
}

// ReadDir reads the directory named by dirname and returns a list of directory entries.
func (s *defaultSystem) ReadDir(dirname string) ([]os.DirEntry, error) {
	return os.ReadDir(dirname)
}

// Remove removes the named file or (empty) directory.
func (s *defaultSystem) Remove(name string) error {
	return os.Remove(name)
}

// RuntimeARCH returns the architecture for the current runtime.
func (s *defaultSystem) RuntimeARCH() string {
	return runtime.GOARCH
}

// RuntimeOS returns the operating system for the current runtime.
func (s *defaultSystem) RuntimeOS() string {
	return runtime.GOOS
}

// RuntimeVersion returns the Go version of the current runtime.
func (s *defaultSystem) RuntimeVersion() string {
	return runtime.Version()
}

// Stat returns a FileInfo describing the named file.
func (s *defaultSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// UserHomeDir returns the current user's home directory.
func (s *defaultSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
