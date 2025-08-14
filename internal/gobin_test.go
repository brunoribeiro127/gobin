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
	"github.com/brunoribeiro127/gobin/internal/model"
)

//nolint:gochecknoglobals // test variables
var (
	binInfo1 = model.BinaryInfo{
		Name:        "mockproj1",
		FullPath:    "/home/user/go/bin/mockproj1",
		PackagePath: "example.com/mockorg/mockproj1/cmd/mockproj1",
		Module:      model.NewModule("example.com/mockorg/mockproj1", model.NewVersion("v0.1.0")),
		ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:   "go1.24.5",
		OS:          "darwin",
		Arch:        "arm64",
		Feature:     "v8.0",
		EnvVars:     []string{"CGO_ENABLED=1"},
	}

	binInfo2 = model.BinaryInfo{
		Name:        "mockproj2",
		FullPath:    "/home/user/go/bin/mockproj2",
		PackagePath: "example.com/mockorg/mockproj2/cmd/mockproj2",
		Module:      model.NewModule("example.com/mockorg/mockproj2", model.NewVersion("v1.1.0")),
		ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:   "go1.24.5",
		OS:          "darwin",
		Arch:        "arm64",
		Feature:     "v8.0",
		EnvVars:     []string{"CGO_ENABLED=1"},
	}

	binInfo3 = model.BinaryInfo{
		Name:        "mockproj3",
		FullPath:    "/home/user/go/bin/mockproj3",
		PackagePath: "example.com/mockorg/mockproj3/v2/cmd/mockproj3",
		Module:      model.NewModule("example.com/mockorg/mockproj3/v2", model.NewVersion("v2.1.0")),
		ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:   "go1.24.5",
		OS:          "darwin",
		Arch:        "arm64",
		Feature:     "v8.0",
		EnvVars:     []string{"CGO_ENABLED=1"},
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
	info        model.BinaryInfo
	upgradeInfo model.BinaryUpgradeInfo
	err         error
}

type mockDiagnoseBinaryCall struct {
	bin  string
	info model.BinaryDiagnostic
	err  error
}

type mockMigrateBinaryCall struct {
	path string
	err  error
}

type mockPinBinaryCall struct {
	bin  model.Binary
	kind model.Kind
	err  error
}

type mockUninstallBinaryCall struct {
	bin model.Binary
	err error
}

type mockUpgradeBinaryCall struct {
	path string
	err  error
}

func TestGobin_DiagnoseBinaries(t *testing.T) {
	mockproj1Diagnostic := model.BinaryDiagnostic{
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
		IsNotManaged:          true,
		IsPseudoVersion:       true,
		NotBuiltWithGoModules: false,
		IsOrphaned:            false,
		Retracted:             "mock rationale",
		Deprecated:            "mock deprecated",
		Vulnerabilities: []model.Vulnerability{
			{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
		},
	}

	mockproj2Diagnostic := model.BinaryDiagnostic{
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
		Vulnerabilities: []model.Vulnerability{
			{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
		},
	}

	mockproj3Diagnostic := model.BinaryDiagnostic{
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
		mockGetGoBinPath             string
		callListBinariesFullPaths    bool
		mockListBinariesFullPaths    []string
		mockListBinariesFullPathsErr error
		mockDiagnoseBinaryCalls      []mockDiagnoseBinaryCall
		expectedErr                  error
		expectedStdOut               string
		expectedStdErr               string
	}{
		"success": {
			stdOut:                    &bytes.Buffer{},
			parallelism:               1,
			mockGetGoBinPath:          "/home/user/go/bin",
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
    ❗ not managed by gobin
    ❗ pseudo-version
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj2
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj2
        • /usr/local/bin/mockproj2
    ❗ pseudo-version
    ❗ orphaned: unknown source, likely built locally
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
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
			mockGetGoBinPath:          "/home/user/go/bin",
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
    ❗ not managed by gobin
    ❗ pseudo-version
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj2
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj2
        • /usr/local/bin/mockproj2
    ❗ pseudo-version
    ❗ orphaned: unknown source, likely built locally
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
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
			mockGetGoBinPath:          "/home/user/go/bin",
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
			expectedErr:    errors.New("unexpected error"),
			expectedStdErr: "❌ error diagnosing binary \"mockproj2\"\n",
			expectedStdOut: `🛠️  mockproj1
    ❗ not in PATH
    ❗ duplicated in PATH:
        • /home/user/go/bin/mockproj1
        • /usr/local/bin/mockproj1
    ❗ not managed by gobin
    ❗ pseudo-version
    ❗ go version mismatch: expected go1.23.11, actual go1.24.5
    ❗ platform mismatch: expected linux/amd64, actual darwin/arm64
    ❗ retracted module version: mock rationale
    ❗ deprecated module: mock deprecated
    ❗ found 1 vulnerability:
        • GO-2025-3770 (https://pkg.go.dev/vuln/GO-2025-3770)
🛠️  mockproj3
    ❗ built without Go modules (GO111MODULE=off)

2 binaries checked, 2 with issues
`,
		},
		"success-no-issues": {
			stdOut:                    &bytes.Buffer{},
			parallelism:               1,
			mockGetGoBinPath:          "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockDiagnoseBinaryCalls: []mockDiagnoseBinaryCall{
				{bin: "/home/user/go/bin/mockproj1", info: model.BinaryDiagnostic{}},
				{bin: "/home/user/go/bin/mockproj2", info: model.BinaryDiagnostic{}},
				{bin: "/home/user/go/bin/mockproj3", info: model.BinaryDiagnostic{}},
			},
			expectedStdOut: "3 binaries checked, 0 with issues\n",
		},
		"error-list-binaries-full-paths": {
			stdOut:                       &bytes.Buffer{},
			mockGetGoBinPath:             "/home/user/go/bin",
			callListBinariesFullPaths:    true,
			mockListBinariesFullPathsErr: os.ErrNotExist,
			expectedErr:                  os.ErrNotExist,
		},
		"error-write-error": {
			stdOut:                    &errorWriter{},
			parallelism:               1,
			mockGetGoBinPath:          "/home/user/go/bin",
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
			var stdErr bytes.Buffer
			binaryManager := mocks.NewBinaryManager(t)
			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			if tc.callListBinariesFullPaths {
				binaryManager.EXPECT().ListBinariesFullPaths(tc.mockGetGoBinPath).
					Return(tc.mockListBinariesFullPaths, tc.mockListBinariesFullPathsErr).
					Once()
			}

			for _, call := range tc.mockDiagnoseBinaryCalls {
				binaryManager.EXPECT().DiagnoseBinary(context.Background(), call.bin).
					Return(call.info, call.err).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, tc.stdOut, nil, workspace)
			err := gobin.DiagnoseBinaries(context.Background(), tc.parallelism)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_InstallPackages(t *testing.T) {
	cases := map[string]struct {
		parallelism int
		packages    []model.Package
		expectedErr error
	}{
		"success-single-package": {
			parallelism: 1,
			packages: []model.Package{
				model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@latest"),
			},
		},
		"success-multiple-packages-with-parallelism": {
			parallelism: 2,
			packages: []model.Package{
				model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@latest"),
				model.NewPackage("example.com/mockorg/mockproj2/cmd/mockproj2@v1.1.0"),
			},
		},
		"error-install-package": {
			parallelism: 1,
			packages: []model.Package{
				model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@latest"),
			},
			expectedErr: errors.New("exit status 1: unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			binaryManager := mocks.NewBinaryManager(t)

			for _, pkg := range tc.packages {
				binaryManager.EXPECT().InstallPackage(context.Background(), pkg).
					Return(tc.expectedErr).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, nil, nil, nil, nil)
			err := gobin.InstallPackages(context.Background(), tc.parallelism, tc.packages...)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGobin_ListInstalledBinaries(t *testing.T) {
	cases := map[string]struct {
		stdOut                   io.ReadWriter
		managed                  bool
		mockGetAllBinaryInfos    []model.BinaryInfo
		mockGetAllBinaryInfosErr error
		expectedErr              error
		expectedStdOut           string
	}{
		"success-go-bin-path-binaries": {
			stdOut:  &bytes.Buffer{},
			managed: false,
			mockGetAllBinaryInfos: []model.BinaryInfo{
				getBinaryInfo("mockproj1", "v0.1.0", false, false),
				getBinaryInfo("mockproj2", "v1.1.0", false, false),
				getBinaryInfo("mockproj3", "v2.1.0", false, true),
			},
			expectedStdOut: `Name      → Module                          @ Version
-----------------------------------------------------
mockproj1 → example.com/mockorg/mockproj    @ v0.1.0 
mockproj2 → example.com/mockorg/mockproj    @ v1.1.0 
mockproj3 → example.com/mockorg/mockproj/v2 @ v2.1.0 
`,
		},
		"success-internal-bin-path-binaries": {
			stdOut:  &bytes.Buffer{},
			managed: true,
			mockGetAllBinaryInfos: []model.BinaryInfo{
				getBinaryInfo("mockproj", "v0.1.0", true, true),
				getBinaryInfo("mockproj", "v1.1.0", true, true),
				getBinaryInfo("mockproj", "v2.1.0", true, true),
			},
			expectedStdOut: `Name     → Module                          @ Version
----------------------------------------------------
mockproj → example.com/mockorg/mockproj/v2 @ v2.1.0 
mockproj → example.com/mockorg/mockproj    @ v1.1.0 
mockproj → example.com/mockorg/mockproj    @ v0.1.0 
`,
		},
		"error-get-all-binary-infos": {
			stdOut:                   &bytes.Buffer{},
			managed:                  false,
			mockGetAllBinaryInfosErr: errors.New("unexpected error"),
			expectedErr:              errors.New("unexpected error"),
		},
		"error-write-error": {
			stdOut:                &errorWriter{},
			managed:               false,
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			expectedErr:           errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			binaryManager := mocks.NewBinaryManager(t)

			binaryManager.EXPECT().GetAllBinaryInfos(tc.managed).
				Return(tc.mockGetAllBinaryInfos, tc.mockGetAllBinaryInfosErr).
				Once()

			gobin := internal.NewGobin(binaryManager, nil, nil, tc.stdOut, nil, nil)
			err := gobin.ListInstalledBinaries(tc.managed)
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
		mockGetAllBinaryInfos         []model.BinaryInfo
		mockGetAllBinaryInfosErr      error
		mockGetBinaryUpgradeInfoCalls []mockGetBinaryUpgradeInfoCall
		expectedErr                   error
		expectedStdOut                string
	}{
		"success-no-outdated-binaries": {
			stdOut:                &bytes.Buffer{},
			checkMajor:            false,
			parallelism:           1,
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v2",
							model.NewVersion("v2.1.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					err:  internal.ErrBinaryBuiltWithoutGoModules,
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v2",
							model.NewVersion("v2.1.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					err:  internal.ErrModuleInfoNotAvailable,
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v2",
							model.NewVersion("v2.1.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.2.0"),
						),
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v2",
							model.NewVersion("v2.2.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.2.0"),
						),
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v3",
							model.NewVersion("v3.1.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.2.0"),
						),
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v3",
							model.NewVersion("v3.1.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.2.0"),
						),
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
			mockGetAllBinaryInfos: []model.BinaryInfo{binInfo1, binInfo2, binInfo3},
			mockGetBinaryUpgradeInfoCalls: []mockGetBinaryUpgradeInfoCall{
				{
					info: binInfo1,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo1,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj1",
							model.NewVersion("v0.1.0"),
						),
						IsUpgradeAvailable: false,
					},
				},
				{
					info: binInfo2,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo2,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj2",
							model.NewVersion("v1.2.0"),
						),
						IsUpgradeAvailable: true,
					},
				},
				{
					info: binInfo3,
					upgradeInfo: model.BinaryUpgradeInfo{
						BinaryInfo: binInfo3,
						LatestModule: model.NewModule(
							"example.com/mockorg/mockproj3/v2",
							model.NewVersion("v2.2.0"),
						),
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

			binaryManager.EXPECT().GetAllBinaryInfos(false).
				Return(tc.mockGetAllBinaryInfos, tc.mockGetAllBinaryInfosErr).
				Once()

			for _, call := range tc.mockGetBinaryUpgradeInfoCalls {
				binaryManager.EXPECT().GetBinaryUpgradeInfo(
					context.Background(),
					call.info,
					tc.checkMajor,
				).Return(call.upgradeInfo, call.err).Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, nil, tc.stdOut, nil, nil)
			err := gobin.ListOutdatedBinaries(context.Background(), tc.checkMajor, tc.parallelism)
			assert.Equal(t, tc.expectedErr, err)

			bytes, err := io.ReadAll(tc.stdOut)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStdOut, string(bytes))
		})
	}
}

func TestGobin_MigrateBinaries(t *testing.T) {
	cases := map[string]struct {
		bins                         []model.Binary
		mockGetGoBinPath             string
		callListBinariesFullPaths    bool
		mockListBinariesFullPaths    []string
		mockListBinariesFullPathsErr error
		mockMigrateBinaryCalls       []mockMigrateBinaryCall
		expectedErr                  error
		expectedStdErr               string
	}{
		"success-single-binary": {
			bins:             []model.Binary{model.NewBinary("mockproj1")},
			mockGetGoBinPath: "/home/user/go/bin",
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
				},
			},
		},
		"success-multiple-binaries": {
			bins: []model.Binary{
				model.NewBinary("mockproj1"),
				model.NewBinary("mockproj2"),
				model.NewBinary("mockproj3"),
			},
			mockGetGoBinPath: "/home/user/go/bin",
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
				},
				{
					path: "/home/user/go/bin/mockproj2",
				},
				{
					path: "/home/user/go/bin/mockproj3",
				},
			},
		},
		"success-all-binaries": {
			mockGetGoBinPath:          "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
				},
				{
					path: "/home/user/go/bin/mockproj2",
				},
				{
					path: "/home/user/go/bin/mockproj3",
				},
			},
		},
		"success-skip-binary-already-managed": {
			bins:             []model.Binary{model.NewBinary("mockproj1")},
			mockGetGoBinPath: "/home/user/go/bin",
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
					err:  internal.ErrBinaryAlreadyManaged,
				},
			},
			expectedErr:    internal.ErrBinaryAlreadyManaged,
			expectedStdErr: "❌ binary \"mockproj1\" already managed\n",
		},
		"partial-success-skip-binary-not-found": {
			bins:             []model.Binary{model.NewBinary("mockproj1")},
			mockGetGoBinPath: "/home/user/go/bin",
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
					err:  internal.ErrBinaryNotFound,
				},
			},
			expectedErr:    internal.ErrBinaryNotFound,
			expectedStdErr: "❌ binary \"mockproj1\" not found\n",
		},
		"error-list-binaries-full-paths": {
			mockGetGoBinPath:             "/home/user/go/bin",
			callListBinariesFullPaths:    true,
			mockListBinariesFullPathsErr: errors.New("unexpected error"),
			expectedErr:                  errors.New("unexpected error"),
		},
		"error-migrate-binary": {
			mockGetGoBinPath:          "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{"/home/user/go/bin/mockproj1"},
			mockMigrateBinaryCalls: []mockMigrateBinaryCall{
				{
					path: "/home/user/go/bin/mockproj1",
					err:  errors.New("unexpected error"),
				},
			},
			expectedErr:    errors.New("unexpected error"),
			expectedStdErr: "❌ error migrating binary \"mockproj1\"\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			binaryManager := mocks.NewBinaryManager(t)

			if tc.callListBinariesFullPaths {
				binaryManager.EXPECT().ListBinariesFullPaths(tc.mockGetGoBinPath).
					Return(tc.mockListBinariesFullPaths, tc.mockListBinariesFullPathsErr).
					Once()
			}

			for _, call := range tc.mockMigrateBinaryCalls {
				binaryManager.EXPECT().MigrateBinary(call.path).
					Return(call.err).
					Once()
			}

			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, nil, workspace)
			err := gobin.MigrateBinaries(tc.bins...)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
		})
	}
}

