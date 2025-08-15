package system

import (
	"context"
	"os"
	"os/exec"
)

// Exec is the interface for creating commands to be executed.
type Exec interface {
	Run(ctx context.Context, name string, args ...string) ExecRun
	CombinedOutput(ctx context.Context, name string, args ...string) ExecCombinedOutput
}

// ExecRun is an interface that represents a command that can be run and inject
// environment variables.
type ExecRun interface {
	Run() error
	InjectEnv(env ...string)
}

// ExecCombinedOutput is an interface that represents a command that can be run
// and returns the combined output.
type ExecCombinedOutput interface {
	CombinedOutput() ([]byte, error)
}

// execCmd is the default implementation of the Exec interface.
type execCmd struct{}

// NewExec creates a new Exec.
func NewExec() Exec {
	return &execCmd{}
}

// Run creates a new ExecRun that runs a command.
func (e *execCmd) Run(ctx context.Context, name string, args ...string) ExecRun {
	return NewExecRun(ctx, name, args...)
}

// CombinedOutput creates a new ExecCombinedOutput that runs a command.
func (e *execCmd) CombinedOutput(ctx context.Context, name string, args ...string) ExecCombinedOutput {
	return NewExecCombinedOutput(ctx, name, args...)
}

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

// InjectEnv injects environment variables into the command.
func (e *execRun) InjectEnv(env ...string) {
	e.cmd.Env = append(e.cmd.Env, env...)
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
