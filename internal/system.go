package internal

import (
	"debug/buildinfo"
	"os"
	"os/exec"
	"runtime"
)

// System is an interface that provides methods to interact with the system.
type System interface {
	// GetEnvVar returns the value of an environment variable.
	GetEnvVar(key string) (string, bool)
	// LookPath returns the path to an executable file in the PATH environment variable.
	LookPath(file string) (string, error)
	// LStat returns the FileInfo structure describing the named file.
	LStat(name string) (os.FileInfo, error)
	// MkdirAll creates a directory named path, along with any necessary parents,
	// with mode perm.
	MkdirAll(path string, perm os.FileMode) error
	// MkdirTemp creates a new temporary directory.
	MkdirTemp(dir, pattern string) (string, error)
	// PathListSeparator returns the OS-specific path list separator.
	PathListSeparator() rune
	// ReadBuildInfo reads build information from a Go binary file.
	ReadBuildInfo(path string) (*buildinfo.BuildInfo, error)
	// ReadDir reads the directory named by dirname and returns a list of
	// directory entries.
	ReadDir(dirname string) ([]os.DirEntry, error)
	// Readlink returns the target of a symbolic link.
	Readlink(name string) (string, error)
	// Remove removes the named file or (empty) directory.
	Remove(name string) error
	// RemoveAll removes path and any children it contains.
	RemoveAll(path string) error
	// Rename renames (moves) oldpath to newpath.
	Rename(oldpath, newpath string) error
	// RuntimeARCH returns the architecture of the current runtime.
	RuntimeARCH() string
	// RuntimeOS returns the operating system of the current runtime.
	RuntimeOS() string
	// RuntimeVersion returns the version of the Go runtime.
	RuntimeVersion() string
	// Stat returns the FileInfo structure describing file.
	Stat(name string) (os.FileInfo, error)
	// Symlink creates newname as a symbolic link to oldname.
	Symlink(oldname, newname string) error
	// UserHomeDir returns the current user's home directory.
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

// LStat returns the FileInfo structure describing the named file.
func (s *defaultSystem) LStat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

// MkdirAll creates a directory named path, along with any necessary parents,
// with mode perm.
func (s *defaultSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// MkdirTemp creates a new temporary directory in the directory dir
// and returns the pathname of the new directory.
func (s *defaultSystem) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
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

// Readlink returns the target of a symbolic link.
func (s *defaultSystem) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

// Remove removes the named file or (empty) directory.
func (s *defaultSystem) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
func (s *defaultSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames (moves) oldpath to newpath.
func (s *defaultSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
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

// Symlink creates newname as a symbolic link to oldname.
func (s *defaultSystem) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

// UserHomeDir returns the current user's home directory.
func (s *defaultSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
