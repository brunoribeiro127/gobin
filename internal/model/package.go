package model

import (
	"strings"

	"golang.org/x/mod/semver"
)

// Package represents a package.
type Package struct {
	Path    string
	Version Version
}

// NewPackage creates a new package from a package version string. If the
// package does not contain a version, it defaults to "latest".
func NewPackage(pkg string) Package {
	path, version := pkg, "latest"

	//nolint:mnd // expected version format: path@version
	if parts := strings.Split(pkg, "@"); len(parts) == 2 {
		path, version = parts[0], parts[1]
	}

	return Package{
		Path:    path,
		Version: NewVersion(version),
	}
}

// NewPackageWithVersion creates a new package with the given path and version.
func NewPackageWithVersion(path string, version Version) Package {
	return Package{
		Path:    path,
		Version: version,
	}
}

// GetBinaryName returns the binary name of the package. It returns the last
// part of the package path, unless the last part is a major version. In that
// case, it returns the previous part of the path.
func (p Package) GetBinaryName() string {
	parts := strings.Split(p.Path, "/")
	binName := parts[len(parts)-1]
	if semver.Major(binName) != "" {
		binName = parts[len(parts)-2]
	}

	return binName
}

// IsValid checks if the package is valid. A package is valid if it has a
// non-empty path and a valid version.
func (p Package) IsValid() bool {
	return strings.TrimSpace(p.Path) != "" && p.Version.IsValid()
}

// String returns the string representation of the package.
func (p Package) String() string {
	return p.Path + "@" + p.Version.String()
}
