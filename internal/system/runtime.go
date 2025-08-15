package system

import "runtime"

// Runtime is the interface for the runtime.
type Runtime interface {
	// OS returns the operating system.
	OS() string
	// Platform returns the platform in the format "os/arch".
	Platform() string
	// Version returns the version.
	Version() string
}

// rt is the default implementation of the Runtime interface.
type rt struct{}

// NewRuntime creates a new runtime.
func NewRuntime() Runtime {
	return &rt{}
}

// OS returns the operating system.
func (r *rt) OS() string {
	return runtime.GOOS
}

// Platform returns the platform in the format "os/arch".
func (r *rt) Platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// Version returns the version.
func (r *rt) Version() string {
	return runtime.Version()
}
