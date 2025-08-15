package system_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunoribeiro127/gobin/internal/system"
)

func TestFileSystem_CreateDir(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := fs.CreateDir(tempDir+"/test", 0700)
	require.NoError(t, err)

	stat, err := os.Stat(tempDir + "/test")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), stat.Mode().Perm())
}

func TestFileSystem_CreateTempDir(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	tempDir, cleanup, err := fs.CreateTempDir(tempDir, "test")
	require.NoError(t, err)

	stat, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), stat.Mode().Perm())

	err = cleanup()
	require.NoError(t, err)

	_, err = os.Stat(tempDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileSystem_IsSymlinkToDir(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()
	testfile := tempDir + "/testfile"
	testlink := tempDir + "testlink"

	err := os.WriteFile(testfile, []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(testfile, testlink)
	require.NoError(t, err)

	isSymlink, err := fs.IsSymlinkToDir(testlink, tempDir)
	require.NoError(t, err)
	assert.True(t, isSymlink)
}

func TestFileSystem_ListBinaries(t *testing.T) {
	fs := system.NewFileSystem(system.NewRuntime())

	tempDir := t.TempDir()

	err := os.Mkdir(tempDir+"/dir", 0700)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		err = os.WriteFile(tempDir+"/bin.exe", []byte{}, 0755)
		require.NoError(t, err)
	} else {
		err = os.WriteFile(tempDir+"/bin", []byte{}, 0755)
		require.NoError(t, err)
	}

	binaries, err := fs.ListBinaries(tempDir)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Equal(t, []string{tempDir + "/bin.exe"}, binaries)
	} else {
		assert.Equal(t, []string{tempDir + "/bin"}, binaries)
	}
}

func TestFileSystem_LocateBinaryInPath(t *testing.T) {
	fs := system.NewFileSystem(system.NewRuntime())

	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/bin", []byte{}, 0755)
	require.NoError(t, err)

	t.Setenv("PATH", tempDir)

	binaries := fs.LocateBinaryInPath("bin")
	require.Equal(t, []string{tempDir + "/bin"}, binaries)
}

func TestFileSystem_Move(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := os.Mkdir(tempDir+"/dir1", 0700)
	require.NoError(t, err)

	err = os.WriteFile(tempDir+"/dir1/bin", []byte{}, 0755)
	require.NoError(t, err)

	err = os.Mkdir(tempDir+"/dir2", 0700)
	require.NoError(t, err)

	err = fs.Move(tempDir+"/dir1/bin", tempDir+"/dir2/bin")
	require.NoError(t, err)

	_, err = os.Stat(tempDir + "/dir1/bin")
	require.ErrorIs(t, err, os.ErrNotExist)

	stat, err := os.Stat(tempDir + "/dir2/bin")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), stat.Mode().Perm())
}

func TestFileSystem_MoveWithSymlink(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := os.Mkdir(tempDir+"/dir1", 0700)
	require.NoError(t, err)

	err = os.WriteFile(tempDir+"/dir1/bin", []byte{}, 0755)
	require.NoError(t, err)

	err = os.Mkdir(tempDir+"/dir2", 0700)
	require.NoError(t, err)

	err = fs.MoveWithSymlink(tempDir+"/dir2/bin", tempDir+"/dir1/bin")
	require.NoError(t, err)

	info, err := os.Lstat(tempDir + "/dir1/bin")
	require.NoError(t, err)
	require.NotEqual(t, 0, info.Mode()&os.ModeSymlink)

	info, err = os.Stat(tempDir + "/dir2/bin")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestFileSystem_Remove(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := os.Mkdir(tempDir+"/dir", 0700)
	require.NoError(t, err)

	err = os.WriteFile(tempDir+"/dir/bin", []byte{}, 0755)
	require.NoError(t, err)

	err = fs.Remove(tempDir + "/dir/bin")
	require.NoError(t, err)

	_, err = os.Stat(tempDir + "/dir/bin")
	require.ErrorIs(t, err, os.ErrNotExist)

	err = fs.Remove(tempDir + "/dir")
	require.NoError(t, err)

	_, err = os.Stat(tempDir + "/dir")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileSystem_ReplaceSymlink(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := os.WriteFile(tempDir+"/bin1", []byte{}, 0755)
	require.NoError(t, err)

	err = os.WriteFile(tempDir+"/bin2", []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(tempDir+"/bin1", tempDir+"/bin")
	require.NoError(t, err)

	err = fs.ReplaceSymlink(tempDir+"/bin2", tempDir+"/bin")
	require.NoError(t, err)

	target, err := os.Readlink(tempDir + "/bin")
	require.NoError(t, err)
	require.Equal(t, tempDir+"/bin2", target)
}

func TestFileSystem_GetSymlinkTarget(t *testing.T) {
	fs := system.NewFileSystem(nil)

	tempDir := t.TempDir()

	err := os.WriteFile(tempDir+"/bin1", []byte{}, 0755)
	require.NoError(t, err)

	err = os.Symlink(tempDir+"/bin1", tempDir+"/bin")
	require.NoError(t, err)

	target, err := fs.GetSymlinkTarget(tempDir + "/bin")
	require.NoError(t, err)
	require.Equal(t, tempDir+"/bin1", target)
}
