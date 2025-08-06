package internal_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/mocks"
)

//nolint:gochecknoglobals // test variables
var (
	binInfo1 = internal.BinaryInfo{
		Name:          "mockproj1",
		FullPath:      "/home/user/go/bin/mockproj1",
		PackagePath:   "example.com/mockorg/mockproj1/cmd/mockproj1",
		ModulePath:    "example.com/mockorg/mockproj1",
		ModuleVersion: "v0.1.0",
		ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:     "go1.24.5",
		OS:            "darwin",
		Arch:          "arm64",
		Feature:       "v8.0",
		EnvVars:       []string{"CGO_ENABLED=1"},
	}

	binInfo2 = internal.BinaryInfo{
		Name:          "mockproj2",
		FullPath:      "/home/user/go/bin/mockproj2",
		PackagePath:   "example.com/mockorg/mockproj2/cmd/mockproj2",
		ModulePath:    "example.com/mockorg/mockproj2",
		ModuleVersion: "v1.1.0",
		ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:     "go1.24.5",
		OS:            "darwin",
		Arch:          "arm64",
		Feature:       "v8.0",
		EnvVars:       []string{"CGO_ENABLED=1"},
	}

	binInfo3 = internal.BinaryInfo{
		Name:          "mockproj3",
		FullPath:      "/home/user/go/bin/mockproj3",
		PackagePath:   "example.com/mockorg/mockproj3/v2/cmd/mockproj3",
		ModulePath:    "example.com/mockorg/mockproj3/v2",
		ModuleVersion: "v2.1.0",
		ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:     "go1.24.5",
		OS:            "darwin",
		Arch:          "arm64",
		Feature:       "v8.0",
		EnvVars:       []string{"CGO_ENABLED=1"},
	}

	errMockWriteError = errors.New("write error")
)

type errorWriter struct{}

func (e *errorWriter) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (e *errorWriter) Write([]byte) (int, error) {
	return 0, errMockWriteError
}

type mockGetBinaryUpgradeInfoCall struct {
	info        internal.BinaryInfo
	upgradeInfo internal.BinaryUpgradeInfo
	err         error
}

type mockDiagnoseBinaryCall struct {
	bin  string
	info internal.BinaryDiagnostic
	err  error
}

type mockUpgradeBinaryCall struct {
	bin string
	err error
}

