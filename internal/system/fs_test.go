package system_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunoribeiro127/gobin/internal/system"
)

func TestFileSystem_CreateDir(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := fs.CreateDir(filepath.Join(tempDir, "test"), 0700)
	require.NoError(t, err)

	stat, err := os.Stat(filepath.Join(tempDir, "test"))
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}

func TestFileSystem_CreateTempDir(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	tempDir, cleanup, err := fs.CreateTempDir(tempDir, "test")
	require.NoError(t, err)

	stat, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	err = cleanup()
	require.NoError(t, err)

	_, err = os.Stat(tempDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileSystem_IsSymlinkToDir(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "file")
	link := filepath.Join(tempDir, "link")

	err := os.WriteFile(file, []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(file, link)
	require.NoError(t, err)

	isSymlink, err := fs.IsSymlinkToDir(link, tempDir)
	require.NoError(t, err)
	assert.True(t, isSymlink)

	isSymlink, err = fs.IsSymlinkToDir(file, tempDir)
	require.NoError(t, err)
	assert.False(t, isSymlink)
}

func TestFileSystem_ListBinaries(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempDir, "dir"), 0700)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		err = os.WriteFile(filepath.Join(tempDir, "bin.exe"), []byte{'M', 'Z'}, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tempDir, "file"), []byte{}, 0644)
		require.NoError(t, err)
	} else {
		err = os.WriteFile(filepath.Join(tempDir, "bin"), []byte{}, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tempDir, "file"), []byte{}, 0644)
		require.NoError(t, err)
	}

	binaries, err := fs.ListBinaries(tempDir)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Equal(t, []string{filepath.Join(tempDir, "bin.exe")}, binaries)
	} else {
		assert.Equal(t, []string{filepath.Join(tempDir, "bin")}, binaries)
	}
}

func TestFileSystem_LocateBinaryInPath(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	if runtime.GOOS == "windows" {
		err := os.WriteFile(filepath.Join(tempDir, "bin.exe"), []byte{'M', 'Z'}, 0755)
		require.NoError(t, err)
	} else {
		err := os.WriteFile(filepath.Join(tempDir, "bin"), []byte{}, 0755)
		require.NoError(t, err)
	}

	t.Setenv("PATH", tempDir)

	if runtime.GOOS == "windows" {
		binaries := fs.LocateBinaryInPath("bin.exe")
		assert.Equal(t, []string{filepath.Join(tempDir, "bin.exe")}, binaries)
	} else {
		binaries := fs.LocateBinaryInPath("bin")
		assert.Equal(t, []string{filepath.Join(tempDir, "bin")}, binaries)
	}
}

func TestFileSystem_Move(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempDir, "dir1"), 0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "dir1", "bin"), []byte{}, 0755)
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(tempDir, "dir2"), 0700)
	require.NoError(t, err)

	err = fs.Move(filepath.Join(tempDir, "dir1", "bin"), filepath.Join(tempDir, "dir2", "bin"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "dir1", "bin"))
	require.ErrorIs(t, err, os.ErrNotExist)

	stat, err := os.Stat(filepath.Join(tempDir, "dir2", "bin"))
	require.NoError(t, err)
	assert.True(t, stat.Mode().IsRegular())
}

func TestFileSystem_MoveWithSymlink(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempDir, "dir1"), 0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "dir1", "bin"), []byte{}, 0755)
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(tempDir, "dir2"), 0700)
	require.NoError(t, err)

	err = fs.MoveWithSymlink(filepath.Join(tempDir, "dir1", "bin"), filepath.Join(tempDir, "dir2", "bin"))
	require.NoError(t, err)

	info, err := os.Lstat(filepath.Join(tempDir, "dir1", "bin"))
	require.NoError(t, err)
	require.NotEqual(t, 0, info.Mode()&os.ModeSymlink)

	info, err = os.Stat(filepath.Join(tempDir, "dir2", "bin"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}

func TestFileSystem_Remove(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempDir, "dir"), 0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "dir", "bin"), []byte{}, 0755)
	require.NoError(t, err)

	err = fs.Remove(filepath.Join(tempDir, "dir", "bin"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "dir", "bin"))
	require.ErrorIs(t, err, os.ErrNotExist)

	err = fs.Remove(filepath.Join(tempDir, "dir"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "dir"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileSystem_ReplaceSymlink(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "bin1"), []byte{}, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "bin2"), []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(filepath.Join(tempDir, "bin1"), filepath.Join(tempDir, "bin"))
	require.NoError(t, err)

	err = fs.ReplaceSymlink(filepath.Join(tempDir, "bin2"), filepath.Join(tempDir, "bin"))
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(tempDir, "bin"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tempDir, "bin2"), target)
}

func TestFileSystem_GetSymlinkTarget(t *testing.T) {
	fs := system.NewFileSystem()

	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "bin1"), []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(filepath.Join(tempDir, "bin1"), filepath.Join(tempDir, "bin"))
	require.NoError(t, err)

	target, err := fs.GetSymlinkTarget(filepath.Join(tempDir, "bin"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tempDir, "bin1"), target)
}
