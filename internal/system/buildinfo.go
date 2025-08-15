package system

import "debug/buildinfo"

// BuildInfo is the interface for reading build info.
type BuildInfo interface {
	// Read reads the build info from the given path.
	Read(path string) (*buildinfo.BuildInfo, error)
}

// buildInfo is the implementation of the BuildInfo interface.
type buildInfo struct{}

// NewBuildInfo creates a new build info.
func NewBuildInfo() BuildInfo {
	return &buildInfo{}
}

// Read reads the build info from the given path.
func (b *buildInfo) Read(path string) (*buildinfo.BuildInfo, error) {
	return buildinfo.ReadFile(path)
}
