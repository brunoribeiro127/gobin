package internal

import (
	"os"
	"os/exec"
)

type ExecRun interface {
	Run() error
}

type ExecRunFunc func(name string, args ...string) ExecRun

type ExecCombinedOutput interface {
	CombinedOutput() ([]byte, error)
}

type ExecCombinedOutputFunc func(name string, args ...string) ExecCombinedOutput

type execRun struct {
	cmd *exec.Cmd
}

func NewExecRun(
	name string,
	args ...string,
) ExecRun {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return &execRun{
		cmd: cmd,
	}
}

func (e *execRun) Run() error {
	return e.cmd.Run()
}

type execCombinedOutput struct {
	cmd *exec.Cmd
}

func NewExecCombinedOutput(
	name string,
	args ...string,
) ExecCombinedOutput {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()

	return &execCombinedOutput{
		cmd: cmd,
	}
}

func (e *execCombinedOutput) CombinedOutput() ([]byte, error) {
	return e.cmd.CombinedOutput()
}
