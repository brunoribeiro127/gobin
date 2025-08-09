package internal_test

import (
	"context"
	"debug/buildinfo"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/mocks"
)

type mockDirEntry struct {
	name string
}

func NewMockDirEntry(name string) *mockDirEntry {
	return &mockDirEntry{name: name}
}

func (m *mockDirEntry) Name() string {
	return m.name
}

func (m *mockDirEntry) IsDir() bool {
	return false
}

func (m *mockDirEntry) Type() os.FileMode {
	return os.ModeIrregular
}

func (m *mockDirEntry) Info() (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

type mockFileInfo struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func NewMockFileInfo(
	name string,
	mode os.FileMode,
	isDir bool,
) *mockFileInfo {
	return &mockFileInfo{
		name:  name,
		mode:  mode,
		isDir: isDir,
	}
}

func (m *mockFileInfo) Name() string {
	return m.name
}

func (m *mockFileInfo) Size() int64 {
	return 0
}

func (m *mockFileInfo) Mode() os.FileMode {
	return m.mode
}

func (m *mockFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (m *mockFileInfo) IsDir() bool {
	return m.isDir
}

func (m *mockFileInfo) Sys() any {
	return nil
}

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

type mockReadlinkCall struct {
	path string
	link string
	err  error
}

type mockStatInfoCall struct {
	path string
	info os.FileInfo
	err  error
}

//nolint:gocognit
func TestGoBinaryManager_DiagnoseBinary(t *testing.T) {
	cases := map[string]struct {
		path                   string
		mockGetBuildInfo       *buildinfo.BuildInfo
		mockGetBuildInfoErr    error
		callRuntimeOSTimes     int
		mockRuntimeOS          string
		callRuntimeARCH        bool
		mockRuntimeARCH        string
		callGetEnvVar          bool
		mockGetEnvVarValue     string
		callPathListSeparator  bool
		mockPathListSeparator  rune
		mockStatInfoCalls      []mockStatInfoCall
		callRuntimeVersion     bool
		mockRuntimeVersion     string
		callGetInternalBinPath bool
		mockGetInternalBinPath string
		callLStat              bool
		mockLStatInfo          os.FileInfo
		mockLStatErr           error
		callReadlink           bool
		mockReadlink           string
		mockReadlinkErr        error
		callLookPath           bool
		mockLookPathErr        error
		callGetModuleFile      bool
		mockGetModuleFile      *modfile.File
		mockGetModuleFileErr   error
		callVulnCheck          bool
		mockVulnCheckVulns     []internal.Vulnerability
		mockVulnCheckErr       error
		expectedDiagnostic     internal.BinaryDiagnostic
		expectedHasIssues      bool
		expectedErr            error
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
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callLStat:              true,
			mockLStatInfo:          NewMockFileInfo("mockproj", os.ModeSymlink, false),
			callReadlink:           true,
			mockReadlink:           "/usr/local/bin/mockproj",
			callLookPath:           true,
			mockLookPathErr:        os.ErrNotExist,
			callGetModuleFile:      true,
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
				IsNotManaged:          true,
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
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callLStat:              true,
			mockLStatInfo:          NewMockFileInfo("mockproj", os.FileMode(0755), false),
			callLookPath:           true,
			callVulnCheck:          true,
			mockVulnCheckVulns: []internal.Vulnerability{
				{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
			},
			expectedDiagnostic: internal.BinaryDiagnostic{
				Name:      "mockproj",
				NotInPath: false,
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
				IsNotManaged:          true,
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
				IsNotManaged:          false,
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
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.24.5",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callLStat:              true,
			mockLStatInfo:          NewMockFileInfo("mockproj", os.ModeSymlink, false),
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callLookPath:           true,
			callGetModuleFile:      true,
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
				IsNotManaged:          false,
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
			expectedErr:         errors.New("unexpected error"),
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
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callLStat:              true,
			mockLStatInfo:          NewMockFileInfo("mockproj", os.ModeSymlink, false),
			callReadlink:           true,
			mockReadlinkErr:        os.ErrNotExist,
			callLookPath:           true,
			mockLookPathErr:        os.ErrNotExist,
			callGetModuleFile:      true,
			mockGetModuleFileErr:   errors.New("unexpected error"),
			expectedErr:            errors.New("unexpected error"),
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
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callLStat:              true,
			mockLStatErr:           os.ErrNotExist,
			callLookPath:           true,
			mockLookPathErr:        os.ErrNotExist,
			callGetModuleFile:      true,
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
			expectedErr:      errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			workspace := mocks.NewWorkspace(t)

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

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

			if tc.callLStat {
				system.EXPECT().LStat(tc.path).
					Return(tc.mockLStatInfo, tc.mockLStatErr).
					Once()
			}

			if tc.callReadlink {
				system.EXPECT().Readlink(tc.path).
					Return(tc.mockReadlink, tc.mockReadlinkErr).
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
				toolchain.EXPECT().GetModuleFile(
					context.Background(),
					tc.mockGetBuildInfo.Main.Path,
					"latest",
				).Return(tc.mockGetModuleFile, tc.mockGetModuleFileErr).Once()
			}

			if tc.callVulnCheck {
				toolchain.EXPECT().VulnCheck(context.Background(), tc.path).
					Return(tc.mockVulnCheckVulns, tc.mockVulnCheckErr).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			diagnostic, err := binaryManager.DiagnoseBinary(context.Background(), tc.path)
			assert.Equal(t, tc.expectedDiagnostic, diagnostic)
			assert.Equal(t, tc.expectedHasIssues, diagnostic.HasIssues())
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetAllBinaryInfos(t *testing.T) {
	cases := map[string]struct {
		managed                     bool
		mockGetGoBinPath            string
		mockGetInternalBinPathTimes int
		mockGetInternalBinPath      string
		mockReadDirEntries          []os.DirEntry
		mockReadDirErr              error
		mockStatInfoCalls           []mockStatInfoCall
		mockRuntimeOS               string
		mockRuntimeOSTimes          int
		mockGetBuildInfoCalls       []mockGetBuildInfoCall
		mockReadlinkCalls           []mockReadlinkCall
		expectedInfos               []internal.BinaryInfo
		expectedErr                 error
	}{
		"success-go-bin-path-binaries": {
			managed:                     false,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
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
				{path: "/home/user/go/bin/bin1", info: getBuildInfo("bin1", "v0.1.0")},
				{path: "/home/user/go/bin/bin2", info: getBuildInfo("bin2", "v0.1.0")},
			},
			mockReadlinkCalls: []mockReadlinkCall{
				{path: "/home/user/go/bin/bin1", link: "/home/user/.gobin/bin/bin1@v0.1.0"},
				{path: "/home/user/go/bin/bin2", err: os.ErrInvalid},
			},
			expectedInfos: []internal.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", false, true),
				getBinaryInfo("bin2", "v0.1.0", false, false),
			},
		},
		"success-internal-bin-path-binaries": {
			managed:                     true,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 3,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
				NewMockDirEntry("dir1"),
				NewMockDirEntry("file1"),
				NewMockDirEntry("bin2"),
				NewMockDirEntry("dir2"),
				NewMockDirEntry("file2"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/.gobin/bin/bin1", info: NewMockFileInfo("bin1", os.FileMode(0755), false)},
				{path: "/home/user/.gobin/bin/dir1", info: NewMockFileInfo("dir1", os.ModeDir, true)},
				{path: "/home/user/.gobin/bin/file1", info: NewMockFileInfo("file1", os.FileMode(0644), false)},
				{path: "/home/user/.gobin/bin/bin2", info: NewMockFileInfo("bin2", os.FileMode(0755), false)},
				{path: "/home/user/.gobin/bin/dir2", info: NewMockFileInfo("dir2", os.ModeDir, true)},
				{path: "/home/user/.gobin/bin/file2", info: NewMockFileInfo("file2", os.FileMode(0644), false)},
			},
			mockRuntimeOS:      "unix",
			mockRuntimeOSTimes: 4,
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{
					path: "/home/user/.gobin/bin/bin1",
					info: getBuildInfo("bin1", "v0.1.0"),
				},
				{
					path: "/home/user/.gobin/bin/bin2",
					info: getBuildInfo("bin2", "v0.1.0"),
				},
			},
			mockReadlinkCalls: []mockReadlinkCall{
				{path: "/home/user/.gobin/bin/bin1", link: "/home/user/.gobin/bin/bin1@v0.1.0"},
				{path: "/home/user/.gobin/bin/bin2", link: "/home/user/.gobin/bin/bin2@v0.1.0"},
			},
			expectedInfos: []internal.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", true, true),
				getBinaryInfo("bin2", "v0.1.0", true, true),
			},
		},
		"success-skip-get-binary-info-error": {
			managed:                     false,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
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
			mockReadlinkCalls: []mockReadlinkCall{
				{path: "/home/user/go/bin/bin1", link: "/home/user/.gobin/bin/bin1@v0.1.0"},
			},
			expectedInfos: []internal.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", false, true),
			},
		},
		"error-list-binaries-full-paths": {
			managed:          false,
			mockGetGoBinPath: "/home/user/go/bin",
			mockReadDirErr:   os.ErrNotExist,
			expectedErr:      os.ErrNotExist,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)
			toolchain := mocks.NewToolchain(t)
			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			mockReadDirPath := tc.mockGetGoBinPath
			if tc.managed {
				mockReadDirPath = tc.mockGetInternalBinPath
			}

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			system.EXPECT().ReadDir(mockReadDirPath).
				Return(tc.mockReadDirEntries, tc.mockReadDirErr).
				Once()

			for _, call := range tc.mockReadlinkCalls {
				system.EXPECT().Readlink(call.path).
					Return(call.link, call.err).
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

			for _, call := range tc.mockGetBuildInfoCalls {
				toolchain.EXPECT().GetBuildInfo(call.path).
					Return(call.info, call.err).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			infos, err := binaryManager.GetAllBinaryInfos(tc.managed)
			assert.Equal(t, tc.expectedInfos, infos)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryInfo(t *testing.T) {
	cases := map[string]struct {
		path                   string
		mockGetBuildInfo       *buildinfo.BuildInfo
		mockGetBuildInfoErr    error
		callReadlink           bool
		mockReadlink           string
		mockReadlinkErr        error
		callGetInternalBinPath bool
		mockGetInternalBinPath string
		expectedInfo           internal.BinaryInfo
		expectedErr            error
	}{
		"success-base-info-unmanaged-binary": {
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
			callReadlink:           true,
			mockReadlinkErr:        os.ErrInvalid,
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			expectedInfo: internal.BinaryInfo{
				Name:          "mockproj",
				FullPath:      "/home/user/go/bin/mockproj",
				InstallPath:   "/home/user/go/bin/mockproj",
				PackagePath:   "example.com/mockorg/mockproj/cmd/mockproj",
				ModulePath:    "example.com/mockorg/mockproj",
				ModuleVersion: "v0.1.0",
				ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:     "go1.24.5",
				OS:            "darwin",
				Arch:          "arm64",
				Feature:       "v8.0",
				EnvVars:       []string{"CGO_ENABLED=1"},
				IsManaged:     false,
			},
		},
		"success-base-info-managed-binary": {
			path: "/home/user/.gobin/bin/mockproj@v0.1.0",
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
			callReadlink:           true,
			mockReadlinkErr:        os.ErrInvalid,
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			expectedInfo: internal.BinaryInfo{
				Name:          "mockproj",
				FullPath:      "/home/user/.gobin/bin/mockproj@v0.1.0",
				InstallPath:   "/home/user/.gobin/bin/mockproj@v0.1.0",
				PackagePath:   "example.com/mockorg/mockproj/cmd/mockproj",
				ModulePath:    "example.com/mockorg/mockproj",
				ModuleVersion: "v0.1.0",
				ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:     "go1.24.5",
				OS:            "darwin",
				Arch:          "arm64",
				Feature:       "v8.0",
				EnvVars:       []string{"CGO_ENABLED=1"},
				IsManaged:     true,
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
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.2-0.20250729191454-dac745d99aac",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			expectedInfo: internal.BinaryInfo{
				Name:           "mockproj",
				FullPath:       "/home/user/go/bin/mockproj",
				InstallPath:    "/home/user/.gobin/bin/mockproj@v0.1.2-0.20250729191454-dac745d99aac",
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
				IsManaged:      true,
			},
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         internal.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)
			if tc.callReadlink {
				system.EXPECT().Readlink(tc.path).
					Return(tc.mockReadlink, tc.mockReadlinkErr).Once()
			}

			toolchain := mocks.NewToolchain(t)
			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).Once()

			workspace := mocks.NewWorkspace(t)
			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			info, err := binaryManager.GetBinaryInfo(tc.path)
			assert.Equal(t, tc.expectedInfo, info)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryRepository(t *testing.T) {
	cases := map[string]struct {
		binary                 string
		mockGetGoBinPath       string
		mockGetBuildInfo       *buildinfo.BuildInfo
		mockGetBuildInfoErr    error
		callReadlink           bool
		mockReadlink           string
		mockReadlinkErr        error
		callGetInternalBinPath bool
		mockGetInternalBinPath string
		callGetModuleOrigin    bool
		mockGetModuleOrigin    *internal.ModuleOrigin
		mockGetModuleOriginErr error
		expectedRepository     string
		expectedErr            error
	}{
		"success-module-origin": {
			binary:                 "mockproj",
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOrigin: &internal.ModuleOrigin{
				URL: "https://github.com/mockorg/mockproj",
			},
			expectedRepository: "https://github.com/mockorg/mockproj",
		},
		"success-module-origin-not-available": {
			binary:                 "mockproj",
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: internal.ErrModuleOriginNotAvailable,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"success-module-not-found": {
			binary:                 "mockproj",
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: internal.ErrModuleNotFound,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"error-get-build-info": {
			binary:              "mockproj",
			mockGetGoBinPath:    "/home/user/go/bin",
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         internal.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-module-origin": {
			binary:                 "mockproj",
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:           true,
			mockReadlink:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: errors.New("unexpected error"),
			expectedErr:            errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			if tc.callReadlink {
				system.EXPECT().Readlink(filepath.Join(tc.mockGetGoBinPath, tc.binary)).
					Return(tc.mockReadlink, tc.mockReadlinkErr).
					Once()
			}

			toolchain := mocks.NewToolchain(t)

			toolchain.EXPECT().
				GetBuildInfo(filepath.Join(tc.mockGetGoBinPath, tc.binary)).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callGetModuleOrigin {
				toolchain.EXPECT().GetModuleOrigin(
					context.Background(),
					tc.mockGetBuildInfo.Main.Path,
					tc.mockGetBuildInfo.Main.Version,
				).Return(tc.mockGetModuleOrigin, tc.mockGetModuleOriginErr).Once()
			}

			workspace := mocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			repository, err := binaryManager.GetBinaryRepository(context.Background(), tc.binary)
			assert.Equal(t, tc.expectedRepository, repository)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryUpgradeInfo(t *testing.T) {
	cases := map[string]struct {
		info                            internal.BinaryInfo
		checkMajor                      bool
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		expectedInfo                    internal.BinaryUpgradeInfo
		expectedErr                     error
	}{
		"success-check-minor-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v0.1.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
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
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v0.1.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available-v2": {
			info:       getBinaryInfo("mockproj", "v2.0.0", false, true),
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
				BinaryInfo:         getBinaryInfo("mockproj", "v2.0.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj/v2",
				LatestVersion:      "v2.0.0",
				IsUpgradeAvailable: false,
			},
		},
		"success-check-minor-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.0.0",
				},
			},
			expectedInfo: internal.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj",
				LatestVersion:      "v1.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
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
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj/v2",
				LatestVersion:      "v2.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-multiple-major-upgrades-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
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
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true),
				LatestModulePath:   "example.com/mockorg/mockproj/v3",
				LatestVersion:      "v3.0.0",
				IsUpgradeAvailable: true,
			},
		},
		"error-get-latest-module-minor-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: "example.com/mockorg/mockproj",
					err:    internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: internal.ErrModuleInfoNotAvailable,
		},
		"error-get-latest-module-major-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true),
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
			expectedErr: internal.ErrModuleInfoNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toolchain := mocks.NewToolchain(t)

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(context.Background(), call.module).
					Return(call.latestModule, call.latestVersion, call.err).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(nil, toolchain, nil)
			info, err := binaryManager.GetBinaryUpgradeInfo(
				context.Background(), tc.info, tc.checkMajor,
			)
			assert.Equal(t, tc.expectedInfo, info)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

//nolint:gocognit
func TestGoBinaryManager_InstallPackage(t *testing.T) {
	cases := map[string]struct {
		pkgVersion              string
		callGetInternalTempPath bool
		mockInternalTempPath    string
		callMkdirTemp           bool
		mockMkdirTempPattern    string
		mockMkdirTempPath       string
		mockMkdirTempErr        error
		callRemoveAll           bool
		mockRemoveAllErr        error
		callInstall             bool
		mockInstallPackage      string
		mockInstallVersion      string
		mockInstallErr          error
		callGetBuildInfo        bool
		mockGetBuildInfoPath    string
		mockGetBuildInfo        *buildinfo.BuildInfo
		mockGetBuildInfoErr     error
		callGetInternalBinPath  bool
		mockInternalBinPath     string
		callRename              bool
		mockRenameSrc           string
		mockRenameDst           string
		mockRenameErr           error
		callGetGoBinPath        bool
		mockGoBinPath           string
		callRemove              bool
		mockRemovePath          string
		mockRemoveErr           error
		callSymlink             bool
		mockSymlinkSrc          string
		mockSymlinkDst          string
		mockSymlinkErr          error
		expectedErr             error
	}{
		"success-package": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "latest",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"success-package-with-version": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.0.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.0.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"success-package-version-suffix-with-version": {
			pkgVersion:              "example.com/mockorg/mockproj/v2@v2.0.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/v2",
			mockInstallVersion:      "v2.0.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v2.0.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v2.0.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v2.0.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"error-mkdir-temp": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			mockMkdirTempErr:        os.ErrNotExist,
			expectedErr:             os.ErrNotExist,
		},
		"error-install": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			mockInstallErr:          errors.New("exit status 1: unexpected error"),
			expectedErr:             errors.New("exit status 1: unexpected error"),
		},
		"error-get-build-info": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.1.0"),
			mockGetBuildInfoErr:     os.ErrNotExist,
			expectedErr:             os.ErrNotExist,
		},
		"error-rename": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.1.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockRenameErr:           os.ErrExist,
			expectedErr:             os.ErrExist,
		},
		"error-remove": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.1.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			mockRemoveErr:           errors.New("unexpected error"),
			expectedErr:             errors.New("unexpected error"),
		},
		"error-symlink": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj@v1.1.0",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.1.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
			mockSymlinkErr:          os.ErrExist,
			expectedErr:             os.ErrExist,
		},
		"skip-error-remove-all": {
			pkgVersion:              "example.com/mockorg/mockproj/cmd/mockproj",
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			mockRemoveAllErr:        os.ErrNotExist,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "latest",
			callGetBuildInfo:        true,
			mockGetBuildInfoPath:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:        getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:  true,
			mockInternalBinPath:     "/home/user/.gobin/bin",
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			workspace := mocks.NewWorkspace(t)

			if tc.callGetInternalTempPath {
				workspace.EXPECT().GetInternalTempPath().
					Return(tc.mockInternalTempPath).
					Once()
			}

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockInternalBinPath).
					Once()
			}

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGoBinPath).
					Once()
			}

			system := mocks.NewSystem(t)

			if tc.callMkdirTemp {
				system.EXPECT().MkdirTemp(tc.mockInternalTempPath, tc.mockMkdirTempPattern).
					Return(tc.mockMkdirTempPath, tc.mockMkdirTempErr).Once()
			}

			if tc.callRemoveAll {
				system.EXPECT().RemoveAll(tc.mockMkdirTempPath).
					Return(tc.mockRemoveAllErr).Once()
			}

			if tc.callRename {
				system.EXPECT().Rename(tc.mockRenameSrc, tc.mockRenameDst).
					Return(tc.mockRenameErr).Once()
			}

			if tc.callRemove {
				system.EXPECT().Remove(tc.mockRemovePath).
					Return(tc.mockRemoveErr).Once()
			}

			if tc.callSymlink {
				system.EXPECT().Symlink(tc.mockSymlinkSrc, tc.mockSymlinkDst).
					Return(tc.mockSymlinkErr).Once()
			}

			toolchain := mocks.NewToolchain(t)

			if tc.callGetBuildInfo {
				toolchain.EXPECT().GetBuildInfo(tc.mockGetBuildInfoPath).
					Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
					Once()
			}

			if tc.callInstall {
				toolchain.EXPECT().Install(
					context.Background(),
					tc.mockMkdirTempPath,
					tc.mockInstallPackage,
					tc.mockInstallVersion,
				).Return(tc.mockInstallErr).Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			err := binaryManager.InstallPackage(context.Background(), tc.pkgVersion)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_ListBinariesFullPaths(t *testing.T) {
	cases := map[string]struct {
		dir                string
		mockReadDirEntries []os.DirEntry
		mockReadDirErr     error
		mockStatInfoCalls  []mockStatInfoCall
		mockRuntimeOS      string
		mockRuntimeOSTimes int
		expectedBinaries   []string
		expectedErr        error
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
			expectedErr:    os.ErrNotExist,
		},
		"skip-stat-error": {
			dir: "/home/user/go/bin",
			mockReadDirEntries: []os.DirEntry{
				NewMockDirEntry("bin1"),
			},
			mockStatInfoCalls: []mockStatInfoCall{
				{path: "/home/user/go/bin/bin1", err: os.ErrNotExist},
			},
			expectedBinaries: []string{},
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

			binaryManager := internal.NewGoBinaryManager(system, nil, nil)
			binaries, err := binaryManager.ListBinariesFullPaths(tc.dir)
			assert.Equal(t, tc.expectedBinaries, binaries)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_MigrateBinary(t *testing.T) {
	cases := map[string]struct {
		path                        string
		mockGetBuildInfo            *buildinfo.BuildInfo
		mockGetBuildInfoErr         error
		callReadlink                bool
		mockReadlink                string
		mockReadlinkErr             error
		mockGetInternalBinPathTimes int
		mockGetInternalBinPath      string
		callRename                  bool
		mockRenameDst               string
		mockRenameErr               error
		callSymlink                 bool
		mockSymlinkSrc              string
		mockSymlinkErr              error
		expectedErr                 error
	}{
		"success": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/go/bin/mockproj",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			callRename:                  true,
			mockRenameDst:               "/home/user/.gobin/bin/mockproj@v0.1.0",
			callSymlink:                 true,
			mockSymlinkSrc:              "/home/user/.gobin/bin/mockproj@v0.1.0",
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfo:    getBuildInfo("mockproj", "v0.1.0"),
			mockGetBuildInfoErr: internal.ErrBinaryNotFound,
			expectedErr:         internal.ErrBinaryNotFound,
		},
		"error-already-managed": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			expectedErr:                 internal.ErrBinaryAlreadyManaged,
		},
		"error-rename": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/go/bin/mockproj",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			callRename:                  true,
			mockRenameDst:               "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockRenameErr:               errors.New("unexpected error"),
			expectedErr:                 errors.New("unexpected error"),
		},
		"error-symlink": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/go/bin/mockproj",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			callRename:                  true,
			mockRenameDst:               "/home/user/.gobin/bin/mockproj@v0.1.0",
			callSymlink:                 true,
			mockSymlinkSrc:              "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockSymlinkErr:              errors.New("unexpected error"),
			expectedErr:                 errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			if tc.callReadlink {
				system.EXPECT().Readlink(tc.path).
					Return(tc.mockReadlink, tc.mockReadlinkErr).
					Once()
			}

			if tc.callRename {
				system.EXPECT().Rename(tc.path, tc.mockRenameDst).
					Return(tc.mockRenameErr).
					Once()
			}

			if tc.callSymlink {
				system.EXPECT().Symlink(tc.mockSymlinkSrc, tc.path).
					Return(tc.mockSymlinkErr).
					Once()
			}

			toolchain := mocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			workspace := mocks.NewWorkspace(t)

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			err := binaryManager.MigrateBinary(tc.path)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

//nolint:gocognit
func TestGoBinaryManager_UpgradeBinary(t *testing.T) {
	cases := map[string]struct {
		binFullPath                     string
		majorUpgrade                    bool
		rebuild                         bool
		mockGetBuildInfo                *buildinfo.BuildInfo
		mockGetBuildInfoErr             error
		callReadlink                    bool
		mockReadlink                    string
		mockReadlinkErr                 error
		mockGetInternalBinPathTimes     int
		mockGetInternalBinPath          string
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		callGetInternalTempPath         bool
		mockInternalTempPath            string
		callMkdirTemp                   bool
		mockMkdirTempPattern            string
		mockMkdirTempPath               string
		mockMkdirTempErr                error
		callRemoveAll                   bool
		mockRemoveAllErr                error
		callInstall                     bool
		mockInstallPackage              string
		mockInstallVersion              string
		mockInstallErr                  error
		callGetBuildInfo2               bool
		mockGetBuildInfo2Path           string
		mockGetBuildInfo2               *buildinfo.BuildInfo
		mockGetBuildInfo2Err            error
		callRename                      bool
		mockRenameSrc                   string
		mockRenameDst                   string
		mockRenameErr                   error
		callGetGoBinPath                bool
		mockGoBinPath                   string
		callRemove                      bool
		mockRemovePath                  string
		mockRemoveErr                   error
		callSymlink                     bool
		mockSymlinkSrc                  string
		mockSymlinkDst                  string
		mockSymlinkErr                  error
		expectedErr                     error
	}{
		"success-no-minor-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
		},
		"success-no-major-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
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
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     true,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v0.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v0.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v0.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"success-minor-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"success-major-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
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
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/v2/cmd/mockproj",
			mockInstallVersion:      "v2.0.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v2.0.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v2.0.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v2.0.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
		"error-get-build-info": {
			binFullPath:         "/home/user/go/bin/mockproj",
			majorUpgrade:        false,
			rebuild:             false,
			mockGetBuildInfoErr: internal.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         internal.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-latest-module-version": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: "example.com/mockorg/mockproj",
					err:    internal.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: internal.ErrModuleInfoNotAvailable,
		},
		"error-mkdir-temp": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			mockMkdirTempErr:        os.ErrNotExist,
			expectedErr:             os.ErrNotExist,
		},
		"error-install": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			mockInstallErr:          errors.New("exit status 1: unexpected error"),
			expectedErr:             errors.New("exit status 1: unexpected error"),
		},
		"error-get-build-info-2": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			mockGetBuildInfo2Err:    os.ErrNotExist,
			expectedErr:             os.ErrNotExist,
		},
		"error-rename": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockRenameErr:           os.ErrExist,
			expectedErr:             os.ErrExist,
		},
		"error-remove": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			mockRemoveErr:           errors.New("unexpected error"),
			expectedErr:             errors.New("unexpected error"),
		},
		"error-symlink": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
			mockSymlinkErr:          os.ErrExist,
			expectedErr:             os.ErrExist,
		},
		"skip-error-remove-all": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callReadlink:                true,
			mockReadlink:                "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:        "example.com/mockorg/mockproj",
					latestModule:  "example.com/mockorg/mockproj",
					latestVersion: "v1.1.0",
				},
			},
			callGetInternalTempPath: true,
			mockInternalTempPath:    "/home/user/.gobin/.tmp",
			callMkdirTemp:           true,
			mockMkdirTempPattern:    "mockproj-*",
			mockMkdirTempPath:       "/home/user/.gobin/.tmp/mockproj-0123456789",
			callRemoveAll:           true,
			mockRemoveAllErr:        os.ErrNotExist,
			callInstall:             true,
			mockInstallPackage:      "example.com/mockorg/mockproj/cmd/mockproj",
			mockInstallVersion:      "v1.1.0",
			callGetBuildInfo2:       true,
			mockGetBuildInfo2Path:   "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:       getBuildInfo("mockproj", "v1.1.0"),
			callRename:              true,
			mockRenameSrc:           "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockRenameDst:           "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:        true,
			mockGoBinPath:           "/home/user/go/bin",
			callRemove:              true,
			mockRemovePath:          "/home/user/go/bin/mockproj",
			callSymlink:             true,
			mockSymlinkSrc:          "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockSymlinkDst:          "/home/user/go/bin/mockproj",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			system := mocks.NewSystem(t)

			if tc.callReadlink {
				system.EXPECT().Readlink(tc.binFullPath).
					Return(tc.mockReadlink, tc.mockReadlinkErr).
					Once()
			}

			if tc.callMkdirTemp {
				system.EXPECT().MkdirTemp(tc.mockInternalTempPath, tc.mockMkdirTempPattern).
					Return(tc.mockMkdirTempPath, tc.mockMkdirTempErr).Once()
			}

			if tc.callRemoveAll {
				system.EXPECT().RemoveAll(tc.mockMkdirTempPath).
					Return(tc.mockRemoveAllErr).Once()
			}

			if tc.callRename {
				system.EXPECT().Rename(tc.mockRenameSrc, tc.mockRenameDst).
					Return(tc.mockRenameErr).Once()
			}

			if tc.callRemove {
				system.EXPECT().Remove(tc.mockRemovePath).
					Return(tc.mockRemoveErr).Once()
			}

			if tc.callSymlink {
				system.EXPECT().Symlink(tc.mockSymlinkSrc, tc.mockSymlinkDst).
					Return(tc.mockSymlinkErr).Once()
			}

			toolchain := mocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.binFullPath).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(context.Background(), call.module).
					Return(call.latestModule, call.latestVersion, call.err).
					Once()
			}

			if tc.callGetBuildInfo2 {
				toolchain.EXPECT().GetBuildInfo(tc.mockGetBuildInfo2Path).
					Return(tc.mockGetBuildInfo2, tc.mockGetBuildInfo2Err).
					Once()
			}

			if tc.callInstall {
				toolchain.EXPECT().Install(
					context.Background(),
					tc.mockMkdirTempPath,
					tc.mockInstallPackage,
					tc.mockInstallVersion,
				).Return(tc.mockInstallErr).Once()
			}

			workspace := mocks.NewWorkspace(t)

			if tc.callGetInternalTempPath {
				workspace.EXPECT().GetInternalTempPath().
					Return(tc.mockInternalTempPath).
					Once()
			}

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGoBinPath).
					Once()
			}

			binaryManager := internal.NewGoBinaryManager(system, toolchain, workspace)
			err := binaryManager.UpgradeBinary(
				context.Background(),
				tc.binFullPath,
				tc.majorUpgrade,
				tc.rebuild,
			)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func getBinaryInfo(
	name, version string,
	internalBinPath, managed bool,
) internal.BinaryInfo {
	packagePath := "example.com/mockorg/mockproj/cmd/" + name
	modulePath := "example.com/mockorg/mockproj"
	if major := semver.Major(version); major != "v0" && major != "v1" {
		packagePath = modulePath + "/" + major + "/cmd/" + name
		modulePath = modulePath + "/" + major
	}

	path := "/home/user/go/bin/" + name
	if internalBinPath {
		path = "/home/user/.gobin/bin/" + name
	}

	installPath := path
	if managed {
		installPath = "/home/user/.gobin/bin/" + name + "@" + version
	}

	return internal.BinaryInfo{
		Name:          name,
		FullPath:      path,
		InstallPath:   installPath,
		PackagePath:   packagePath,
		ModulePath:    modulePath,
		ModuleVersion: version,
		ModuleSum:     "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:     "go1.24.5",
		OS:            "darwin",
		Arch:          "arm64",
		Feature:       "v8.0",
		EnvVars:       []string{"CGO_ENABLED=1"},
		IsManaged:     managed,
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
