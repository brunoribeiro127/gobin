package toolchain

import (
	"bytes"
	"context"
	"os"

	"golang.org/x/vuln/scan"

	"github.com/brunoribeiro127/gobin/internal/system"
)

// ScanExecCombinedOutputFunc is a function that creates a new ExecCombinedOutput
// that runs the govulncheck command.
type ScanExecCombinedOutputFunc func(ctx context.Context, args ...string) system.ExecCombinedOutput

// scanExecCombinedOutput is the default implementation of ExecCombinedOutput
// that runs the govulncheck command.
type scanExecCombinedOutput struct {
	cmd    *scan.Cmd
	output *bytes.Buffer
}

// NewScanExecCombinedOutput creates a new ExecCombinedOutput. It uses the scan
// package to run the govulncheck command, injecting the environment variables
// from the current process.
func NewScanExecCombinedOutput(
	ctx context.Context,
	args ...string,
) system.ExecCombinedOutput {
	var output bytes.Buffer
	cmd := scan.Command(ctx, args...)
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Env = os.Environ()

	return &scanExecCombinedOutput{
		cmd:    cmd,
		output: &output,
	}
}

// CombinedOutput runs the govulncheck command and returns the combined output.
// It starts the command and waits for it to complete, returning the combined
// output of the command.
func (s *scanExecCombinedOutput) CombinedOutput() ([]byte, error) {
	if err := s.cmd.Start(); err != nil {
		return s.output.Bytes(), err
	}

	if err := s.cmd.Wait(); err != nil {
		return s.output.Bytes(), err
	}

	return s.output.Bytes(), nil
}