func TestGobin_PinBinaries(t *testing.T) {
	cases := map[string]struct {
		kind               model.Kind
		bins               []model.Binary
		mockPinBinaryCalls []mockPinBinaryCall
		expectedErr        error
		expectedStdErr     string
	}{
		"success-single-binary-with-latest-kind": {
			kind: model.KindLatest,
			bins: []model.Binary{model.NewBinary("mockproj1")},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindLatest},
			},
		},
		"success-multiple-binaries-with-versions-and-latest-kind": {
			kind: model.KindLatest,
			bins: []model.Binary{
				model.NewBinary("mockproj1"),
				model.NewBinary("mockproj2@v1"),
				model.NewBinary("mockproj3@v2.1"),
			},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindLatest},
				{bin: model.NewBinary("mockproj2@v1"), kind: model.KindLatest},
				{bin: model.NewBinary("mockproj3@v2.1"), kind: model.KindLatest},
			},
		},
		"success-multiple-binaries-with-versions-and-major-kind": {
			kind: model.KindMajor,
			bins: []model.Binary{
				model.NewBinary("mockproj1"),
				model.NewBinary("mockproj2@v1"),
				model.NewBinary("mockproj3@v2.1"),
			},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindMajor},
				{bin: model.NewBinary("mockproj2@v1"), kind: model.KindMajor},
				{bin: model.NewBinary("mockproj3@v2.1"), kind: model.KindMajor},
			},
		},
		"success-multiple-binaries-with-versions-and-minor-kind": {
			kind: model.KindMinor,
			bins: []model.Binary{
				model.NewBinary("mockproj1"),
				model.NewBinary("mockproj2@v1"),
				model.NewBinary("mockproj3@v2.1"),
			},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindMinor},
				{bin: model.NewBinary("mockproj2@v1"), kind: model.KindMinor},
				{bin: model.NewBinary("mockproj3@v2.1"), kind: model.KindMinor},
			},
		},
		"error-pin-binary-not-found": {
			kind: model.KindLatest,
			bins: []model.Binary{model.NewBinary("mockproj1")},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindLatest, err: internal.ErrBinaryNotFound},
			},
			expectedErr:    internal.ErrBinaryNotFound,
			expectedStdErr: "❌ binary \"mockproj1\" not found\n",
		},
		"error-pin-binary-unexpected-error": {
			kind: model.KindLatest,
			bins: []model.Binary{model.NewBinary("mockproj1")},
			mockPinBinaryCalls: []mockPinBinaryCall{
				{bin: model.NewBinary("mockproj1"), kind: model.KindLatest, err: errors.New("unexpected error")},
			},
			expectedErr:    errors.New("unexpected error"),
			expectedStdErr: "❌ error pinning binary \"mockproj1\"\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer
			binaryManager := mocks.NewBinaryManager(t)

			for _, call := range tc.mockPinBinaryCalls {
				binaryManager.EXPECT().PinBinary(call.bin, tc.kind).
					Return(call.err).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, nil, nil)
			err := gobin.PinBinaries(tc.kind, tc.bins...)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
		})
	}
}

