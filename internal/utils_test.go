package internal_test

import (
	"os"
	"time"
)

type mockDirEntry struct {
	name string
}

func NewMockDirEntry(name string) *mockDirEntry {
	return &mockDirEntry{name: name}
}

func (m *mockDirEntry) Name() string {
	return m.name
}

func (m *mockDirEntry) IsDir() bool {
	return false
}

func (m *mockDirEntry) Type() os.FileMode {
	return os.ModeIrregular
}

func (m *mockDirEntry) Info() (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

type mockNewFileInfo struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func NewMockFileInfo(
	name string,
	mode os.FileMode,
	isDir bool,
) *mockNewFileInfo {
	return &mockNewFileInfo{
		name:  name,
		mode:  mode,
		isDir: isDir,
	}
}

func (m *mockNewFileInfo) Name() string {
	return m.name
}

func (m *mockNewFileInfo) Size() int64 {
	return 0
}

func (m *mockNewFileInfo) Mode() os.FileMode {
	return m.mode
}

func (m *mockNewFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (m *mockNewFileInfo) IsDir() bool {
	return m.isDir
}

func (m *mockNewFileInfo) Sys() any {
	return nil
}
