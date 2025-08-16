package model

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// Version represents a version.
type Version string

// NewVersion creates a new version from a version string.
func NewVersion(version string) Version {
	return Version(strings.ToLower(strings.TrimSpace(version)))
}

// NewLatestVersion creates a new version "latest".
func NewLatestVersion() Version {
	return NewVersion("latest")
}

// Compare compares two versions.
func (v Version) Compare(other Version) int {
	return semver.Compare(string(v), string(other))
}

// IsLatest checks if the version is "latest".
func (v Version) IsLatest() bool {
	//nolint:goconst,nolintlint
	return string(v) == "latest"
}

// IsPartOf checks if a full version is part of a base version. If base version
// is a major or major.minor version, it checks if the full version is greater
// than or equal to the base version and less than the next major or major.minor
// version. Otherwise it performs a full version comparison.
func (v Version) IsPartOf(baseVersion Version) bool {
	parts := strings.Split(strings.TrimPrefix(string(baseVersion), "v"), ".")

	switch len(parts) {
	case 1:
		major, _ := strconv.Atoi(parts[0])
		lower := fmt.Sprintf("v%d.0.0", major)
		upper := fmt.Sprintf("v%d.0.0", major+1)
		return semver.Compare(string(v), lower) >= 0 && semver.Compare(string(v), upper) < 0
	case 2: //nolint:mnd // expected version format: major.minor
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])
		lower := fmt.Sprintf("v%d.%d.0", major, minor)
		upper := fmt.Sprintf("v%d.%d.0", major, minor+1)
		return semver.Compare(string(v), lower) >= 0 && semver.Compare(string(v), upper) < 0
	default:
		return semver.Compare(string(v), string(baseVersion)) == 0
	}
}

// IsValid checks if the version is valid. If the version is "latest", it is
// considered valid. Otherwise it checks if the version is a valid semantic
// version.
func (v Version) IsValid() bool {
	if v.IsLatest() {
		return true
	}

	return semver.IsValid(string(v))
}

// Major returns the major version.
func (v Version) Major() string {
	return semver.Major(string(v))
}

// MajorMinor returns the major and minor version.
func (v Version) MajorMinor() string {
	return semver.MajorMinor(string(v))
}

// NextMajorVersion returns the next major version.
func (v Version) NextMajorVersion() Version {
	major := semver.Major(string(v))
	if major == "v0" || major == "v1" {
		return Version("v2")
	}

	nextMajor, _ := strconv.Atoi(strings.TrimPrefix(major, "v"))
	return Version("v" + strconv.Itoa(nextMajor+1))
}

// String returns the string representation of the version.
func (v Version) String() string {
	return string(v)
}
