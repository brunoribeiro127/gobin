package internal

import (
	"context"
	"os"
	"os/exec"
)

// ExecRun is an interface that represents a command that can be run.
type ExecRun interface {
	Run() error
}

// ExecRunFunc is a function that creates a new ExecRun that runs a command.
type ExecRunFunc func(ctx context.Context, name string, args ...string) ExecRun

// ExecCombinedOutput is an interface that represents a command that can be run
// and returns the combined output.
type ExecCombinedOutput interface {
	CombinedOutput() ([]byte, error)
}

// ExecCombinedOutputFunc is a function that creates a new ExecCombinedOutput
// that runs a command and returns the combined output.
type ExecCombinedOutputFunc func(ctx context.Context, name string, args ...string) ExecCombinedOutput

// execRun is the default implementation of ExecRun that runs a command.
type execRun struct {
	cmd *exec.Cmd
}

// NewExecRun creates a new ExecRun that runs a command. It uses the exec
// package to run the command, injecting the environment variables from the
// current process.
func NewExecRun(
	ctx context.Context,
	name string,
	args ...string,
) ExecRun {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return &execRun{
		cmd: cmd,
	}
}

// Run runs the command and returns the error if the command fails.
func (e *execRun) Run() error {
	return e.cmd.Run()
}

// execCombinedOutput is the default implementation of ExecCombinedOutput that
// runs a command and returns the combined output.
type execCombinedOutput struct {
	cmd *exec.Cmd
}

// NewExecCombinedOutput creates a new ExecCombinedOutput that runs a command.
// It uses the exec package to run the command, injecting the environment
// variables from the current process.
func NewExecCombinedOutput(
	ctx context.Context,
	name string,
	args ...string,
) ExecCombinedOutput {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()

	return &execCombinedOutput{
		cmd: cmd,
	}
}

// CombinedOutput runs the command and returns the combined output.
func (e *execCombinedOutput) CombinedOutput() ([]byte, error) {
	return e.cmd.CombinedOutput()
}
