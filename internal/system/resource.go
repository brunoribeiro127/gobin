package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Resource is the interface for handling resources.
type Resource interface {
	// Open opens a resource using the default system tools.
	Open(ctx context.Context, resource string) error
}

// resource is the default implementation of the Resource interface.
type resource struct {
	exec    Exec
	runtime Runtime
}

// NewResource creates a new Resource.
func NewResource(
	exec Exec,
	runtime Runtime,
) Resource {
	return &resource{
		exec:    exec,
		runtime: runtime,
	}
}

// Open opens a resource using the default system tools. It returns an
// error if the resource cannot be opened or the platform is not supported.
func (r *resource) Open(ctx context.Context, resource string) error {
	logger := slog.Default().With("resource", resource)

	var cmd ExecCombinedOutput
	runtimeOS := r.runtime.OS()
	switch runtimeOS {
	case "darwin":
		cmd = r.exec.CombinedOutput(ctx, "open", resource)
	case "linux":
		cmd = r.exec.CombinedOutput(ctx, "xdg-open", resource)
	case "windows": //nolint:goconst,nolintlint
		cmd = r.exec.CombinedOutput(ctx, "cmd", "/c", "start", resource)
	default:
		err := fmt.Errorf("unsupported platform: %s", runtimeOS)
		logger.ErrorContext(ctx, "error opening resource", "err", err)
		return err
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			err = fmt.Errorf("%w: %s", err, outputStr)
		}

		logger.ErrorContext(ctx, "error opening resource", "err", err)
		return err
	}

	return nil
}
