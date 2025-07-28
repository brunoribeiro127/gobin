package toolchain

import (
	"bytes"
	"context"
	"os"

	"github.com/brunoribeiro127/gobin/internal"
	"golang.org/x/vuln/scan"
)

type ScanExecCombinedOutput interface {
	internal.ExecCombinedOutput
}

type ScanExecCombinedOutputFunc func(args ...string) ScanExecCombinedOutput

type scanExecCombinedOutput struct {
	cmd    *scan.Cmd
	output *bytes.Buffer
}

func NewScanExecRun(args ...string) ScanExecCombinedOutput {
	var output bytes.Buffer
	cmd := scan.Command(context.Background(), args...)
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Env = os.Environ()

	return &scanExecCombinedOutput{
		cmd:    cmd,
		output: &output,
	}
}

func (s *scanExecCombinedOutput) CombinedOutput() ([]byte, error) {
	if err := s.cmd.Start(); err != nil {
		return s.output.Bytes(), err
	}

	if err := s.cmd.Wait(); err != nil {
		return s.output.Bytes(), err
	}

	return s.output.Bytes(), nil
}
