package model

import "strings"

// BinaryInfo represents the information for a binary.
type BinaryInfo struct {
	Name           string
	FullPath       string
	InstallPath    string
	PackagePath    string
	Module         Module
	ModuleSum      string
	GoVersion      string
	CommitRevision string
	CommitTime     string
	OS             string
	Arch           string
	Feature        string
	EnvVars        []string

	IsManaged bool
}

// GetPinnedVersion returns the pinned version of the binary. If the binary
// name contains a version suffix, it returns the version. Otherwise, it
// returns "latest".
func (b BinaryInfo) GetPinnedVersion() Version {
	parts := strings.Split(b.Name, "-")
	//nolint:mnd // expected version format: name-v<major> or name-v<major>.<minor>
	if len(parts) == 2 {
		return NewVersion(parts[1])
	}

	return NewLatestVersion()
}

// BinaryUpgradeInfo represents the upgrade information for a binary.
type BinaryUpgradeInfo struct {
	BinaryInfo

	LatestModule       Module
	IsUpgradeAvailable bool
}

// GetUpgradePackage returns the package for a binary upgrade. If the latest
// version is a major version v2 or higher, it adjusts the package path to
// include the major version, following the Go module versioning rules.
func (b BinaryUpgradeInfo) GetUpgradePackage() Package {
	baseModule := b.LatestModule.GetBaseModule()
	packageSuffix := strings.TrimPrefix(b.PackagePath, b.Module.Path)

	pkg := baseModule + packageSuffix
	if major := b.LatestModule.Version.Major(); major != "v0" && major != "v1" {
		pkg = baseModule + "/" + major + packageSuffix
	}

	return Package{
		Path:    pkg,
		Version: b.LatestModule.Version,
	}
}
