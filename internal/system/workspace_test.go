package system_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

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
			mockUserHomeDir:     filepath.Join("home", "user"),
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", ".gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", ".tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", ".gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", ".gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", ".gobin", ".tmp"),
		},
		"success-unix-gobin-env-var": {
			mockUserHomeDir:    filepath.Join("home", "user"),
			callGetGOBINEnvVar: true,
			mockGOBINEnvVar:    filepath.Join("home", "user", "go", "bin"),
			mockGOBINEnvVarOk:  true,
			callRuntimeOS:      true,
			mockRuntimeOS:      "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", ".gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", ".tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", ".gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", ".gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", ".gobin", ".tmp"),
		},
		"success-unix-gopath-env-var": {
			mockUserHomeDir:     filepath.Join("home", "user"),
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			mockGOPATHEnvVar:    filepath.Join("home", "user", "go"),
			mockGOPATHEnvVarOk:  true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", ".gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", ".gobin", ".tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", ".gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", ".gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", ".gobin", ".tmp"),
		},
		"success-windows-default-go-bin-path": {
			mockUserHomeDir:     filepath.Join("home", "user"),
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", "AppData", "Local", "gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
		},
		"success-windows-gobin-env-var": {
			mockUserHomeDir:    filepath.Join("home", "user"),
			callGetGOBINEnvVar: true,
			mockGOBINEnvVar:    filepath.Join("home", "user", "go", "bin"),
			mockGOBINEnvVarOk:  true,
			callRuntimeOS:      true,
			mockRuntimeOS:      "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", "AppData", "Local", "gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
		},
		"success-windows-gopath-env-var": {
			mockUserHomeDir:     filepath.Join("home", "user"),
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			mockGOPATHEnvVar:    filepath.Join("home", "user", "go"),
			mockGOPATHEnvVarOk:  true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "windows",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
					perm: 0700,
				},
				{
					dir:  filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
					perm: 0700,
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", "AppData", "Local", "gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", "AppData", "Local", "gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", "AppData", "Local", "gobin", "tmp"),
		},
		"error-user-home-dir": {
			mockUserHomeDirErr: errors.New("unexpected error"),
			expectedErr:        errors.New("unexpected error"),
		},
		"error-mkdir-all": {
			mockUserHomeDir:     filepath.Join("home", "user"),
			callGetGOBINEnvVar:  true,
			callGetGOPATHEnvVar: true,
			callRuntimeOS:       true,
			mockRuntimeOS:       "linux",
			mockMkdirAllCalls: []mockMkdirAllCall{
				{
					dir:  filepath.Join("home", "user", ".gobin"),
					perm: 0700,
					err:  errors.New("unexpected error"),
				},
			},
			expectedGoBinPath:        filepath.Join("home", "user", "go", "bin"),
			expectedInternalBasePath: filepath.Join("home", "user", ".gobin"),
			expectedInternalBinPath:  filepath.Join("home", "user", ".gobin", "bin"),
			expectedInternalTempPath: filepath.Join("home", "user", ".gobin", ".tmp"),
			expectedErr:              errors.New("unexpected error"),
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

			workspace, err := system.NewWorkspace(env, fs, rt)
			if err != nil {
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.Equal(t, tc.expectedGoBinPath, workspace.GetGoBinPath())
				assert.Equal(t, tc.expectedInternalBasePath, workspace.GetInternalBasePath())
				assert.Equal(t, tc.expectedInternalBinPath, workspace.GetInternalBinPath())
				assert.Equal(t, tc.expectedInternalTempPath, workspace.GetInternalTempPath())

				err = workspace.Initialize()
				assert.Equal(t, tc.expectedErr, err)
			}
		})
	}
}
