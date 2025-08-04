package internal_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/mocks"
)

func TestGetLatestModuleVersion(t *testing.T) {
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
		expectedError      error
	}{
		"success": {
			module:             "example.com/mockorg/mockproj",
			mockExecCmdOutput:  makeExecCmdOutput(t, "go.mod"),
			expectedModulePath: "example.com/mockorg/mockproj",
			expectedVersion:    "v0.1.0",
		},
		"error-module-not-found": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`no matching versions for query`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  internal.ErrModuleNotFound,
		},
		"error-getting-latest-version": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`unexpected error	`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  errors.New("unexpected error"),
		},
		"error-parsing-module-latest-version": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedError: errors.New("unexpected end of JSON input"),
		},
		"error-reading-go-mod-file": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"GoMod":"./go.mod","Version":"v0.1.0"}`)
			}(),
			expectedError: errors.New("open ./go.mod: no such file or directory"),
		},
		"error-parsing-go-mod-file": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "invalid.go.mod"),
			expectedError:     errors.New("go.mod:1: unknown directive: invalid"),
		},
		"error-module-info-not-available": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "empty.go.mod"),
			expectedError:     internal.ErrModuleInfoNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mockExecCmd := mocks.NewExecCombinedOutput(t)
			mockExecCmd.On("CombinedOutput").Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).Once()

			mockExecCmdFunc := func(name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"list", "-m", "-json", fmt.Sprintf("%s@latest", tc.module)}, args)
				return mockExecCmd
			}

			toolchain := internal.NewGoToolchain(mockExecCmdFunc, nil, nil)
			modulePath, version, err := toolchain.GetLatestModuleVersion(tc.module)
			assert.Equal(t, tc.expectedModulePath, modulePath)
			assert.Equal(t, tc.expectedVersion, version)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetModuleFile(t *testing.T) {
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
		expectedError     error
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
		"error-parsing-module-download-error-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  errors.New("unexpected end of JSON input"),
		},
		"error-downloading-module": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"unexpected error"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  errors.New("unexpected error"),
		},
		"error-parsing-module-download-success-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedError: errors.New("unexpected end of JSON input"),
		},
		"error-reading-go-mod-file": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"GoMod":"./go.mod"}`)
			}(),
			expectedError: errors.New("open ./go.mod: no such file or directory"),
		},
		"error-parsing-go-mod-file": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: makeExecCmdOutput(t, "invalid.go.mod"),
			expectedError:     errors.New("go.mod:1: unknown directive: invalid"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mockExecCmd := mocks.NewExecCombinedOutput(t)
			mockExecCmd.On("CombinedOutput").Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).Once()

			mockExecCmdFunc := func(name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"mod", "download", "-json", fmt.Sprintf("%s@%s", tc.module, tc.version)}, args)
				return mockExecCmd
			}

			toolchain := internal.NewGoToolchain(mockExecCmdFunc, nil, nil)
			modFile, err := toolchain.GetModuleFile(tc.module, tc.version)
			assert.Equal(t, tc.expectedModFile, modFile)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetModuleOrigin(t *testing.T) {
	cases := map[string]struct {
		module            string
		version           string
		mockExecCmdOutput []byte
		mockExecCmdErr    error
		expectedModOrigin *internal.ModuleOrigin
		expectedError     error
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
				Ref:  ptr("refs/heads/v0.1.0"),
			},
		},
		"error-parsing-module-download-error-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  errors.New("unexpected end of JSON input"),
		},
		"error-module-not-found": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"not found"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  internal.ErrModuleNotFound,
		},
		"error-downloading-module": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(`{"Error":"unexpected error"}`)
			}(),
			mockExecCmdErr: errors.New("exit status 1"),
			expectedError:  errors.New("unexpected error"),
		},
		"error-parsing-module-download-success-response": {
			module: "example.com/mockorg/mockproj",
			mockExecCmdOutput: func() []byte {
				return []byte(``)
			}(),
			expectedError: errors.New("unexpected end of JSON input"),
		},
		"error-module-origin-not-available": {
			module:            "example.com/mockorg/mockproj",
			mockExecCmdOutput: []byte(`{"Origin":null}`),
			expectedError:     internal.ErrModuleOriginNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mockExecCmd := mocks.NewExecCombinedOutput(t)
			mockExecCmd.On("CombinedOutput").Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).Once()

			mockExecCmdFunc := func(name string, args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"mod", "download", "-json", fmt.Sprintf("%s@%s", tc.module, tc.version)}, args)
				return mockExecCmd
			}

			toolchain := internal.NewGoToolchain(mockExecCmdFunc, nil, nil)
			modOrigin, err := toolchain.GetModuleOrigin(tc.module, tc.version)
			assert.Equal(t, tc.expectedModOrigin, modOrigin)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	cases := map[string]struct {
		pkg            string
		version        string
		mockExecCmdErr error
		expectedError  error
	}{
		"success": {
			pkg:     "example.com/mockorg/mockproj",
			version: "v0.1.0",
		},
		"error-installing-binary": {
			pkg:            "example.com/mockorg/mockproj",
			version:        "v0.1.0",
			mockExecCmdErr: errors.New("unexpected error"),
			expectedError:  errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mockExecCmd := mocks.NewExecRun(t)
			mockExecCmd.On("Run").Return(tc.mockExecCmdErr).Once()

			mockExecCmdFunc := func(name string, args ...string) internal.ExecRun {
				assert.Equal(t, "go", name)
				assert.Equal(t, []string{"install", "-a", fmt.Sprintf("%s@%s", tc.pkg, tc.version)}, args)
				return mockExecCmd
			}

			toolchain := internal.NewGoToolchain(nil, mockExecCmdFunc, nil)
			err := toolchain.Install(tc.pkg, tc.version)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVulnCheck(t *testing.T) {
	cases := map[string]struct {
		path              string
		mockExecCmdOutput []byte
		mockExecCmdErr    error
		expectedVulns     []internal.Vulnerability
		expectedError     error
	}{
		"success-filter-vulns-by-affected-status": {
			path: "$HOME/go/bin/mockproj",
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
			path:              "$HOME/go/bin/mockproj",
			mockExecCmdOutput: []byte(`unexpected error`),
			mockExecCmdErr:    errors.New("unexpected error"),
			expectedError:     errors.New("unexpected error"),
		},
		"error-parsing-govulncheck-response": {
			path:              "$HOME/go/bin/mockproj",
			mockExecCmdOutput: []byte(``),
			expectedError:     errors.New("unexpected end of JSON input"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mockExecCmd := mocks.NewExecCombinedOutput(t)
			mockExecCmd.On("CombinedOutput").Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).Once()

			mockExecCmdFunc := func(args ...string) internal.ExecCombinedOutput {
				assert.Equal(t, []string{"-mode", "binary", "-format", "openvex", tc.path}, args)
				return mockExecCmd
			}

			toolchain := internal.NewGoToolchain(nil, nil, mockExecCmdFunc)
			vulns, err := toolchain.VulnCheck(tc.path)
			assert.Equal(t, tc.expectedVulns, vulns)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func ptr(v string) *string {
	return &v
}