func TestGobin_PrintBinaryInfo(t *testing.T) {
	cases := map[string]struct {
		stdOut               io.ReadWriter
		binary               model.Binary
		mockGetGoBinPath     string
		callGetBinaryInfo    bool
		mockGetBinaryInfo    model.BinaryInfo
		mockGetBinaryInfoErr error
		expectedErr          error
		expectedStdErr       string
		expectedStdOut       string
	}{
		"success-base-info": {
			stdOut:            &bytes.Buffer{},
			binary:            model.NewBinary("mockproj1"),
			mockGetGoBinPath:  "/home/user/go/bin",
			callGetBinaryInfo: true,
			mockGetBinaryInfo: model.BinaryInfo{
				Name:        "mockproj",
				FullPath:    "/home/user/go/bin/mockproj",
				InstallPath: "/home/user/.gobin/bin/mockproj@v0.1.0",
				PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
				Module:      model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:   "go1.24.5",
				OS:          "darwin",
				Arch:        "arm64",
				Feature:     "v8.0",
				EnvVars:     []string{"CGO_ENABLED=1"},
			},
			expectedStdOut: `Path          /home/user/go/bin/mockproj
Location      /home/user/.gobin/bin/mockproj@v0.1.0
Package       example.com/mockorg/mockproj/cmd/mockproj
Module        example.com/mockorg/mockproj@v0.1.0
Module Sum    h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=
Go Version    go1.24.5
Platform      darwin/arm64/v8.0
Env Vars      CGO_ENABLED=1
`,
		},
		"success-all-info": {
			stdOut:            &bytes.Buffer{},
			binary:            model.NewBinary("mockproj1"),
			mockGetGoBinPath:  "/home/user/go/bin",
			callGetBinaryInfo: true,
			mockGetBinaryInfo: model.BinaryInfo{
				Name:        "mockproj",
				FullPath:    "/home/user/go/bin/mockproj",
				InstallPath: "/home/user/go/bin/mockproj",
				PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
				Module: model.NewModule(
					"example.com/mockorg/mockproj",
					model.NewVersion("v0.1.2-0.20250729191454-dac745d99aac"),
				),
				GoVersion:      "go1.24.5",
				CommitRevision: "dac745d99aacf872dd3232e7eceab0f9047051da",
				CommitTime:     "2025-07-29T19:14:54Z",
				OS:             "darwin",
				Arch:           "arm64",
				Feature:        "v8.0",
				EnvVars:        []string{"CGO_ENABLED=1"},
			},
			expectedStdOut: `Path          /home/user/go/bin/mockproj
Location      <unmanaged>
Package       example.com/mockorg/mockproj/cmd/mockproj
Module        example.com/mockorg/mockproj@v0.1.2-0.20250729191454-dac745d99aac
Module Sum    <none>
Commit        dac745d99aacf872dd3232e7eceab0f9047051da (2025-07-29T19:14:54Z)
Go Version    go1.24.5
Platform      darwin/arm64/v8.0
Env Vars      CGO_ENABLED=1
`,
		},
		"error-binary-not-found": {
			stdOut:               &bytes.Buffer{},
			binary:               model.NewBinary("mockproj1"),
			mockGetGoBinPath:     "/home/user/go/bin",
			callGetBinaryInfo:    true,
			mockGetBinaryInfoErr: internal.ErrBinaryNotFound,
			expectedErr:          internal.ErrBinaryNotFound,
			expectedStdErr:       "❌ binary \"mockproj1\" not found\n",
		},
		"error-get-binary-info": {
			stdOut:               &bytes.Buffer{},
			binary:               model.NewBinary("mockproj1"),
			mockGetGoBinPath:     "/home/user/go/bin",
			callGetBinaryInfo:    true,
			mockGetBinaryInfoErr: errors.New("unexpected error"),
			expectedErr:          errors.New("unexpected error"),
			expectedStdErr:       "❌ error getting info for binary \"mockproj1\"\n",
		},
		"error-write-error": {
			stdOut:            &errorWriter{},
			binary:            model.NewBinary("mockproj1"),
			mockGetGoBinPath:  "/home/user/go/bin",
			callGetBinaryInfo: true,
			mockGetBinaryInfo: model.BinaryInfo{},
			expectedErr:       errMockWriteError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			binaryManager := mocks.NewBinaryManager(t)

			if tc.callGetBinaryInfo {
				binaryManager.EXPECT().GetBinaryInfo(filepath.Join(tc.mockGetGoBinPath, tc.binary.Name)).
					Return(tc.mockGetBinaryInfo, tc.mockGetBinaryInfoErr).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, tc.stdOut, nil, workspace)
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
		mockGetBinaryInfo    model.BinaryInfo
		mockGetBinaryInfoErr error
		expectedErr          error
		expectedStdOut       string
	}{
		"success": {
			binary: "/home/user/go/bin/mockproj1",
			mockGetBinaryInfo: model.BinaryInfo{
				Module: model.NewModule(
					"example.com/mockorg/mockproj",
					model.NewVersion("v1.0.0"),
				),
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

			gobin := internal.NewGobin(binaryManager, nil, nil, &stdOut, nil, nil)
			err := gobin.PrintShortVersion(tc.binary)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_PrintVersion(t *testing.T) {
	cases := map[string]struct {
		binary               string
		mockGetBinaryInfo    model.BinaryInfo
		mockGetBinaryInfoErr error
		expectedErr          error
		expectedStdOut       string
	}{
		"success": {
			binary: "/home/user/go/bin/mockproj1",
			mockGetBinaryInfo: model.BinaryInfo{
				Module: model.NewModule(
					"example.com/mockorg/mockproj",
					model.NewVersion("v1.0.0"),
				),
				GoVersion: "go1.24.5",
				OS:        "linux",
				Arch:      "amd64",
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

			gobin := internal.NewGobin(binaryManager, nil, nil, &stdOut, nil, nil)
			err := gobin.PrintVersion(tc.binary)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_ShowBinaryRepository(t *testing.T) {
	cases := map[string]struct {
		binary                     model.Binary
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
			binary:                  model.NewBinary("mockproj1"),
			open:                    false,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			expectedStdOut:          "https://github.com/mockproj1\n",
		},
		"success-open-repository-url-darwin": {
			binary:                  model.NewBinary("mockproj1"),
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "darwin",
			callExecCmd:             true,
		},
		"success-open-repository-url-linux": {
			binary:                  model.NewBinary("mockproj1"),
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "linux",
			callExecCmd:             true,
		},
		"success-open-repository-url-windows": {
			binary:                  model.NewBinary("mockproj1"),
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "windows",
			callExecCmd:             true,
		},
		"error-binary-not-found": {
			binary:                     model.NewBinary("mockproj1"),
			open:                       true,
			mockGetBinaryRepositoryErr: internal.ErrBinaryNotFound,
			expectedErr:                internal.ErrBinaryNotFound,
			expectedStdErr:             "❌ binary \"mockproj1\" not found\n",
		},
		"error-get-binary-repository": {
			binary:                     model.NewBinary("mockproj1"),
			open:                       true,
			mockGetBinaryRepositoryErr: errors.New("unexpected error"),
			expectedErr:                errors.New("unexpected error"),
			expectedStdErr:             "❌ error getting repository for binary \"mockproj1\"\n",
		},
		"error-unsupported-platform": {
			binary:                  model.NewBinary("mockproj1"),
			open:                    true,
			mockGetBinaryRepository: "https://github.com/mockproj1",
			callRuntimeOS:           true,
			mockRuntimeOS:           "unsupported",
			expectedErr:             errors.New("unsupported platform: unsupported"),
		},
		"error-open-repository-url": {
			binary:                  model.NewBinary("mockproj1"),
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

			gobin := internal.NewGobin(binaryManager, execCmdFunc, &stdErr, &stdOut, system, nil)
			err := gobin.ShowBinaryRepository(context.Background(), tc.binary, tc.open)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedStdErr, stdErr.String())
			assert.Equal(t, tc.expectedStdOut, stdOut.String())
		})
	}
}

func TestGobin_UninstallBinaries(t *testing.T) {
	cases := map[string]struct {
		bins                     []model.Binary
		mockUninstallBinaryCalls []mockUninstallBinaryCall
		expectedErr              error
		expectedStdErr           string
	}{
		"success-no-binaries": {
			bins: []model.Binary{},
		},
		"success-single-binary": {
			bins:                     []model.Binary{model.NewBinary("mockproj1")},
			mockUninstallBinaryCalls: []mockUninstallBinaryCall{{bin: model.NewBinary("mockproj1")}},
		},
		"success-multiple-binaries": {
			bins: []model.Binary{
				model.NewBinary("mockproj1"),
				model.NewBinary("mockproj2"),
			},
			mockUninstallBinaryCalls: []mockUninstallBinaryCall{
				{bin: model.NewBinary("mockproj1")},
				{bin: model.NewBinary("mockproj2")},
			},
		},
		"error-binary-not-found": {
			bins: []model.Binary{model.NewBinary("mockproj1")},
			mockUninstallBinaryCalls: []mockUninstallBinaryCall{
				{bin: model.NewBinary("mockproj1"), err: os.ErrNotExist},
			},
			expectedErr:    os.ErrNotExist,
			expectedStdErr: "❌ binary \"mockproj1\" not found\n",
		},
		"error-remove-binary": {
			bins: []model.Binary{model.NewBinary("mockproj1")},
			mockUninstallBinaryCalls: []mockUninstallBinaryCall{
				{bin: model.NewBinary("mockproj1"), err: errors.New("unexpected error")},
			},
			expectedErr:    errors.New("unexpected error"),
			expectedStdErr: "❌ error uninstalling binary \"mockproj1\"\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer
			binaryManager := mocks.NewBinaryManager(t)

			for _, call := range tc.mockUninstallBinaryCalls {
				binaryManager.EXPECT().UninstallBinary(call.bin).
					Return(call.err).
					Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, nil, nil)
			err := gobin.UninstallBinaries(tc.bins...)
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
		bins                         []model.Binary
		mockGetGoBinPath             string
		callListBinariesFullPaths    bool
		mockListBinariesFullPaths    []string
		mockListBinariesFullPathsErr error
		mockUpgradeBinaryCalls       []mockUpgradeBinaryCall
		expectedErr                  error
		expectedStdErr               string
	}{
		"success-all-bins": {
			parallelism:               1,
			mockGetGoBinPath:          "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{path: "/home/user/go/bin/mockproj1"},
				{path: "/home/user/go/bin/mockproj2"},
				{path: "/home/user/go/bin/mockproj3"},
			},
		},
		"success-specific-bins": {
			parallelism: 1,
			bins: []model.Binary{
				model.NewBinary("mockproj2"),
				model.NewBinary("mockproj3"),
			},
			mockGetGoBinPath: "/home/user/go/bin",
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{path: "/home/user/go/bin/mockproj2"},
				{path: "/home/user/go/bin/mockproj3"},
			},
		},
		"success-with-parallelism": {
			parallelism:               2,
			mockGetGoBinPath:          "/home/user/go/bin",
			callListBinariesFullPaths: true,
			mockListBinariesFullPaths: []string{
				"/home/user/go/bin/mockproj1",
				"/home/user/go/bin/mockproj2",
				"/home/user/go/bin/mockproj3",
			},
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{path: "/home/user/go/bin/mockproj1"},
				{path: "/home/user/go/bin/mockproj2"},
				{path: "/home/user/go/bin/mockproj3"},
			},
		},
		"error-list-binaries-full-paths": {
			parallelism:                  1,
			mockGetGoBinPath:             "/home/user/go/bin",
			callListBinariesFullPaths:    true,
			mockListBinariesFullPathsErr: errors.New("unexpected error"),
			expectedErr:                  errors.New("unexpected error"),
		},
		"error-binary-not-found": {
			parallelism:      1,
			bins:             []model.Binary{model.NewBinary("mockproj1")},
			mockGetGoBinPath: "/home/user/go/bin",
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{path: "/home/user/go/bin/mockproj1", err: internal.ErrBinaryNotFound},
			},
			expectedErr:    internal.ErrBinaryNotFound,
			expectedStdErr: "❌ binary \"mockproj1\" not found\n",
		},
		"error-upgrade-binary": {
			parallelism:      1,
			bins:             []model.Binary{model.NewBinary("mockproj1")},
			mockGetGoBinPath: "/home/user/go/bin",
			mockUpgradeBinaryCalls: []mockUpgradeBinaryCall{
				{path: "/home/user/go/bin/mockproj1", err: errors.New("unexpected error")},
			},
			expectedErr:    errors.New("unexpected error"),
			expectedStdErr: "❌ error upgrading binary \"mockproj1\"\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdErr bytes.Buffer

			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			binaryManager := mocks.NewBinaryManager(t)

			if tc.callListBinariesFullPaths {
				binaryManager.EXPECT().ListBinariesFullPaths(tc.mockGetGoBinPath).
					Return(tc.mockListBinariesFullPaths, tc.mockListBinariesFullPathsErr).
					Once()
			}

			for _, call := range tc.mockUpgradeBinaryCalls {
				binaryManager.EXPECT().UpgradeBinary(
					context.Background(),
					call.path,
					tc.majorUpgrade,
					tc.rebuild,
				).Return(call.err).Once()
			}

			gobin := internal.NewGobin(binaryManager, nil, &stdErr, nil, nil, workspace)
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
