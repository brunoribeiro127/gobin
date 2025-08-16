package model

import (
	"strconv"
	"strings"
)

// Module represents a module.
type Module struct {
	Path    string
	Version Version
}

// NewModule creates a new module from a module path and version.
func NewModule(path string, version Version) Module {
	return Module{
		Path:    path,
		Version: version,
	}
}

// NewLatestModule creates a new module with the given path and version "latest".
func NewLatestModule(path string) Module {
	return NewModule(path, NewLatestVersion())
}

// GetBaseModule returns the base module path. It returns the module path
// without the version suffix.
func (m Module) GetBaseModule() string {
	baseModule, _ := getBaseModuleAndVersionSuffix(m.Path)
	return baseModule
}

// IsValid checks if the module is valid. A module is valid if it has a
// non-empty path and a valid version.
func (m Module) IsValid() bool {
	return strings.TrimSpace(m.Path) != "" && m.Version.IsValid()
}

// NextMajorModule returns the next major module. It returns the latest module
// with the base module path and the next major version.
func (m Module) NextMajorModule() Module {
	baseModule, versionSuffix := getBaseModuleAndVersionSuffix(m.Path)

	if versionSuffix == "" {
		return NewLatestModule(baseModule + "/v2")
	}

	return NewLatestModule(baseModule + "/" + NewVersion(versionSuffix).NextMajorVersion().String())
}

// String returns the string representation of the module.
func (m Module) String() string {
	return m.Path + "@" + m.Version.String()
}

// getBaseModuleAndVersionSuffix gets the base module path and the version suffix
// from a module path. If the module path does not have a version suffix, it
// returns the module path unchanged and an empty string.
func getBaseModuleAndVersionSuffix(module string) (string, string) {
	parts := strings.Split(module, "/")
	lastPart := parts[len(parts)-1]

	if strings.HasPrefix(lastPart, "v") {
		if _, err := strconv.Atoi(lastPart[1:]); err == nil {
			return strings.Join(parts[:len(parts)-1], "/"), lastPart
		}
	}

	return module, ""
}
