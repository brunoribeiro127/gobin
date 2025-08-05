package internal

import (
	"debug/buildinfo"
	"os"
	"os/exec"
	"runtime"
)

type System interface {
	GetEnvVar(key string) (string, bool)
	LookPath(file string) (string, error)
	PathListSeparator() rune
	ReadBuildInfo(path string) (*buildinfo.BuildInfo, error)
	ReadDir(dirname string) ([]os.DirEntry, error)
	RuntimeARCH() string
	RuntimeOS() string
	RuntimeVersion() string
	Stat(name string) (os.FileInfo, error)
	UserHomeDir() (string, error)
}

type defaultSystem struct{}

func NewSystem() System {
	return &defaultSystem{}
}

func (s *defaultSystem) GetEnvVar(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (s *defaultSystem) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (s *defaultSystem) PathListSeparator() rune {
	return os.PathListSeparator
}

func (s *defaultSystem) ReadBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	return buildinfo.ReadFile(path)
}

func (s *defaultSystem) ReadDir(dirname string) ([]os.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (s *defaultSystem) RuntimeARCH() string {
	return runtime.GOARCH
}

func (s *defaultSystem) RuntimeOS() string {
	return runtime.GOOS
}

func (s *defaultSystem) RuntimeVersion() string {
	return runtime.Version()
}

func (s *defaultSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (s *defaultSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
