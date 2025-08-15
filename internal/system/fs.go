package system

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// CleanupFunc is a function that cleans up a resource.
type CleanupFunc func() error

// FileSystem is the interface for the file system.
type FileSystem interface {
	// CreateDir creates a directory with the given path and permissions.
	CreateDir(path string, perm os.FileMode) error
	// CreateTempDir creates a temporary directory with the given path and pattern.
	CreateTempDir(dir, pattern string) (string, CleanupFunc, error)
	// IsSymlinkToDir checks if a path is a symlink to another directory.
	IsSymlinkToDir(path string, baseDir string) (bool, error)
	// ListBinaries lists the binaries in a directory.
	ListBinaries(path string) ([]string, error)
	// LocateBinaryInPath locates a binary in the PATH environment variable.
	LocateBinaryInPath(name string) []string
	// Move moves a file or directory.
	Move(source, target string) error
	// MoveWithSymlink moves a file or directory with a symlink.
	MoveWithSymlink(source, target string) error
	// Remove removes a file or directory.
	Remove(path string) error
	// ReplaceSymlink replaces a symlink with a new source.
	ReplaceSymlink(source, target string) error
	// GetSymlinkTarget gets the target of a symlink.
	GetSymlinkTarget(path string) (string, error)
}

// fileSystem is the default implementation of the FileSystem interface.
type fileSystem struct {
	runtime Runtime
}

// NewFileSystem creates a new file system.
func NewFileSystem(
	runtime Runtime,
) FileSystem {
	return &fileSystem{
		runtime: runtime,
	}
}

// CreateDir creates a directory with the given path and permissions. It returns
// an error if the directory cannot be created.
func (fs *fileSystem) CreateDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// CreateTempDir creates a temporary directory with the given path and pattern.
// It returns the path to the temporary directory, a function to clean up the
// directory, and an error if the directory cannot be created.
func (fs *fileSystem) CreateTempDir(dir, pattern string) (string, CleanupFunc, error) {
	logger := slog.Default().With("dir", dir, "pattern", pattern)

	tempDir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		logger.Error("error while creating temp dir", "err", err)
		return "", nil, err
	}

	cleanup := func() error {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil {
			logger.Error("error while removing temp dir", "err", rmErr)
			return rmErr
		}

		return nil
	}

	return tempDir, cleanup, nil
}

// IsSymlinkToDir checks if a path is a symlink to another directory.
func (fs *fileSystem) IsSymlinkToDir(path string, baseDir string) (bool, error) {
	logger := slog.Default().With("path", path, "base_dir", baseDir)

	info, err := os.Lstat(path)
	if err != nil {
		logger.Error("error while getting symlink info", "err", err)
		return false, err
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}

	target, err := os.Readlink(path)
	if err != nil {
		logger.Error("error while reading symlink", "err", err)
		return false, err
	}

	return strings.HasPrefix(target, baseDir+string(os.PathSeparator)), nil
}

// ListBinaries lists the binaries in a directory. It returns an error if the
// directory cannot be read.
func (fs *fileSystem) ListBinaries(path string) ([]string, error) {
	logger := slog.Default().With("path", path)

	entries, err := os.ReadDir(path)
	if err != nil {
		logger.Error("error while listing directory", "err", err)
		return nil, err
	}

	binaries := make([]string, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if fs.isBinary(fullPath) {
			binaries = append(binaries, fullPath)
		}
	}

	return binaries, nil
}

// LocateBinaryInPath locates a binary in the PATH environment variable. It
// returns a list of full paths to the binary, or an empty list if the binary is
// not found.
func (fs *fileSystem) LocateBinaryInPath(name string) []string {
	locations := []string{}

	path, _ := os.LookupEnv("PATH")
	for dir := range strings.SplitSeq(path, string(filepath.ListSeparator)) {
		fullPath := filepath.Join(dir, name)
		if fs.isBinary(fullPath) {
			locations = append(locations, fullPath)
		}
	}

	return locations
}

// Move moves a file or directory. It returns an error if the file or directory
// cannot be moved.
func (fs *fileSystem) Move(source, target string) error {
	return os.Rename(source, target)
}

// MoveWithSymlink moves a file or directory with a symlink. It returns an
// error if the file or directory cannot be moved or the symlink cannot be
// created.
func (fs *fileSystem) MoveWithSymlink(source, target string) error {
	logger := slog.Default().With("source", source, "target", target)

	if err := os.Rename(target, source); err != nil {
		logger.Error("error while moving file", "err", err)
		return err
	}

	if err := os.Symlink(source, target); err != nil {
		logger.Error("error while creating symlink", "err", err)
		return err
	}

	return nil
}

// Remove removes a file or directory. It returns an error if the file or
// directory cannot be removed.
func (fs *fileSystem) Remove(path string) error {
	return os.Remove(path)
}

// ReplaceSymlink replaces a symlink with a new source. It returns an error if
// the symlink cannot be removed or created.
func (fs *fileSystem) ReplaceSymlink(source, target string) error {
	logger := slog.Default().With("source", source, "target", target)

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		logger.Error("error while removing symlink", "err", err)
		return err
	}

	if err := os.Symlink(source, target); err != nil {
		logger.Error("error while creating symlink", "err", err)
		return err
	}

	return nil
}

// GetSymlinkTarget gets the target of a symlink.
func (fs *fileSystem) GetSymlinkTarget(path string) (string, error) {
	return os.Readlink(path)
}

// isBinary checks if a path is a binary file. It returns true if the path is a
// regular file and executable for Unix, or if it is a Windows executable.
func (fs *fileSystem) isBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if fs.runtime.OS() == "windows" { //nolint:goconst,nolintlint
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}

	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}
