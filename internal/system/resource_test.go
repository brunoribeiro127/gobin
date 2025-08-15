package system_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/system"
	"github.com/brunoribeiro127/gobin/internal/system/mocks"
)

func TestResource_Open(t *testing.T) {
	cases := map[string]struct {
		resource      string
		mockRuntimeOS string
		callCmd       bool
		mockCmdName   string
		mockCmdArgs   []string
		mockCmdOutput []byte
		mockCmdErr    error
		expectedErr   error
	}{
		"success-darwin": {
			resource:      "https://example.com",
			mockRuntimeOS: "darwin",
			callCmd:       true,
			mockCmdName:   "open",
		},
		"success-linux": {
			resource:      "https://example.com",
			mockRuntimeOS: "linux",
			callCmd:       true,
			mockCmdName:   "xdg-open",
		},
		"success-windows": {
			resource:      "https://example.com",
			mockRuntimeOS: "windows",
			callCmd:       true,
			mockCmdName:   "cmd",
			mockCmdArgs:   []string{"/c", "start"},
			mockCmdOutput: []byte{},
		},
		"error-unsupported-platform": {
			resource:      "https://example.com",
			mockRuntimeOS: "unsupported",
			expectedErr:   errors.New("unsupported platform: unsupported"),
		},
		"error-cmd-output": {
			resource:      "https://example.com",
			mockRuntimeOS: "windows",
			callCmd:       true,
			mockCmdName:   "cmd",
			mockCmdArgs:   []string{"/c", "start"},
			mockCmdOutput: []byte("unexpected error"),
			mockCmdErr:    errors.New("exit status 1"),
			expectedErr:   errors.New("exit status 1: unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			exec := mocks.NewExec(t)
			execCmd := mocks.NewExecCombinedOutput(t)
			runtime := mocks.NewRuntime(t)

			runtime.EXPECT().OS().Return(tc.mockRuntimeOS).Once()

			if tc.callCmd {
				exec.EXPECT().CombinedOutput(
					context.Background(),
					tc.mockCmdName,
					append(tc.mockCmdArgs, tc.resource),
				).Return(execCmd).Once()

				execCmd.EXPECT().CombinedOutput().Return(tc.mockCmdOutput, tc.mockCmdErr).Once()
			}

			resource := system.NewResource(exec, runtime)
			err := resource.Open(context.Background(), tc.resource)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
