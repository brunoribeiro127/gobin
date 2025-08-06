package internal_test

import (
	"debug/buildinfo"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/mocks"
)

type mockGetBuildInfoCall struct {
	path string
	info *buildinfo.BuildInfo
	err  error
}

type mockGetLatestModuleVersionCall struct {
	module        string
	latestModule  string
	latestVersion string
	err           error
}

type mockStatInfoCall struct {
	path string
	info os.FileInfo
	err  error
}

//nolint:gocognit
func TestDiagnoseBinary(t *testing.T) {
	cases := map[string]struct {
		path                  string
		mockGetBuildInfo      *buildinfo.BuildInfo
		mockGetBuildInfoErr   error
		callRuntimeOSTimes    int
		mockRuntimeOS         string
		callRuntimeARCH       bool
		mockRuntimeARCH       string
		callGetEnvVar         bool
		mockGetEnvVarValue    string
		callPathListSeparator bool
		mockPathListSeparator rune
		mockStatInfoCalls     []mockStatInfoCall
		callRuntimeVersion    bool
		mockRuntimeVersion    string
		callLookPath          bool
		mockLookPathErr       error
		callGetModuleFile     bool
		mockGetModuleFile     *modfile.File
		mockGetModuleFileErr  error
		callVulnCheck         bool
		mockVulnCheckVulns    []internal.Vulnerability
		mockVulnCheckErr      error
		expectedDiagnostic    internal.BinaryDiagnostic
		expectedHasIssues     bool
		expectedError         error
	}{
		"success-has-issues": {
			path:                  "/home/user/go/bin/mockproj",
			mockGetBuildInfo:      getBuildInfo("mockproj", "v0.0.0-20250714171936-2fc2d3f24795"),
			callRuntimeOSTimes:    3,
			mockRuntimeOS:         "linux",
			callRuntimeARCH:       true,
			mockRuntimeARCH:       "amd64",
			callGetEnvVar:         true,
			mockGetEnvVarValue:    "/home/user/go/bin:/usr/local/bin",
			callPathListSeparator: true,
			mockPathListSeparator: ':',
			mockStatInfoCalls: []mockStatInfoCall{
				{
					path: "/home/user/go/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
				{
					path: "/usr/local/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
			},
			callRuntimeVersion: true,
			mockRuntimeVersion: "go1.23.11",
			callLookPath:       true,
			mockLookPathErr:    os.ErrNotExist,
			callGetModuleFile:  true,
			mockGetModuleFile: &modfile.File{
				Module: &modfile.Module{
					Deprecated: "mock deprecated",
				},
				Retract: []*modfile.Retract{
					{
						VersionInterval: modfile.VersionInterval{
							Low:  "v0.0.0-20250714171936-2fc2d3f24795",
							High: "v0.0.0-20250714171936-2fc2d3f24795",
						},
						Rationale: "mock rationale",
					},
				},
			},
			callVulnCheck: true,
			mockVulnCheckVulns: []internal.Vulnerability{
				{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
			},
			expectedDiagnostic: internal.BinaryDiagnostic{
				Name:      "mockproj",
				NotInPath: true,
				DuplicatesInPath: []string{
					"/home/user/go/bin/mockproj",
					"/usr/local/bin/mockproj",
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
			},
			expectedHasIssues: true,
		},
		"success-orphaned": {
			path: "/home/user/go/bin/mockproj",
			mockGetBuildInfo: func() *buildinfo.BuildInfo {
				info := getBuildInfo("mockproj", "v0.0.0-20250714171936-2fc2d3f24795")
				info.Main.Sum = ""
				return info
			}(),
			callRuntimeOSTimes:    3,
			mockRuntimeOS:         "linux",
			callRuntimeARCH:       true,
			mockRuntimeARCH:       "amd64",
			callGetEnvVar:         true,
			mockGetEnvVarValue:    "/home/user/go/bin:/usr/local/bin",
			callPathListSeparator: true,
			mockPathListSeparator: ':',
			mockStatInfoCalls: []mockStatInfoCall{
				{
					path: "/home/user/go/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
				{
					path: "/usr/local/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
			},
			callRuntimeVersion: true,
			mockRuntimeVersion: "go1.23.11",
			callLookPath:       true,
			mockLookPathErr:    os.ErrNotExist,
			callVulnCheck:      true,
			mockVulnCheckVulns: []internal.Vulnerability{
				{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
			},
			expectedDiagnostic: internal.BinaryDiagnostic{
				Name:      "mockproj",
				NotInPath: true,
				DuplicatesInPath: []string{
					"/home/user/go/bin/mockproj",
					"/usr/local/bin/mockproj",
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
			},
			expectedHasIssues: true,
		},
		"success-built-without-go-modules": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedDiagnostic: internal.BinaryDiagnostic{
				Name:             "mockproj",
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
			},
			expectedHasIssues: true,
		},
		"success-no-issues": {
			path:                  "/home/user/go/bin/mockproj",
			mockGetBuildInfo:      getBuildInfo("mockproj", "v0.1.0"),
			callRuntimeOSTimes:    2,
			mockRuntimeOS:         "darwin",
			callRuntimeARCH:       true,
			mockRuntimeARCH:       "arm64",
			callGetEnvVar:         true,
			mockGetEnvVarValue:    "/home/user/go/bin:/usr/local/bin",
			callPathListSeparator: true,
			mockPathListSeparator: ':',
			mockStatInfoCalls: []mockStatInfoCall{
				{
					path: "/home/user/go/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
				{
					path: "/usr/local/bin/mockproj",
					err:  os.ErrNotExist,
				},
			},
			callRuntimeVersion: true,
			mockRuntimeVersion: "go1.24.5",
			callLookPath:       true,
			callGetModuleFile:  true,
			mockGetModuleFile: &modfile.File{
				Module: &modfile.Module{},
			},
			callVulnCheck:      true,
			mockVulnCheckVulns: []internal.Vulnerability{},
			expectedDiagnostic: internal.BinaryDiagnostic{
				Name:             "mockproj",
				NotInPath:        false,
				DuplicatesInPath: nil,
				GoVersion: struct {
					Actual   string
					Expected string
				}{
					Actual:   "go1.24.5",
					Expected: "go1.24.5",
				},
				Platform: struct {
					Actual   string
					Expected string
				}{
					Actual:   "darwin/arm64",
					Expected: "darwin/arm64",
				},
				IsPseudoVersion:       false,
				NotBuiltWithGoModules: false,
				IsOrphaned:            false,
				Retracted:             "",
				Deprecated:            "",
				Vulnerabilities:       []internal.Vulnerability{},
			},
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: errors.New("unexpected error"),
			expectedError:       errors.New("unexpected error"),
		},
		"error-get-module-file": {
			path:                  "/home/user/go/bin/mockproj",
			mockGetBuildInfo:      getBuildInfo("mockproj", "v0.1.0"),
			callRuntimeOSTimes:    3,
			mockRuntimeOS:         "linux",
			callRuntimeARCH:       true,
			mockRuntimeARCH:       "amd64",
			callGetEnvVar:         true,
			mockGetEnvVarValue:    "/home/user/go/bin:/usr/local/bin",
			callPathListSeparator: true,
			mockPathListSeparator: ':',
			mockStatInfoCalls: []mockStatInfoCall{
				{
					path: "/home/user/go/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
				{
					path: "/usr/local/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
			},
			callRuntimeVersion:   true,
			mockRuntimeVersion:   "go1.23.11",
			callLookPath:         true,
			mockLookPathErr:      os.ErrNotExist,
			callGetModuleFile:    true,
			mockGetModuleFileErr: errors.New("unexpected error"),
			expectedError:        errors.New("unexpected error"),
		},
		"error-vuln-check": {
			path:                  "/home/user/go/bin/mockproj",
			mockGetBuildInfo:      getBuildInfo("mockproj", "v0.1.0"),
			callRuntimeOSTimes:    3,
			mockRuntimeOS:         "linux",
			callRuntimeARCH:       true,
			mockRuntimeARCH:       "amd64",
			callGetEnvVar:         true,
			mockGetEnvVarValue:    "/home/user/go/bin:/usr/local/bin",
			callPathListSeparator: true,
			mockPathListSeparator: ':',
			mockStatInfoCalls: []mockStatInfoCall{
				{
					path: "/home/user/go/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
				{
					path: "/usr/local/bin/mockproj",
					info: NewMockFileInfo("mockproj", os.FileMode(0755), false),
				},
			},
			callRuntimeVersion: true,
			mockRuntimeVersion: "go1.23.11",
			callLookPath:       true,
			mockLookPathErr:    os.ErrNotExist,
			callGetModuleFile:  true,
			mockGetModuleFile: &modfile.File{
				Module: &modfile.Module{
					Deprecated: "mock deprecated",
				},
				Retract: []*modfile.Retract{
					{
						VersionInterval: modfile.VersionInterval{
							Low:  "v0.1.0",
							High: "v0.1.0",
						},
						Rationale: "mock rationale",
					},
				},
			},
			callVulnCheck:    true,
			mockVulnCheckErr: errors.New("unexpected error"),
			expectedError:    errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			if tc.callRuntimeOSTimes > 0 {
				system.EXPECT().RuntimeOS().
					Return(tc.mockRuntimeOS).
					Times(tc.callRuntimeOSTimes)
			}

			if tc.callRuntimeARCH {
				system.EXPECT().RuntimeARCH().
					Return(tc.mockRuntimeARCH).
					Once()
			}

			if tc.callGetEnvVar {
				system.EXPECT().GetEnvVar("PATH").
					Return(tc.mockGetEnvVarValue, false).
					Once()
			}

			if tc.callPathListSeparator {
				system.EXPECT().PathListSeparator().
					Return(tc.mockPathListSeparator).
					Once()
			}

			for _, call := range tc.mockStatInfoCalls {
				system.EXPECT().Stat(call.path).
					Return(call.info, call.err).
					Once()
			}

			if tc.callRuntimeVersion {
				system.EXPECT().RuntimeVersion().
					Return(tc.mockRuntimeVersion).
					Once()
			}

			if tc.callLookPath {
				system.EXPECT().LookPath(filepath.Base(tc.path)).
					Return("", tc.mockLookPathErr).
					Once()
			}

			toolchain := mocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callGetModuleFile {
				toolchain.EXPECT().GetModuleFile(tc.mockGetBuildInfo.Main.Path, "latest").
					Return(tc.mockGetModuleFile, tc.mockGetModuleFileErr).
					Once()
			}

			if tc.callVulnCheck {
				toolchain.EXPECT().VulnCheck(tc.path).
					Return(tc.mockVulnCheckVulns, tc.mockVulnCheckErr).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain)
			diagnostic, err := binaryManager.DiagnoseBinary(tc.path)
			assert.Equal(t, tc.expectedDiagnostic, diagnostic)
			assert.Equal(t, tc.expectedHasIssues, diagnostic.HasIssues())
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGetAllBinaryInfos(t *testing.T) {
	cases := map[string]struct {
		mockUserHomeDir       string
		mockUserHomeDirErr    error
		callReadDir           bool
		mockReadDirEntries    []os.DirEntry
		mockReadDirErr        error
		mockStatInfoCalls     []mockStatInfoCall
		mockRuntimeOS         string
		mockRuntimeOSTimes    int
		mockGetBuildInfoCalls []mockGetBuildInfoCall
		expectedInfos         []internal.BinaryInfo
		expectedError         error
	}{
		"success": {
			mockUserHomeDir: "/home/user",
			callReadDir:     true,
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
				NewMockDirEntry("dir1"),
				NewMockDirEntry("file1"),
				NewMockDirEntry("bin2"),
				NewMockDirEntry("dir2"),
				NewMockDirEntry("file2"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1", info: NewMockFileInfo("bin1", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir1", info: NewMockFileInfo("dir1", os.ModeDir, true)},
				{path: "/home/user/go/bin/file1", info: NewMockFileInfo("file1", os.FileMode(0644), false)},
				{path: "/home/user/go/bin/bin2", info: NewMockFileInfo("bin2", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir2", info: NewMockFileInfo("dir2", os.ModeDir, true)},
				{path: "/home/user/go/bin/file2", info: NewMockFileInfo("file2", os.FileMode(0644), false)},
			},
			mockRuntimeOS:      "unix",
			mockRuntimeOSTimes: 4,
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{
					path: "/home/user/go/bin/bin1",
					info: getBuildInfo("bin1", "v0.1.0"),
				},
				{
					path: "/home/user/go/bin/bin2",
					info: getBuildInfo("bin2", "v0.1.0"),
				},
			},
			expectedInfos: []internal.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0"),
				getBinaryInfo("bin2", "v0.1.0"),
			},
		},
		"success-skip-get-binary-info-error": {
			mockUserHomeDir: "/home/user",
			callReadDir:     true,
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
				NewMockDirEntry("dir1"),
				NewMockDirEntry("file1"),
				NewMockDirEntry("bin2"),
				NewMockDirEntry("dir2"),
				NewMockDirEntry("file2"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1", info: NewMockFileInfo("bin1", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir1", info: NewMockFileInfo("dir1", os.ModeDir, true)},
				{path: "/home/user/go/bin/file1", info: NewMockFileInfo("file1", os.FileMode(0644), false)},
				{path: "/home/user/go/bin/bin2", info: NewMockFileInfo("bin2", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir2", info: NewMockFileInfo("dir2", os.ModeDir, true)},
				{path: "/home/user/go/bin/file2", info: NewMockFileInfo("file2", os.FileMode(0644), false)},
			},
			mockRuntimeOS:      "unix",
			mockRuntimeOSTimes: 4,
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{
					path: "/home/user/go/bin/bin1",
					info: getBuildInfo("bin1", "v0.1.0"),
				},
				{
					path: "/home/user/go/bin/bin2",
					err:  internal.ErrBinaryBuiltWithoutGoModules,
				},
			},
			expectedInfos: []internal.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0"),
			},
		},
		"error-get-bin-full-path": {
			mockUserHomeDirErr: errors.New("unexpected error"),
			expectedError:      errors.New("unexpected error"),
		},
		"error-list-binaries-full-paths": {
			mockUserHomeDir: "/home/user",
			callReadDir:     true,
			mockReadDirErr:  os.ErrNotExist,
			expectedError:   os.ErrNotExist,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			system.EXPECT().GetEnvVar("GOBIN").
				Return("", false).
				Once()

			system.EXPECT().GetEnvVar("GOPATH").
				Return("", false).
				Once()

			system.EXPECT().UserHomeDir().
				Return(tc.mockUserHomeDir, tc.mockUserHomeDirErr).
				Once()

			if tc.callReadDir {
				system.EXPECT().ReadDir(filepath.Join(tc.mockUserHomeDir, "go", "bin")).
					Return(tc.mockReadDirEntries, tc.mockReadDirErr).
					Once()
			}

			for _, call := range tc.mockStatInfoCalls {
				system.EXPECT().Stat(call.path).
					Return(call.info, call.err).
					Once()
			}

			if tc.mockRuntimeOSTimes > 0 {
				system.EXPECT().RuntimeOS().
					Return(tc.mockRuntimeOS).
					Times(tc.mockRuntimeOSTimes)
			}

			toolchain := mocks.NewToolchain(t)

			for _, call := range tc.mockGetBuildInfoCalls {
				toolchain.EXPECT().GetBuildInfo(call.path).
					Return(call.info, call.err).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain)
			infos, err := binaryManager.GetAllBinaryInfos()
			assert.Equal(t, tc.expectedInfos, infos)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGetBinaryInfo(t *testing.T) {
	cases := map[string]struct {
		path                string
		mockGetBuildInfo    *buildinfo.BuildInfo
		mockGetBuildInfoErr error
		expectedInfo        internal.BinaryInfo
		expectedError       error
	}{
		"success-base-info": {
			path: "/home/user/go/bin/mockproj",
			mockGetBuildInfo: &buildinfo.BuildInfo{
				Path: "example.com/mockorg/mockproj/cmd/mockproj",
				Main: debug.Module{
					Path:    "example.com/mockorg/mockproj",
					Version: "v0.1.0",
					Sum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				},
				GoVersion: "go1.24.5",
				Settings: []debug.BuildSetting{
					{Key: "GOOS", Value: "darwin"},
					{Key: "GOARCH", Value: "arm64"},
					{Key: "GOARM64", Value: "v8.0"},
					{Key: "CGO_ENABLED", Value: "1"},
				},
			},
			expectedInfo: internal.BinaryInfo{
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
		},
		"success-all-info": {
			path: "/home/user/go/bin/mockproj",
			mockGetBuildInfo: &buildinfo.BuildInfo{
				Path: "example.com/mockorg/mockproj/cmd/mockproj",
				Main: debug.Module{
					Path:    "example.com/mockorg/mockproj",
					Version: "v0.1.2-0.20250729191454-dac745d99aac",
				},
				GoVersion: "go1.24.5",
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "dac745d99aacf872dd3232e7eceab0f9047051da"},
					{Key: "vcs.time", Value: "2025-07-29T19:14:54Z"},
					{Key: "GOOS", Value: "darwin"},
					{Key: "GOARCH", Value: "arm64"},
					{Key: "GOARM64", Value: "v8.0"},
					{Key: "CGO_ENABLED", Value: "1"},
				},
			},
			expectedInfo: internal.BinaryInfo{
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
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedError:       internal.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toolchain := mocks.NewToolchain(t)
			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).Once()

			binaryManager := internal.NewGoBinaryManager(nil, toolchain)
			info, err := binaryManager.GetBinaryInfo(tc.path)
			assert.Equal(t, tc.expectedInfo, info)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGetBinaryRepository(t *testing.T) {
	cases := map[string]struct {
		binary                 string
		mockUserHomeDir        string
		mockUserHomeDirErr     error
		callGetBuildInfo       bool
		mockGetBuildInfo       *buildinfo.BuildInfo
		mockGetBuildInfoErr    error
		callGetModuleOrigin    bool
		mockGetModuleOrigin    *internal.ModuleOrigin
		mockGetModuleOriginErr error
		expectedRepository     string
		expectedError          error
	}{
		"success-module-origin": {
			binary:              "mockproj",
			mockUserHomeDir:     "/home/user",
			callGetBuildInfo:    true,
			mockGetBuildInfo:    getBuildInfo("mockproj", "v0.1.0"),
			callGetModuleOrigin: true,
			mockGetModuleOrigin: &internal.ModuleOrigin{
				URL: "https://github.com/mockorg/mockproj",
			},
			expectedRepository: "https://github.com/mockorg/mockproj",
		},
		"success-module-origin-not-available": {
			binary:                 "mockproj",
			mockUserHomeDir:        "/home/user",
			callGetBuildInfo:       true,
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: internal.ErrModuleOriginNotAvailable,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"success-module-not-found": {
			binary:                 "mockproj",
			mockUserHomeDir:        "/home/user",
			callGetBuildInfo:       true,
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: internal.ErrModuleNotFound,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"error-get-bin-full-path": {
			binary:             "mockproj",
			mockUserHomeDirErr: errors.New("unexpected error"),
			expectedError:      errors.New("unexpected error"),
		},
		"error-get-build-info": {
			binary:              "mockproj",
			mockUserHomeDir:     "/home/user",
			callGetBuildInfo:    true,
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedError:       internal.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-module-origin": {
			binary:                 "mockproj",
			mockUserHomeDir:        "/home/user",
			callGetBuildInfo:       true,
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: errors.New("unexpected error"),
			expectedError:          errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			system.EXPECT().GetEnvVar("GOBIN").
				Return("", false).
				Once()

			system.EXPECT().GetEnvVar("GOPATH").
				Return("", false).
				Once()

			system.EXPECT().UserHomeDir().
				Return(tc.mockUserHomeDir, tc.mockUserHomeDirErr).
				Once()

			toolchain := mocks.NewToolchain(t)

			if tc.callGetBuildInfo {
				toolchain.EXPECT().
					GetBuildInfo(filepath.Join(tc.mockUserHomeDir, "go", "bin", tc.binary)).
					Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
					Once()
			}

			if tc.callGetModuleOrigin {
				toolchain.EXPECT().GetModuleOrigin(
					tc.mockGetBuildInfo.Main.Path,
					tc.mockGetBuildInfo.Main.Version,
				).Return(tc.mockGetModuleOrigin, tc.mockGetModuleOriginErr).Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain)
			repository, err := binaryManager.GetBinaryRepository(tc.binary)
			assert.Equal(t, tc.expectedRepository, repository)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGetBinaryUpgradeInfo(t *testing.T) {
	cases := map[string]struct {
		info                            internal.BinaryInfo
		checkMajor                      bool
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		expectedInfo                    internal.BinaryUpgradeInfo
		expectedError                   error
	}{
		"success-check-minor-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0"),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v0.1.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
				{
					module: "example.com/mockorg/mockproj/v2",
					err:    internal.ErrModuleNotFound,
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0"),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v0.1.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available-v2": {
			info:       getBinaryInfo("mockproj", "v2.0.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj/v2",
					latestModule:  "example.com/mockorg/mockproj/v2",
					latestVersion: "v2.0.0",
				},
				{
					module: "example.com/mockorg/mockproj/v3",
					err:    internal.ErrModuleNotFound,
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v2.0.0"),
				LatestModulePath:   "example.com/mockorg/mockproj/v2",
				LatestVersion:      "v2.0.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-minor-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0"),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v1.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
				{
					module:        "example.com/mockorg/mockproj/v2",
					latestModule:  "example.com/mockorg/mockproj/v2",
					latestVersion: "v2.0.0",
				},
				{
					module: "example.com/mockorg/mockproj/v3",
					err:    internal.ErrModuleNotFound,
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0"),
				LatestModulePath:   "example.com/mockorg/mockproj/v2",
				LatestVersion:      "v2.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-multiple-major-upgrades-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
				{
					module:        "example.com/mockorg/mockproj/v2",
					latestModule:  "example.com/mockorg/mockproj/v2",
					latestVersion: "v2.0.0",
				},
				{
					module:        "example.com/mockorg/mockproj/v3",
					latestModule:  "example.com/mockorg/mockproj/v3",
					latestVersion: "v3.0.0",
				},
				{
					module: "example.com/mockorg/mockproj/v4",
					err:    internal.ErrModuleNotFound,
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0"),
				LatestModulePath:   "example.com/mockorg/mockproj/v3",
				LatestVersion:      "v3.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"error-get-latest-module-minor-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: "example.com/mockorg/mockproj",
					err:    internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedError: internal.ErrModuleInfoNotAvailable,
		},
		"error-get-latest-module-major-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0"),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
				{
					module: "example.com/mockorg/mockproj/v2",
					err:    internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedError: internal.ErrModuleInfoNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toolchain := mocks.NewToolchain(t)

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(call.module).
					Return(call.latestModule, call.latestVersion, call.err).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(nil, toolchain)
			info, err := binaryManager.GetBinaryUpgradeInfo(tc.info, tc.checkMajor)
			assert.Equal(t, tc.expectedInfo, info)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGetBinFullPath(t *testing.T) {
	cases := map[string]struct {
		mockGOBINEnvVar    string
		mockGOBINEnvVarOk  bool
		callGOPATHEnvVar   bool
		mockGOPATHEnvVar   string
		mockGOPATHEnvVarOk bool
		callUserHomeDir    bool
		mockUserHomeDir    string
		mockUserHomeDirErr error
		expectedPath       string
		expectedError      error
	}{
		"success-gobin-env-var": {
			mockGOBINEnvVar:   "/home/user/go/bin",
			mockGOBINEnvVarOk: true,
			expectedPath:      "/home/user/go/bin",
		},
		"success-gopath-env-var": {
			callGOPATHEnvVar:   true,
			mockGOPATHEnvVar:   "/home/user/go",
			mockGOPATHEnvVarOk: true,
			expectedPath:       "/home/user/go/bin",
		},
		"success-user-home-dir": {
			callGOPATHEnvVar: true,
			callUserHomeDir:  true,
			mockUserHomeDir:  "/home/user",
			expectedPath:     "/home/user/go/bin",
		},
		"error-user-home-dir": {
			callGOPATHEnvVar:   true,
			callUserHomeDir:    true,
			mockUserHomeDirErr: errors.New("unexpected error"),
			expectedError:      errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			system.EXPECT().GetEnvVar("GOBIN").
				Return(tc.mockGOBINEnvVar, tc.mockGOBINEnvVarOk).
				Once()

			if tc.callGOPATHEnvVar {
				system.EXPECT().GetEnvVar("GOPATH").
					Return(tc.mockGOPATHEnvVar, tc.mockGOPATHEnvVarOk).
					Once()
			}
			if tc.callUserHomeDir {
				system.EXPECT().UserHomeDir().
					Return(tc.mockUserHomeDir, tc.mockUserHomeDirErr).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, nil)
			path, err := binaryManager.GetBinFullPath()
			assert.Equal(t, tc.expectedPath, path)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestListBinariesFullPaths(t *testing.T) {
	cases := map[string]struct {
		dir                string
		mockReadDirEntries []os.DirEntry
		mockReadDirErr     error
		mockStatInfoCalls  []mockStatInfoCall
		mockRuntimeOS      string
		mockRuntimeOSTimes int
		expectedBinaries   []string
		expectedError      error
	}{
		"success-unix": {
			dir: "/home/user/go/bin",
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
				NewMockDirEntry("dir1"),
				NewMockDirEntry("file1"),
				NewMockDirEntry("bin2"),
				NewMockDirEntry("dir2"),
				NewMockDirEntry("file2"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1", info: NewMockFileInfo("bin1", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir1", info: NewMockFileInfo("dir1", os.ModeDir, true)},
				{path: "/home/user/go/bin/file1", info: NewMockFileInfo("file1", os.FileMode(0644), false)},
				{path: "/home/user/go/bin/bin2", info: NewMockFileInfo("bin2", os.FileMode(0755), false)},
				{path: "/home/user/go/bin/dir2", info: NewMockFileInfo("dir2", os.ModeDir, true)},
				{path: "/home/user/go/bin/file2", info: NewMockFileInfo("file2", os.FileMode(0644), false)},
			},
			mockRuntimeOS:      "unix",
			mockRuntimeOSTimes: 4,
			expectedBinaries: []string{
				"/home/user/go/bin/bin1",
				"/home/user/go/bin/bin2",
			},
		},
		"success-windows": {
			dir: "/home/user/go/bin",
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1.exe"),
				NewMockDirEntry("dir1"),
				NewMockDirEntry("file1"),
				NewMockDirEntry("bin2.exe"),
				NewMockDirEntry("dir2"),
				NewMockDirEntry("file2"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1.exe", info: NewMockFileInfo("bin1.exe", 0, false)},
				{path: "/home/user/go/bin/dir1", info: NewMockFileInfo("dir1", os.ModeDir, true)},
				{path: "/home/user/go/bin/file1", info: NewMockFileInfo("file1", 0, false)},
				{path: "/home/user/go/bin/bin2.exe", info: NewMockFileInfo("bin2.exe", 0, false)},
				{path: "/home/user/go/bin/dir2", info: NewMockFileInfo("dir2", os.ModeDir, true)},
				{path: "/home/user/go/bin/file2", info: NewMockFileInfo("file2", 0, false)},
			},
			mockRuntimeOS:      "windows",
			mockRuntimeOSTimes: 4,
			expectedBinaries: []string{
				"/home/user/go/bin/bin1.exe",
				"/home/user/go/bin/bin2.exe",
			},
		},
		"error-read-dir": {
			dir:            "/home/user/go/bin",
			mockReadDirErr: os.ErrNotExist,
			expectedError:  os.ErrNotExist,
		},
		"skip-stat-error": {
			dir: "/home/user/go/bin",
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1", err: os.ErrNotExist},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			system.EXPECT().ReadDir(tc.dir).
				Return(tc.mockReadDirEntries, tc.mockReadDirErr).
				Once()

			for _, call := range tc.mockStatInfoCalls {
				system.EXPECT().Stat(call.path).
					Return(call.info, call.err).
					Once()
			}

			if tc.mockRuntimeOSTimes > 0 {
				system.EXPECT().RuntimeOS().
					Return(tc.mockRuntimeOS).
					Times(tc.mockRuntimeOSTimes)
			}

			binaryManager := internal.NewGoBinaryManager(system, nil)
			binaries, err := binaryManager.ListBinariesFullPaths(tc.dir)
			assert.Equal(t, tc.expectedBinaries, binaries)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestUpgradeBinary(t *testing.T) {
	cases := map[string]struct {
		binFullPath                     string
		majorUpgrade                    bool
		rebuild                         bool
		mockGetBuildInfo                *buildinfo.BuildInfo
		mockGetBuildInfoErr             error
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		callInstall                     bool
		mockInstallPackage              string
		mockInstallVersion              string
		mockInstallErr                  error
		expectedError                   error
	}{
		"success-no-minor-upgrade-available": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     false,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
		},
		"success-no-major-upgrade-available": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     true,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
				{
					module: "example.com/mockorg/mockproj/v2",
					err:    internal.ErrModuleNotFound,
				},
			},
		},
		"success-no-upgrade-available-rebuild": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     false,
			rebuild:          true,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
			callInstall:        true,
			mockInstallPackage: "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion: "v0.1.0",
		},
		"success-minor-upgrade-available": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     false,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
			},
			callInstall:        true,
			mockInstallPackage: "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion: "v1.0.0",
		},
		"success-major-upgrade-available": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     true,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
				{
					module:        "example.com/mockorg/mockproj/v2",
					latestModule:  "example.com/mockorg/mockproj/v2",
					latestVersion: "v2.0.0",
				},
				{
					module: "example.com/mockorg/mockproj/v3",
					err:    internal.ErrModuleNotFound,
				},
			},
			callInstall:        true,
			mockInstallPackage: "example.com/mockorg/mockproj/v2/cmd/mockproj",
			mockInstallVersion: "v2.0.0",
		},
		"error-get-build-info": {
			binFullPath:         "/home/user/go/bin/mockproj",
			majorUpgrade:        false,
			rebuild:             false,
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedError:       internal.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-latest-module-version": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     false,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: "example.com/mockorg/mockproj",
					err:    internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedError: internal.ErrModuleInfoNotAvailable,
		},
		"error-install": {
			binFullPath:      "/home/user/go/bin/mockproj",
			majorUpgrade:     false,
			rebuild:          false,
			mockGetBuildInfo: getBuildInfo("mockproj", "v0.1.0"),
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
			},
			callInstall:        true,
			mockInstallPackage: "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion: "v1.0.0",
			mockInstallErr:     errors.New("unexpected error"),
			expectedError:      errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toolchain := mocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.binFullPath).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(call.module).
					Return(call.latestModule, call.latestVersion, call.err).
					Once()
			}

			if tc.callInstall {
				toolchain.EXPECT().Install(tc.mockInstallPackage, tc.mockInstallVersion).
					Return(tc.mockInstallErr).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(nil, toolchain)
			err := binaryManager.UpgradeBinary(tc.binFullPath, tc.majorUpgrade, tc.rebuild)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func getBinaryInfo(name, version string) internal.BinaryInfo {
	packagePath := "example.com/mockorg/mockproj/cmd/" + name
	modulePath := "example.com/mockorg/mockproj"
	if major := semver.Major(version); major != "v0" && major != "v1" {
		packagePath = modulePath + "/" + major + "/cmd/" + name
		modulePath = modulePath + "/" + major
	}

	return internal.BinaryInfo{
		Name:          name,
		FullPath:      "/home/user/go/bin/" + name,
		PackagePath:   packagePath,
		ModulePath:    modulePath,
		ModuleVersion: version,
		ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:     "go1.24.5",
		OS:            "darwin",
		Arch:          "arm64",
		Feature:       "v8.0",
		EnvVars:       []string{"CGO_ENABLED=1"},
	}
}

func getBuildInfo(name, version string) *buildinfo.BuildInfo {
	return &buildinfo.BuildInfo{
		Path: "example.com/mockorg/mockproj/cmd/" + name,
		Main: debug.Module{
			Path:    "example.com/mockorg/mockproj",
			Version: version,
			Sum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		},
		GoVersion: "go1.24.5",
		Settings: []debug.BuildSetting{
			{Key: "GOOS", Value: "darwin"},
			{Key: "GOARCH", Value: "arm64"},
			{Key: "GOARM64", Value: "v8.0"},
			{Key: "CGO_ENABLED", Value: "1"},
		},
	}
}
