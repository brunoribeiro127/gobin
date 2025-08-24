package model

import (
	"path/filepath"
	"strings"
)

// Binary represents a binary.
type Binary struct {
	Name      string
	Version   Version
	Extension string
}

// NewBinary creates a new binary with the given name, version and extension.
func NewBinary(
	name string,
	version Version,
	extension string,
) Binary {
	return Binary{
		Name:      name,
		Version:   version,
		Extension: extension,
	}
}

// NewBinaryFromString creates a new binary from a binary name, version and
// extension. If the binary name does not have a version, it defaults to
// "latest". Extension is the extension of the binary and can be empty.
func NewBinaryFromString(bin string) Binary {
	extension := filepath.Ext(bin)
	if extension != ".exe" {
		extension = ""
	}

	version := "latest"
	name := strings.TrimSuffix(bin, extension)

	//nolint:mnd // expected version format: name@version
	if parts := strings.Split(bin, "@"); len(parts) == 2 {
		name, version = parts[0], strings.TrimSuffix(parts[1], extension)
	}

	return Binary{
		Name:      name,
		Version:   NewVersion(version),
		Extension: extension,
	}
}

// GetPinKind returns the pin kind of the binary. If the binary name contains
// a version suffix, it returns the kind. Otherwise, it returns latest.
func (b Binary) GetPinKind() Kind {
	parts := strings.Split(b.Name, "-")
	if version := NewVersion(parts[len(parts)-1]); version.IsValid() {
		if strings.Contains(version.String(), ".") {
			return KindMinor
		}

		return KindMajor
	}

	return KindLatest
}

// GetPinnedVersion returns the pinned version of the binary. If the binary
// name contains a version suffix, it returns the version. Otherwise, it
// returns "latest".
func (b Binary) GetPinnedVersion() Version {
	parts := strings.Split(b.Name, "-")
	if len(parts) > 1 {
		if version := NewVersion(parts[len(parts)-1]); version.IsValid() {
			return version
		}
	}

	return NewLatestVersion()
}

// GetTargetBinName returns the target binary name for a binary based on the
// pin kind.
func (b Binary) GetTargetBinName(kind Kind) string {
	var name string

	switch kind {
	case KindLatest:
		name = b.Name + b.Extension
	case KindMajor:
		name = b.Name + "-" + b.Version.Major() + b.Extension
	case KindMinor:
		name = b.Name + "-" + b.Version.MajorMinor() + b.Extension
	}

	return name
}

// IsPartOf checks if a binary is part of another binary. The binary name must
// match and the version of the target must be either latest or the binary
// version must be a part of the target version.
func (b Binary) IsPartOf(o Binary) bool {
	if b.Name != o.Name {
		return false
	}

	return o.Version.IsLatest() || b.Version.IsPartOf(o.Version)
}

// IsValid checks if the binary is valid. A binary is valid if it has a
// non-empty name and a valid version.
func (b Binary) IsValid() bool {
	return strings.TrimSpace(b.Name) != "" && b.Version.IsValid()
}

// String returns the string representation of the binary. If the version is
// latest, it returns the binary name only. Otherwise, it returns the binary
// name and the version.
func (b Binary) String() string {
	if b.Version.IsLatest() {
		return b.Name + b.Extension
	}

	return b.Name + "@" + b.Version.String() + b.Extension
}
