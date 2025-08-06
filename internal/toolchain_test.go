package internal_test

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/mocks"
)

func TestGoToolchain_GetBuildInfo(t *testing.T) {
	cases := map[string]struct {
		path              string
		mockReadFile      *buildinfo.BuildInfo
		mockReadFileErr   error
		expectedBuildInfo *buildinfo.BuildInfo
		expectedErr       error
	}{
		"success": {
			path: "/home/user/go/bin/mockproj",
			mockReadFile: &buildinfo.BuildInfo{
				Main: debug.Module{
					Path: "example.com/mockorg/mockproj",
				},
			},
			expectedBuildInfo: &buildinfo.BuildInfo{
				Main: debug.Module{
					Path: "example.com/mockorg/mockproj",
				},
			},
		},
		"error-binary-not-found": {
			path:            "/home/user/go/bin/mockproj",
			mockReadFileErr: os.ErrNotExist,
			expectedErr:     internal.ErrBinaryNotFound,
		},
		"error-reading-build-info": {
			path:            "/home/user/go/bin/mockproj",
			mockReadFileErr: errors.New("unexpected error"),
			expectedErr:     errors.New("unexpected error"),
		},
		"error-binary-built-without-go-modules": {
			path:         "/home/user/go/bin/mockproj",
			mockReadFile: &buildinfo.BuildInfo{},
			expectedErr:  internal.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			system.EXPECT().ReadBuildInfo(tc.path).
				Return(tc.mockReadFile, tc.mockReadFileErr).
				Once()

			toolchain := internal.NewGoToolchain(nil, nil, nil, system)
			buildInfo, err := toolchain.GetBuildInfo(tc.path)
			assert.Equal(t, tc.expectedBuildInfo, buildInfo)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoToolchain_GetLatestModuleVersion(t *testing.T) {
	makeExecCmdOutput := func(t *testing.T, modFile string) []byte {
		wd, err := os.Getwd()
		require.NoError(t, err)
		testFile := filepath.Join(wd, "testdata", modFile)
		return fmt.Appendf(nil, `{"GoMod":"%s","Version":"v0.1.0"}`, testFile)
	}

	cases := map[string]struct {
		module             string
		mockExecCmdOutput  []byte
		mockExecCmdErr     error
		expectedModulePath string
		expectedVersion    string
		expectedErr        error
	}{
		"success": {
			module:             "example.com/mockorg/mockproj",
			mockExecCmdOutput:  makeExecCmdOutput(t, "go.mod"),
			expectedModulePath: "example.com/mockorg/mockproj",
			expectedVersion:    "v0.1.0",
		},
		"sucess-module-path-update": {
			module:             "example.com/mockorg/mockproj",
			mockExecCmdOutput:  makeExecCmdOutput(t, "new.go.mod"),
			expectedModulePath: "example.com/newmockorg/newmockproj",
			expectedVersion:    "v0.1.0",
		},
		"error-module-not-found": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`no matching versions for query`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedErr:    internal.ErrModuleNotFound,
		},
		"error-getting-latest-version": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`unexpected error	`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedErr:    errors.New("exit status 1: unexpected error"),
		},
		"error-parsing-module-latest-version-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedErr: errors.New("unexpected end of JSON input"),
		},
		"error-reading-go-mod-file": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"GoMod":"./go.mod","Version":"v0.1.0"}`)
			}(),
			expectedErr: errors.New("open ./go.mod: no such file or directory"),
		},
		"error-go-mod-file-not-available": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Version":"v0.1.0"}`)
			}(),
			expectedErr: internal.ErrGoModFileNotAvailable,
		},
		"error-parsing-go-mod-file": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "invalid.go.mod"),
			expectedErr:       errors.New("go.mod:1: unknown directive: invalid"),
		},
		"error-module-info-not-available": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "empty.go.mod"),
			expectedErr:       internal.ErrModuleInfoNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			execCmd := mocks.NewExecCombinedOutput(t)
			execCmd.EXPECT().CombinedOutput().
				Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).
				Once()

			execCmdFunc := func(_ context.Context, name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"list", "-m", "-json", fmt.Sprintf("%s@latest", tc.module)}, args)
				return execCmd
			}

			toolchain := internal.NewGoToolchain(execCmdFunc, nil, nil, nil)
			modulePath, version, err := toolchain.GetLatestModuleVersion(context.Background(), tc.module)
			assert.Equal(t, tc.expectedModulePath, modulePath)
			assert.Equal(t, tc.expectedVersion, version)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGoToolchain_GetModuleFile(t *testing.T) {
	makeExecCmdOutput := func(t *testing.T, modFile string) []byte {
		wd, err := os.Getwd()
		require.NoError(t, err)
		testFile := filepath.Join(wd, "testdata", modFile)
		return fmt.Appendf(nil, `{"GoMod":"%s"}`, testFile)
	}

	cases := map[string]struct {
		module            string
		version           string
		mockExecCmdOutput []byte
		mockExecCmdErr    error
		expectedModFile   *modfile.File
		expectedErr       error
	}{
		"success": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "go.mod"),
			expectedModFile: &modfile.File{
				Module: &modfile.Module{
					Mod: module.Version{
						Path: "example.com/mockorg/mockproj",
					},
					Syntax: &modfile.Line{
						Start: modfile.Position{
							Line:     1,
							LineRune: 1,
						},
						End: modfile.Position{
							Line:     1,
							LineRune: 36,
							Byte:     35,
						},
						Token: []string{
							"module",
							"example.com/mockorg/mockproj",
						},
					},
				},
				Syntax: &modfile.FileSyntax{
					Name: "go.mod",
					Stmt: []modfile.Expr{
						&modfile.Line{
							Start: modfile.Position{
								Line:     1,
								LineRune: 1,
							},
							End: modfile.Position{
								Line:     1,
								LineRune: 36,
								Byte:     35,
							},
							Token: []string{
								"module",
								"example.com/mockorg/mockproj",
							},
						},
					},
				},
			},
		},
		"error-downloading-module": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"unexpected error"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedErr:    errors.New("unexpected error"),
		},
		"error-parsing-module-download-success-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedErr: errors.New("unexpected end of JSON input"),
		},
		"error-reading-go-mod-file": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"GoMod":"./go.mod"}`)
			}(),
			expectedErr: errors.New("open ./go.mod: no such file or directory"),
		},
		"error-parsing-go-mod-file": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "invalid.go.mod"),
			expectedErr:       errors.New("go.mod:1: unknown directive: invalid"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			execCmd := mocks.NewExecCombinedOutput(t)
			execCmd.EXPECT().CombinedOutput().
				Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).
				Once()

			execCmdFunc := func(_ context.Context, name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"mod", "download", "-json", fmt.Sprintf("%s@%s", tc.module, tc.version)}, args)
				return execCmd
			}

			toolchain := internal.NewGoToolchain(execCmdFunc, nil, nil, nil)
			modFile, err := toolchain.GetModuleFile(context.Background(), tc.module, tc.version)
			assert.Equal(t, tc.expectedModFile, modFile)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGoToolchain_GetModuleOrigin(t *testing.T) {
	cases := map[string]struct {
		module            string
		version           string
		mockExecCmdOutput []byte
		mockExecCmdErr    error
		expectedModOrigin *internal.ModuleOrigin
		expectedErr       error
	}{
		"success": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: []byte(`{
				"Origin":{
					"VCS":"git",
					"URL":"https://github.com/mockorg/mockproj",
					"Hash":"1234567890",
					"Ref":"refs/heads/v0.1.0"
				}
			}`),
			expectedModOrigin: &internal.ModuleOrigin{
				VCS:  "git",
				URL:  "https://github.com/mockorg/mockproj",
				Hash: "1234567890",
				Ref: func() *string {
					v := "refs/heads/v0.1.0"
					return &v
				}(),
			},
		},
		"error-module-not-found": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"not found"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedErr:    internal.ErrModuleNotFound,
		},
		"error-downloading-module": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"unexpected error"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedErr:    errors.New("unexpected error"),
		},
		"error-parsing-module-download-success-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedErr: errors.New("unexpected end of JSON input"),
		},
		"error-module-origin-not-available": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: []byte(`{"Origin":null}`),
			expectedErr:       internal.ErrModuleOriginNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			execCmd := mocks.NewExecCombinedOutput(t)
			execCmd.EXPECT().CombinedOutput().
				Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).
				Once()

			execCmdFunc := func(_ context.Context, name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"mod", "download", "-json", fmt.Sprintf("%s@%s", tc.module, tc.version)}, args)
				return execCmd
			}

			toolchain := internal.NewGoToolchain(execCmdFunc, nil, nil, nil)
			modOrigin, err := toolchain.GetModuleOrigin(context.Background(), tc.module, tc.version)
			assert.Equal(t, tc.expectedModOrigin, modOrigin)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGoToolchain_Install(t *testing.T) {
	cases := map[string]struct {
		pkg            string
		version        string
		mockExecCmdErr error
		expectedErr    error
	}{
		"success": {
			pkg:     "example.com/mockorg/mockproj/cmd/mockproj",
			version: "v0.1.0",
		},
		"error-installing-binary": {
			pkg:            "example.com/mockorg/mockproj/cmd/mockproj",
			version:        "v0.1.0",
			mockExecCmdErr: errors.New("unexpected error"),
			expectedErr:    errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			execRun := mocks.NewExecRun(t)
			execRun.EXPECT().Run().Return(tc.mockExecCmdErr).Once()

			execRunFunc := func(_ context.Context, name string, args ...string) internal.ExecRun {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"install", "-a", fmt.Sprintf("%s@%s", tc.pkg, tc.version)}, args)
				return execRun
			}

			toolchain := internal.NewGoToolchain(nil, execRunFunc, nil, nil)
			err := toolchain.Install(context.Background(), tc.pkg, tc.version)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGoToolchain_VulnCheck(t *testing.T) {
	cases := map[string]struct {
		path              string
		mockExecCmdOutput []byte
		mockExecCmdErr    error
		expectedVulns     []internal.Vulnerability
		expectedErr       error
	}{
		"success-filter-vulns-by-affected-status": {
			path: "/home/user/go/bin/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{
					"statements":[
						{
							"vulnerability":{
								"@id":"https://pkg.go.dev/vuln/GO-2025-3754",
								"name":"GO-2025-3754"
							},
							"status":"affected"
						},
												{
							"vulnerability":{
								"@id":"https://pkg.go.dev/vuln/GO-2022-0646",
								"name":"GO-2022-0646"
							},
							"status":"not_affected"
						}
					]
				}`)
			}(),
			expectedVulns: []internal.Vulnerability{
				{
					ID:  "GO-2025-3754",
					URL: "https://pkg.go.dev/vuln/GO-2025-3754",
				},
			},
		},
		"error-running-govulncheck-command": {
			path:              "/home/user/go/bin/mockproj",
			mockExecCmdOutput: []byte(`unexpected error`),
			mockExecCmdErr:    errors.New("exit status 1"),
			expectedErr:       errors.New("exit status 1: unexpected error"),
		},
		"error-parsing-govulncheck-response": {
			path:              "/home/user/go/bin/mockproj",
			mockExecCmdOutput: []byte(``),
			expectedErr:       errors.New("unexpected end of JSON input"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			execCmd := mocks.NewExecCombinedOutput(t)
			execCmd.EXPECT().CombinedOutput().
				Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).
				Once()

			execCmdFunc := func(_ context.Context, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, []string{"-mode", "binary", "-format", "openvex", tc.path}, args)
				return execCmd
			}

			toolchain := internal.NewGoToolchain(nil, nil, execCmdFunc, nil)
			vulns, err := toolchain.VulnCheck(context.Background(), tc.path)
			assert.Equal(t, tc.expectedVulns, vulns)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
