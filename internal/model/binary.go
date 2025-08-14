package model

import (
	"strings"
)

// Binary represents a binary.
type Binary struct {
	Name    string
	Version Version
}

// NewBinary creates a new binary from a binary name and version. If the binary
// name does not have a version, it defaults to "latest".
func NewBinary(bin string) Binary {
	name, version := bin, "latest"

	//nolint:mnd // expected version format: name@version
	if parts := strings.Split(bin, "@"); len(parts) == 2 {
		name, version = parts[0], parts[1]
	}

	return Binary{
		Name:    name,
		Version: NewVersion(version),
	}
}

// NewBinaryWithVersion creates a new binary with the given name and version.
func NewBinaryWithVersion(name string, version Version) Binary {
	return Binary{
		Name:    name,
		Version: version,
	}
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
		return b.Name
	}

	return b.Name + "@" + b.Version.String()
}
