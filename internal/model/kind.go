package model

import (
	"fmt"
	"slices"
	"strings"
)

// Kind is a the kind of pin to create. It implements the [flag.Value] interface.
type Kind string

const (
	// KindLatest is the kind of pin to create for the latest version, ex. "gobin".
	KindLatest Kind = "latest"
	// KindMajor is the kind of pin to create for the major version, ex. "gobin-1".
	KindMajor Kind = "major"
	// KindMinor is the kind of pin to create for the minor version, ex. "gobin-1.2".
	KindMinor Kind = "minor"
)

// allowedKinds is a list of allowed kinds.
//
//nolint:gochecknoglobals // global variable to define allowed kinds
var allowedKinds = []Kind{
	KindLatest,
	KindMajor,
	KindMinor,
}

// IsValid checks if the kind is valid.
func (k *Kind) IsValid() bool {
	return slices.Contains(allowedKinds, *k)
}

// String returns the string representation of the kind.
func (k *Kind) String() string {
	return string(*k)
}

// Set sets the kind from a string.
func (k *Kind) Set(value string) error {
	candidate := Kind(strings.ToLower(value))
	if !candidate.IsValid() {
		return fmt.Errorf("invalid kind %q, allowed values are: %v", value, allowedKinds)
	}
	*k = candidate
	return nil
}

// Type returns the type of the kind.
func (k *Kind) Type() string {
	return "kind"
}
