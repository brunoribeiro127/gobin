package system_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunoribeiro127/gobin/internal/system"
	"github.com/brunoribeiro127/gobin/internal/system/mocks"
)

type mockMkdirAllCall struct {
	dir  string
	perm os.FileMode
	err  error
}

func TestWorkspace(t *testing.T) {
	cases := map[string]struct {
		mockUserHomeDir          string
		mockUserHomeDirErr       error
		callGetGOBINEnvVar       bool
		mockGOBINEnvVar          string
		mockGOBINEnvVarOk        bool
		callGetGOPATHEnvVar      bool
		mockGOPATHEnvVar         string
		mockGOPATHEnvVarOk       bool
		callRuntimeOS            bool
		mockRuntimeOS            string
		mockMkdirAllCalls        []mockMkdirAllCall
		expectedGoBinPath        string
		expectedInternalBasePath string
		expectedInternalBinPath  string
		expectedInternalTempPath string
		expectedErr              error
	}{
		"success-unix-default-go-bin-path": {
			mockUserHomeDir:     "/home/user",
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/.gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/.tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/.gobin",
			expectedInternalBinPath:  "/home/user/.gobin/bin",
			expectedInternalTempPath: "/home/user/.gobin/.tmp",
		},
		"success-unix-gobin-env-var": {
			mockUserHomeDir:    "/home/user",
			callGetGOBINEnvVar: true,
			mockGOBINEnvVar:    "/home/user/go/bin",
			mockGOBINEnvVarOk:  true,
			callRuntimeOS:      true,
			mockRuntimeOS:      "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/.gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/.tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/.gobin",
			expectedInternalBinPath:  "/home/user/.gobin/bin",
			expectedInternalTempPath: "/home/user/.gobin/.tmp",
		},
		"success-unix-gopath-env-var": {
			mockUserHomeDir:     "/home/user",
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			mockGOPATHEnvVar:    "/home/user/go",
			mockGOPATHEnvVarOk:  true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/.gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/.gobin/.tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/.gobin",
			expectedInternalBinPath:  "/home/user/.gobin/bin",
			expectedInternalTempPath: "/home/user/.gobin/.tmp",
		},
		"success-windows-default-go-bin-path": {
			mockUserHomeDir:     "/home/user",
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/AppData/Local/gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/AppData/Local/gobin",
			expectedInternalBinPath:  "/home/user/AppData/Local/gobin/bin",
			expectedInternalTempPath: "/home/user/AppData/Local/gobin/tmp",
		},
		"success-windows-gobin-env-var": {
			mockUserHomeDir:    "/home/user",
			callGetGOBINEnvVar: true,
			mockGOBINEnvVar:    "/home/user/go/bin",
			mockGOBINEnvVarOk:  true,
			callRuntimeOS:      true,
			mockRuntimeOS:      "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/AppData/Local/gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/AppData/Local/gobin",
			expectedInternalBinPath:  "/home/user/AppData/Local/gobin/bin",
			expectedInternalTempPath: "/home/user/AppData/Local/gobin/tmp",
		},
		"success-windows-gopath-env-var": {
			mockUserHomeDir:     "/home/user",
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			mockGOPATHEnvVar:    "/home/user/go",
			mockGOPATHEnvVarOk:  true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/AppData/Local/gobin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/bin",
					perm: 0700,
				},
				{
					dir:  "/home/user/AppData/Local/gobin/tmp",
					perm: 0700,
				},
			},
			expectedGoBinPath:        "/home/user/go/bin",
			expectedInternalBasePath: "/home/user/AppData/Local/gobin",
			expectedInternalBinPath:  "/home/user/AppData/Local/gobin/bin",
			expectedInternalTempPath: "/home/user/AppData/Local/gobin/tmp",
		},
		"error-user-home-dir": {
			mockUserHomeDirErr: errors.New("unexpected error"),
			expectedErr:        errors.New("unexpected error"),
		},
		"error-mkdir-all": {
			mockUserHomeDir:     "/home/user",
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  "/home/user/.gobin",
					perm: 0700,
					err:  errors.New("unexpected error"),
				},
			},
			expectedErr: errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := mocks.NewEnvironment(t)
			fs := mocks.NewFileSystem(t)
			rt := mocks.NewRuntime(t)

			env.EXPECT().UserHomeDir().
				Return(tc.mockUserHomeDir, tc.mockUserHomeDirErr).
				Once()

			if tc.callGetGOBINEnvVar {
				env.EXPECT().Get("GOBIN").
					Return(tc.mockGOBINEnvVar, tc.mockGOBINEnvVarOk).
					Once()
			}

			if tc.callGetGOPATHEnvVar {
				env.EXPECT().Get("GOPATH").
					Return(tc.mockGOPATHEnvVar, tc.mockGOPATHEnvVarOk).
					Once()
			}

			if tc.callRuntimeOS {
				rt.EXPECT().OS().
					Return(tc.mockRuntimeOS).
					Once()
			}

			for _, call := range tc.mockMkdirAllCalls {
				fs.EXPECT().CreateDir(call.dir, call.perm).
					Return(call.err).
					Once()
			}

			workspace := system.NewWorkspace(env, fs, rt)
			err := workspace.Initialize()
			if tc.expectedErr != nil {
				assert.Equal(t, tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedGoBinPath, workspace.GetGoBinPath())
				assert.Equal(t, tc.expectedInternalBasePath, workspace.GetInternalBasePath())
				assert.Equal(t, tc.expectedInternalBinPath, workspace.GetInternalBinPath())
				assert.Equal(t, tc.expectedInternalTempPath, workspace.GetInternalTempPath())
			}
		})
	}
}
