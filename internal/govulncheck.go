package internal

import (
	"bytes"
	"context"
	"os"

	"golang.org/x/vuln/scan"
)

type ScanExecCombinedOutputFunc func(ctx context.Context, args ...string) ExecCombinedOutput

type scanExecCombinedOutput struct {
	cmd    *scan.Cmd
	output *bytes.Buffer
}

func NewScanExecCombinedOutput(
	ctx context.Context,
	args ...string,
) ExecCombinedOutput {
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

func (s *scanExecCombinedOutput) CombinedOutput() ([]byte, error) {
	if err := s.cmd.Start(); err != nil {
		return s.output.Bytes(), err
	}

	if err := s.cmd.Wait(); err != nil {
		return s.output.Bytes(), err
	}

	return s.output.Bytes(), nil
}