func TestGobin_DiagnoseBinaries(t *testing.T) {
	mockproj1Diagnostic := internal.BinaryDiagnostic{
		Name:      "mockproj1",
		NotInPath: true,
		DuplicatesInPath: []string{
			"/home/user/go/bin/mockproj1",
			"/usr/local/bin/mockproj1",
		},
		GoVersion: struct {
			Actual   string
			Expected string
		}{
			Actual:   "go1.24.5",
			Expected: "go1.23.11",
		},
		Platform: struct {
			Actual   string
			Expected string
		}{
			Actual:   "darwin/arm64",
			Expected: "linux/amd64",
		},
		IsPseudoVersion:       true,
		NotBuiltWithGoModules: false,
		IsOrphaned:            false,
		Retracted:             "mock rationale",
		Deprecated:            "mock deprecated",
		Vulnerabilities: []internal.Vulnerability{
			{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
		},
	}

	mockproj2Diagnostic := internal.BinaryDiagnostic{
		Name:      "mockproj2",
		NotInPath: true,
		DuplicatesInPath: []string{
			"/home/user/go/bin/mockproj2",
			"/usr/local/bin/mockproj2",
		},
		GoVersion: struct {
			Actual   string
			Expected string
		}{
			Actual:   "go1.24.5",
			Expected: "go1.23.11",
		},
		Platform: struct {
			Actual   string
			Expected string
		}{
			Actual:   "darwin/arm64",
			Expected: "linux/amd64",
		},
		IsPseudoVersion:       true,
		NotBuiltWithGoModules: false,
		IsOrphaned:            true,
		Retracted:             "",
		Deprecated:            "",
		Vulnerabilities: []internal.Vulnerability{
			{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
		},
	}

	mockproj3Diagnostic := internal.BinaryDiagnostic{
		Name:             "mockproj3",
		NotInPath:        false,
		DuplicatesInPath: nil,
		GoVersion: struct {
			Actual   string
			Expected string
		}{
			Actual:   "",
			Expected: "",
		},
		Platform: struct {
			Actual   string
			Expected string
		}{
			Actual:   "",
			Expected: "",
		},
		IsPseudoVersion:       false,
		NotBuiltWithGoModules: true,
		IsOrphaned:            false,
		Retracted:             "",
		Deprecated:            "",
		Vulnerabilities:       nil,
	}

	cases := map[string]struct {
		stdOut                       io.ReadWriter
		parallelism                  int
		mockGetBinFullPath           string
		mockGetBinFullPathErr        error
		callListBinariesFullPaths    bool
		mockListBinariesFullPaths    []string
		mockListBinariesFullPathsErr error
		mockDiagnoseBinaryCalls      []mockDiagnoseBinaryCall
		expectedErr                  error
		expectedStdOut               string
	}{
		"success": {
			stdOut:                    &bytes.Buffer{},
			parallelism:               1,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockDiagnoseBinaryCalls: []mockDiagnoseBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", info: mockproj1Diagnostic},
				{bin: "/home/user/go/bin/mockproj2", info: mockproj2Diagnostic},
				{bin: "/home/user/go/bin/mockproj3", info: mockproj3Diagnostic},
			},
			expectedStdOut: `🛠️  mockproj1
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj1
        • /usr/local/bin/mockproj1
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ pseudo-version
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj2
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj2
        • /usr/local/bin/mockproj2
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ pseudo-version
    ❗ orphaned: unknown source, likely built locally
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj3
    ❗ built without Go modules (GO111MODULE=off)

3 binaries checked, 3 with issues
`,
		},
		"success-with-parallelism": {
			stdOut:                    &bytes.Buffer{},
			parallelism:               2,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockDiagnoseBinaryCalls: []mockDiagnoseBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", info: mockproj1Diagnostic},
				{bin: "/home/user/go/bin/mockproj2", info: mockproj2Diagnostic},
				{bin: "/home/user/go/bin/mockproj3", info: mockproj3Diagnostic},
			},
			expectedStdOut: `🛠️  mockproj1
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj1
        • /usr/local/bin/mockproj1
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ pseudo-version
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj2
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj2
        • /usr/local/bin/mockproj2
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ pseudo-version
    ❗ orphaned: unknown source, likely built locally
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj3
    ❗ built without Go modules (GO111MODULE=off)

3 binaries checked, 3 with issues
`,
		},
		"partial-success-error-diagnose-binary": {
			stdOut:                    &bytes.Buffer{},
			parallelism:               1,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockDiagnoseBinaryCalls: []mockDiagnoseBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", info: mockproj1Diagnostic},
				{bin: "/home/user/go/bin/mockproj2", err: errors.New("unexpected error")},
				{bin: "/home/user/go/bin/mockproj3", info: mockproj3Diagnostic},
			},
			expectedErr: errors.New("unexpected error"),
			expectedStdOut: `🛠️  mockproj1
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj1
        • /usr/local/bin/mockproj1
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ pseudo-version
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj3
    ❗ built without Go modules (GO111MODULE=off)

2 binaries checked, 2 with issues
`,
		},
		"error-get-bin-full-path": {
			stdOut:                &bytes.Buffer{},
			mockGetBinFullPathErr: errors.New("unexpected error"),
			expectedErr:           errors.New("unexpected error"),
		},
		"error-list-binaries-full-paths": {
			stdOut:                       &bytes.Buffer{},
			mockGetBinFullPath:           "/home/user/go/bin",
			callListBinariesFullPaths:    true,
			mockListBinariesFullPathsErr: os.ErrNotExist,
			expectedErr:                  os.ErrNotExist,
		},
		"error-write-error": {
			stdOut:                    &errorWriter{},
			parallelism:               1,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
			},
			mockDiagnoseBinaryCalls: []mockDiagnoseBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", info: mockproj1Diagnostic},
				{bin: "/home/user/go/bin/mockproj2", info: mockproj2Diagnostic},
			},
			expectedErr: errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinFullPath().
				Return(tc.mockGetBinFullPath, tc.mockGetBinFullPathErr).
				Once()

			if tc.callListBinariesFullPaths {
				binaryManager.EXPECT().ListBinariesFullPaths(tc.mockGetBinFullPath).
					Return(tc.mockListBinariesFullPaths, tc.mockListBinariesFullPathsErr).
					Once()
			}

			for _, call := range tc.mockDiagnoseBinaryCalls {
				binaryManager.EXPECT().DiagnoseBinary(context.Background(), call.bin).
					Return(call.info, call.err).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, nil, tc.stdOut, nil)
			err := gobin.DiagnoseBinaries(context.Background(), tc.parallelism)
			assert.Equal(t, tc.expectedErr, err)

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_ListInstalledBinaries(t *testing.T) {
	cases := map[string]struct {
		stdOut                   io.ReadWriter
		mockGetAllBinaryInfos    []internal.BinaryInfo
		mockGetAllBinaryInfosErr error
		expectedErr              error
		expectedStdOut           string
	}{
		"success-no-outdated-binaries": {
			stdOut:                &bytes.Buffer{},
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			expectedStdOut: `Name      → Module                           @ Version
------------------------------------------------------
mockproj1 → example.com/mockorg/mockproj1    @ v0.1.0 
mockproj2 → example.com/mockorg/mockproj2    @ v1.1.0 
mockproj3 → example.com/mockorg/mockproj3/v2 @ v2.1.0 
`,
		},
		"error-get-all-binary-infos": {
			stdOut:                   &bytes.Buffer{},
			mockGetAllBinaryInfosErr: errors.New("unexpected error"),
			expectedErr:              errors.New("unexpected error"),
		},
		"error-write-error": {
			stdOut:                &errorWriter{},
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			expectedErr:           errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetAllBinaryInfos().
				Return(tc.mockGetAllBinaryInfos, tc.mockGetAllBinaryInfosErr).
				Once()

			gobin := internal.NewGobin(binaryManager, nil, nil, tc.stdOut, nil)
			err := gobin.ListInstalledBinaries()
			assert.Equal(t, tc.expectedErr, err)

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_ListOutdatedBinaries(t *testing.T) {
	cases := map[string]struct {
		stdOut                        io.ReadWriter
		checkMajor                    bool
		parallelism                   int
		mockGetAllBinaryInfos         []internal.BinaryInfo
		mockGetAllBinaryInfosErr      error
		mockGetBinaryUpgradeInfoCalls []mockGetBinaryUpgradeInfoCall
		expectedErr                   error
		expectedStdOut                string
	}{
		"success-no-outdated-binaries": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v2",
						LatestVersion:      "v2.1.0",
						IsUpgradeAvailable: false,
					},
				},
			},
			expectedStdOut: "✅ All binaries are up to date\n",
		},
		"success-no-outdated-binaries-skip-error-built-without-go-modules": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					err:  internal.ErrBinaryBuiltWithoutGoModules,
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v2",
						LatestVersion:      "v2.1.0",
						IsUpgradeAvailable: false,
					},
				},
			},
			expectedStdOut: "✅ All binaries are up to date\n",
		},
		"success-no-outdated-binaries-with-error": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					err:  internal.ErrModuleInfoNotAvailable,
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v2",
						LatestVersion:      "v2.1.0",
						IsUpgradeAvailable: false,
					},
				},
			},
			expectedErr: internal.ErrModuleInfoNotAvailable,
		},
		"success-minor-upgrades": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.2.0",
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v2",
						LatestVersion:      "v2.2.0",
						IsUpgradeAvailable: true,
					},
				},
			},
			expectedStdOut: `Name      → Module                           @ Current ↑ Latest
---------------------------------------------------------------
mockproj2 → example.com/mockorg/mockproj2    @ ` + "\033[31m" + `v1.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v1.2.0` + "\033[0m" + `
mockproj3 → example.com/mockorg/mockproj3/v2 @ ` + "\033[31m" + `v2.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v2.2.0` + "\033[0m" + `
`,
		},
		"success-major-upgrades": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            true,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.2.0",
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v3",
						LatestVersion:      "v3.1.0",
						IsUpgradeAvailable: true,
					},
				},
			},
			expectedStdOut: `Name      → Module                           @ Current ↑ Latest
---------------------------------------------------------------
mockproj2 → example.com/mockorg/mockproj2    @ ` + "\033[31m" + `v1.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v1.2.0` + "\033[0m" + `
mockproj3 → example.com/mockorg/mockproj3/v2 @ ` + "\033[31m" + `v2.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v3.1.0` + "\033[0m" + `
`,
		},
		"success-with-parallelism": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            true,
			parallelism:           2,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.2.0",
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v3",
						LatestVersion:      "v3.1.0",
						IsUpgradeAvailable: true,
					},
				},
			},
			expectedStdOut: `Name      → Module                           @ Current ↑ Latest
---------------------------------------------------------------
mockproj2 → example.com/mockorg/mockproj2    @ ` + "\033[31m" + `v1.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v1.2.0` + "\033[0m" + `
mockproj3 → example.com/mockorg/mockproj3/v2 @ ` + "\033[31m" + `v2.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v3.1.0` + "\033[0m" + `
`,
		},
		"partial-success-error-get-binary-upgrade-info": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            true,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.2.0",
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					err:  internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: internal.ErrModuleInfoNotAvailable,
			expectedStdOut: `Name      → Module                        @ Current ↑ Latest
------------------------------------------------------------
mockproj2 → example.com/mockorg/mockproj2 @ ` + "\033[31m" + `v1.1.0 ` + "\033[0m" + ` ↑ ` + "\033[32m" + `v1.2.0` + "\033[0m" + `
`,
		},
		"error-get-all-binary-infos": {
			stdOut:                   &bytes.Buffer{},
			checkMajor:               true,
			parallelism:              1,
			mockGetAllBinaryInfosErr: errors.New("unexpected error"),
			expectedErr:              errors.New("unexpected error"),
		},
		"error-write-error": {
			stdOut:                &errorWriter{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []internal.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo1,
						LatestModulePath:   "example.com/mockorg/mockproj1",
						LatestVersion:      "v0.1.0",
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo2,
						LatestModulePath:   "example.com/mockorg/mockproj2",
						LatestVersion:      "v1.2.0",
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: internal.BinaryUpgradeInfo{
						BinaryInfo:         binInfo3,
						LatestModulePath:   "example.com/mockorg/mockproj3/v2",
						LatestVersion:      "v2.2.0",
						IsUpgradeAvailable: true,
					},
				},
			},
			expectedErr: errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetAllBinaryInfos().
				Return(tc.mockGetAllBinaryInfos, tc.mockGetAllBinaryInfosErr).
				Once()

			for _, call := range tc.mockGetBinaryUpgradeInfoCalls {
				binaryManager.EXPECT().GetBinaryUpgradeInfo(
					context.Background(),
					call.info,
					tc.checkMajor,
				).Return(call.upgradeInfo, call.err).Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, nil, tc.stdOut, nil)
			err := gobin.ListOutdatedBinaries(context.Background(), tc.checkMajor, tc.parallelism)
			assert.Equal(t, tc.expectedErr, err)

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_PrintBinaryInfo(t *testing.T) {
	cases := map[string]struct {
		stdOut                io.ReadWriter
		binary                string
		mockGetBinFullPath    string
		mockGetBinFullPathErr error
		callGetBinaryInfo     bool
		mockGetBinaryInfo     internal.BinaryInfo
		mockGetBinaryInfoErr  error
		expectedErr           error
		expectedStdErr        string
		expectedStdOut        string
	}{
		"success-base-info": {
			stdOut:             &bytes.Buffer{},
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callGetBinaryInfo:  true,
			mockGetBinaryInfo: internal.BinaryInfo{
				Name:          "mockproj",
				FullPath:      "/home/user/go/bin/mockproj",
				PackagePath:   "example.com/mockorg/mockproj/cmd/mockproj",
				ModulePath:    "example.com/mockorg/mockproj",
				ModuleVersion: "v0.1.0",
				ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:     "go1.24.5",
				OS:            "darwin",
				Arch:          "arm64",
				Feature:       "v8.0",
				EnvVars:       []string{"CGO_ENABLED=1"},
			},
			expectedStdOut: `Path          /home/user/go/bin/mockproj
Package       example.com/mockorg/mockproj/cmd/mockproj
Module        example.com/mockorg/mockproj@v0.1.0
Module Sum    h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=
Go Version    go1.24.5
Platform      darwin/arm64/v8.0
Env Vars      CGO_ENABLED=1
`,
		},
		"success-all-info": {
			stdOut:             &bytes.Buffer{},
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callGetBinaryInfo:  true,
			mockGetBinaryInfo: internal.BinaryInfo{
				Name:           "mockproj",
				FullPath:       "/home/user/go/bin/mockproj",
				PackagePath:    "example.com/mockorg/mockproj/cmd/mockproj",
				ModulePath:     "example.com/mockorg/mockproj",
				ModuleVersion:  "v0.1.2-0.20250729191454-dac745d99aac",
				GoVersion:      "go1.24.5",
				CommitRevision: "dac745d99aacf872dd3232e7eceab0f9047051da",
				CommitTime:     "2025-07-29T19:14:54Z",
				OS:             "darwin",
				Arch:           "arm64",
				Feature:        "v8.0",
				EnvVars:        []string{"CGO_ENABLED=1"},
			},
			expectedStdOut: `Path          /home/user/go/bin/mockproj
Package       example.com/mockorg/mockproj/cmd/mockproj
Module        example.com/mockorg/mockproj@v0.1.2-0.20250729191454-dac745d99aac
Module Sum    <none>
Commit        dac745d99aacf872dd3232e7eceab0f9047051da (2025-07-29T19:14:54Z)
Go Version    go1.24.5
Platform      darwin/arm64/v8.0
Env Vars      CGO_ENABLED=1
`,
		},
		"error-get-bin-full-path": {
			stdOut:                &bytes.Buffer{},
			binary:                "mockproj1",
			mockGetBinFullPathErr: errors.New("unexpected error"),
			expectedErr:           errors.New("unexpected error"),
		},
		"error-get-binary-info": {
			stdOut:               &bytes.Buffer{},
			binary:               "mockproj1",
			mockGetBinFullPath:   "/home/user/go/bin",
			callGetBinaryInfo:    true,
			mockGetBinaryInfoErr: internal.ErrBinaryNotFound,
			expectedErr:          internal.ErrBinaryNotFound,
			expectedStdErr:       "❌ binary \"mockproj1\" not found\n",
		},
		"error-write-error": {
			stdOut:             &errorWriter{},
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callGetBinaryInfo:  true,
			mockGetBinaryInfo:  internal.BinaryInfo{},
			expectedErr:        errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinFullPath().
				Return(tc.mockGetBinFullPath, tc.mockGetBinFullPathErr).
				Once()

			if tc.callGetBinaryInfo {
				binaryManager.EXPECT().GetBinaryInfo(filepath.Join(tc.mockGetBinFullPath, tc.binary)).
					Return(tc.mockGetBinaryInfo, tc.mockGetBinaryInfoErr).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, tc.stdOut, nil)
			err := gobin.PrintBinaryInfo(tc.binary)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_PrintShortVersion(t *testing.T) {
	cases := map[string]struct {
		binary               string
		mockGetBinaryInfo    internal.BinaryInfo
		mockGetBinaryInfoErr error
		expectedErr          error
		expectedStdOut       string
	}{
		"success": {
			binary: "/home/user/go/bin/mockproj1",
			mockGetBinaryInfo: internal.BinaryInfo{
				ModuleVersion: "v1.0.0",
			},
			expectedStdOut: "v1.0.0\n",
		},
		"error-get-binary-info": {
			binary:               "/home/user/go/bin/mockproj1",
			mockGetBinaryInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedErr:          internal.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdOut bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinaryInfo(tc.binary).
				Return(tc.mockGetBinaryInfo, tc.mockGetBinaryInfoErr).
				Once()

			gobin := internal.NewGobin(binaryManager, nil, nil, &stdOut, nil)
			err := gobin.PrintShortVersion(tc.binary)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_PrintVersion(t *testing.T) {
	cases := map[string]struct {
		binary               string
		mockGetBinaryInfo    internal.BinaryInfo
		mockGetBinaryInfoErr error
		expectedErr          error
		expectedStdOut       string
	}{
		"success": {
			binary: "/home/user/go/bin/mockproj1",
			mockGetBinaryInfo: internal.BinaryInfo{
				ModuleVersion: "v1.0.0",
				GoVersion:     "go1.24.5",
				OS:            "linux",
				Arch:          "amd64",
			},
			expectedStdOut: "v1.0.0 (go1.24.5 linux/amd64)\n",
		},
		"error-get-binary-info": {
			binary:               "/home/user/go/bin/mockproj1",
			mockGetBinaryInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedErr:          internal.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdOut bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinaryInfo(tc.binary).
				Return(tc.mockGetBinaryInfo, tc.mockGetBinaryInfoErr).
				Once()

			gobin := internal.NewGobin(binaryManager, nil, nil, &stdOut, nil)
			err := gobin.PrintVersion(tc.binary)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_ShowBinaryRepository(t *testing.T) {
	cases := map[string]struct {
		binary                     string
		open                       bool
		mockGetBinaryRepository    string
		mockGetBinaryRepositoryErr error
		callRuntimeOS              bool
		mockRuntimeOS              string
		callExecCmd                bool
		mockExecCmdOutput          []byte
		mockExecCmdErr             error
		expectedErr                error
		expectedStdErr             string
		expectedStdOut             string
	}{
		"success-print-repository-url": {
			binary:                  "mockproj1",
			open:                    false,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			expectedStdOut:          "https://github.com/mockproj1\n",
		},
		"success-open-repository-url-darwin": {
			binary:                  "mockproj1",
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "darwin",
			callExecCmd:             true,
		},
		"success-open-repository-url-linux": {
			binary:                  "mockproj1",
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "linux",
			callExecCmd:             true,
		},
		"success-open-repository-url-windows": {
			binary:                  "mockproj1",
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "windows",
			callExecCmd:             true,
		},
		"error-binary-not-found": {
			binary:                     "mockproj1",
			open:                       true,
			mockGetBinaryRepositoryErr: internal.ErrBinaryNotFound,
			expectedErr:                internal.ErrBinaryNotFound,
			expectedStdErr:             "❌ binary \"mockproj1\" not found\n",
		},
		"error-unsupported-platform": {
			binary:                  "mockproj1",
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "unsupported",
			expectedErr:             errors.New("unsupported platform: unsupported"),
		},
		"error-open-repository-url": {
			binary:                  "mockproj1",
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "darwin",
			callExecCmd:             true,
			mockExecCmdOutput:       []byte("unexpected error"),
			mockExecCmdErr:          errors.New("exit status 1"),
			expectedErr:             errors.New("exit status 1: unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr, stdOut bytes.Buffer

			execCmd := mocks.NewExecCombinedOutput(t)

			if tc.callExecCmd {
				execCmd.EXPECT().CombinedOutput().
					Return(tc.mockExecCmdOutput, tc.mockExecCmdErr).
					Once()
			}

			execCmdFunc := func(_ context.Context, name string, args ...string) internal.ExecCombinedOutput {
				switch tc.mockRuntimeOS {
				case "darwin":
					assert.Equal(t, "open", name)
					assert.Equal(t, []string{"https://github.com/mockproj1"}, args)
				case "linux":
					assert.Equal(t, "xdg-open", name)
					assert.Equal(t, []string{"https://github.com/mockproj1"}, args)
				case "windows":
					assert.Equal(t, "cmd", name)
					assert.Equal(t, []string{"/c", "start", "https://github.com/mockproj1"}, args)
				}
				return execCmd
			}

			system := mocks.NewSystem(t)

			if tc.callRuntimeOS {
				system.EXPECT().RuntimeOS().
					Return(tc.mockRuntimeOS).
					Once()
			}

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinaryRepository(context.Background(), tc.binary).
				Return(tc.mockGetBinaryRepository, tc.mockGetBinaryRepositoryErr).
				Once()

			gobin := internal.NewGobin(binaryManager, execCmdFunc, &stdErr, &stdOut, system)
			err := gobin.ShowBinaryRepository(context.Background(), tc.binary, tc.open)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_UninstallBinary(t *testing.T) {
	cases := map[string]struct {
		binary                string
		mockGetBinFullPath    string
		mockGetBinFullPathErr error
		callRemove            bool
		mockRemoveErr         error
		expectedErr           error
		expectedStdErr        string
	}{
		"success": {
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callRemove:         true,
		},
		"error-get-bin-full-path": {
			binary:                "mockproj1",
			mockGetBinFullPathErr: errors.New("unexpected error"),
			expectedErr:           errors.New("unexpected error"),
		},
		"error-binary-not-found": {
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callRemove:         true,
			mockRemoveErr:      os.ErrNotExist,
			expectedErr:        os.ErrNotExist,
			expectedStdErr:     "❌ binary \"mockproj1\" not found\n",
		},
		"error-remove-binary": {
			binary:             "mockproj1",
			mockGetBinFullPath: "/home/user/go/bin",
			callRemove:         true,
			mockRemoveErr:      errors.New("unexpected error"),
			expectedErr:        errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinFullPath().
				Return(tc.mockGetBinFullPath, tc.mockGetBinFullPathErr).
				Once()

			system := mocks.NewSystem(t)

			if tc.callRemove {
				system.EXPECT().Remove(filepath.Join(tc.mockGetBinFullPath, tc.binary)).
					Return(tc.mockRemoveErr).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, system)
			err := gobin.UninstallBinary(tc.binary)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGobin_UpgradeBinaries(t *testing.T) {
	cases := map[string]struct {
		majorUpgrade                 bool
		rebuild                      bool
		parallelism                  int
		bins                         []string
		mockGetBinFullPath           string
		mockGetBinFullPathErr        error
		callListBinariesFullPaths    bool
		mockListBinariesFullPaths    []string
		mockListBinariesFullPathsErr error
		mockUpgradeBinaryCalls       []mockUpgradeBinaryCall
		expectedErr                  error
		expectedStdErr               string
	}{
		"success-all-bins": {
			parallelism:               1,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{bin: "/home/user/go/bin/mockproj1"},
				{bin: "/home/user/go/bin/mockproj2"},
				{bin: "/home/user/go/bin/mockproj3"},
			},
		},
		"success-specific-bins": {
			parallelism: 1,
			bins: []string{
				"mockproj2",
				"mockproj3",
			},
			mockGetBinFullPath: "/home/user/go/bin",
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{bin: "/home/user/go/bin/mockproj2"},
				{bin: "/home/user/go/bin/mockproj3"},
			},
		},
		"success-with-parallelism": {
			parallelism:               2,
			mockGetBinFullPath:        "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{bin: "/home/user/go/bin/mockproj1"},
				{bin: "/home/user/go/bin/mockproj2"},
				{bin: "/home/user/go/bin/mockproj3"},
			},
		},
		"error-get-bin-full-path": {
			mockGetBinFullPathErr: errors.New("unexpected error"),
			expectedErr:           errors.New("unexpected error"),
		},
		"error-list-binaries-full-paths": {
			parallelism:                  1,
			mockGetBinFullPath:           "/home/user/go/bin",
			callListBinariesFullPaths:    true,
			mockListBinariesFullPathsErr: errors.New("unexpected error"),
			expectedErr:                  errors.New("unexpected error"),
		},
		"error-upgrade-binary": {
			parallelism:        1,
			bins:               []string{"mockproj1"},
			mockGetBinFullPath: "/home/user/go/bin",
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", err: internal.ErrBinaryNotFound},
			},
			expectedErr:    internal.ErrBinaryNotFound,
			expectedStdErr: "❌ binary \"mockproj1\" not found\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetBinFullPath().
				Return(tc.mockGetBinFullPath, tc.mockGetBinFullPathErr).
				Once()

			if tc.callListBinariesFullPaths {
				binaryManager.EXPECT().ListBinariesFullPaths(tc.mockGetBinFullPath).
					Return(tc.mockListBinariesFullPaths, tc.mockListBinariesFullPathsErr).
					Once()
			}

			for _, call := range tc.mockUpgradeBinaryCalls {
				binaryManager.EXPECT().UpgradeBinary(
					context.Background(),
					call.bin,
					tc.majorUpgrade,
					tc.rebuild,
				).Return(call.err).Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, nil)
			err := gobin.UpgradeBinaries(
				context.Background(),
				tc.majorUpgrade,
				tc.rebuild,
				tc.parallelism,
				tc.bins...,
			)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}
