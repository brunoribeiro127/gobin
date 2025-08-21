package manager_test

import (
	"context"
	"debug/buildinfo"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/brunoribeiro127/gobin/internal/manager"
	"github.com/brunoribeiro127/gobin/internal/model"
	systemmocks "github.com/brunoribeiro127/gobin/internal/system/mocks"
	"github.com/brunoribeiro127/gobin/internal/toolchain"
	toolchainmocks "github.com/brunoribeiro127/gobin/internal/toolchain/mocks"
)

type mockGetBuildInfoCall struct {
	path string
	info *buildinfo.BuildInfo
	err  error
}

type mockGetLatestModuleVersionCall struct {
	module       model.Module
	latestModule model.Module
	err          error
}

type mockListBinariesCall struct {
	path     string
	binaries []string
	err      error
}

type mockGetSymlinkTargetCall struct {
	path   string
	target string
	err    error
}

//nolint:gocognit
func TestGoBinaryManager_DiagnoseBinary(t *testing.T) {
	cases := map[string]struct {
		path                   string
		mockGetBuildInfo       *buildinfo.BuildInfo
		mockGetBuildInfoErr    error
		callRuntimePlatform    bool
		mockRuntimePlatform    string
		callRuntimeVersion     bool
		mockRuntimeVersion     string
		callLocateBinaryInPath bool
		mockLocateBinaryInPath []string
		callGetInternalBinPath bool
		mockGetInternalBinPath string
		callIsSymlinkToDir     bool
		mockIsSymlinkToDir     bool
		mockIsSymlinkToDirErr  error
		callGetModuleFile      bool
		mockGetModuleFile      *modfile.File
		mockGetModuleFileErr   error
		callVulnCheck          bool
		mockVulnCheckVulns     []model.Vulnerability
		mockVulnCheckErr       error
		expectedDiagnostic     model.BinaryDiagnostic
		expectedHasIssues      bool
		expectedErr            error
	}{
		"success-has-issues": {
			path:                   "/home/user/go/bin/mockproj",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.0.0-20250714171936-2fc2d3f24795"),
			callRuntimePlatform:    true,
			mockRuntimePlatform:    "linux/amd64",
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callLocateBinaryInPath: true,
			mockLocateBinaryInPath: []string{
				"/home/user/go/bin/mockproj",
				"/usr/local/bin/mockproj",
			},
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callIsSymlinkToDir:     true,
			mockIsSymlinkToDir:     false,
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
			mockVulnCheckVulns: []model.Vulnerability{
				{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
			},
			expectedDiagnostic: model.BinaryDiagnostic{
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
				IsOrphaned:            false,
				Retracted:             "mock rationale",
				Deprecated:            "mock deprecated",
				Vulnerabilities: []model.Vulnerability{
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
			callRuntimePlatform:    true,
			mockRuntimePlatform:    "linux/amd64",
			callLocateBinaryInPath: true,
			mockLocateBinaryInPath: []string{
				"/home/user/go/bin/mockproj",
				"/usr/local/bin/mockproj",
			},
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callIsSymlinkToDir:     true,
			mockIsSymlinkToDir:     false,
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callVulnCheck:          true,
			mockVulnCheckVulns: []model.Vulnerability{
				{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
			},
			expectedDiagnostic: model.BinaryDiagnostic{
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
				Vulnerabilities: []model.Vulnerability{
					{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
				},
			},
			expectedHasIssues: true,
		},
		"success-built-without-go-modules": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: toolchain.ErrBinaryBuiltWithoutGoModules,
			expectedDiagnostic: model.BinaryDiagnostic{
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
			path:                   "/home/user/go/bin/mockproj",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callRuntimePlatform:    true,
			mockRuntimePlatform:    "darwin/arm64",
			callLocateBinaryInPath: true,
			mockLocateBinaryInPath: []string{
				"/home/user/go/bin/mockproj",
			},
			callIsSymlinkToDir:     true,
			mockIsSymlinkToDir:     true,
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.24.5",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleFile:      true,
			mockGetModuleFile: &modfile.File{
				Module: &modfile.Module{},
			},
			callVulnCheck:      true,
			mockVulnCheckVulns: []model.Vulnerability{},
			expectedDiagnostic: model.BinaryDiagnostic{
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
				Vulnerabilities:       []model.Vulnerability{},
			},
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: errors.New("unexpected error"),
			expectedErr:         errors.New("unexpected error"),
		},
		"error-get-module-file": {
			path:                   "/home/user/go/bin/mockproj",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callRuntimePlatform:    true,
			mockRuntimePlatform:    "linux/amd64",
			callLocateBinaryInPath: true,
			mockLocateBinaryInPath: []string{
				"/home/user/go/bin/mockproj",
				"/usr/local/bin/mockproj",
			},
			callIsSymlinkToDir:     true,
			mockIsSymlinkToDir:     false,
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleFile:      true,
			mockGetModuleFileErr:   errors.New("unexpected error"),
			expectedErr:            errors.New("unexpected error"),
		},
		"error-vuln-check": {
			path:                   "/home/user/go/bin/mockproj",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callRuntimePlatform:    true,
			mockRuntimePlatform:    "linux/amd64",
			callLocateBinaryInPath: true,
			mockLocateBinaryInPath: []string{
				"/home/user/go/bin/mockproj",
				"/usr/local/bin/mockproj",
			},
			callIsSymlinkToDir:     true,
			mockIsSymlinkToDir:     false,
			callRuntimeVersion:     true,
			mockRuntimeVersion:     "go1.23.11",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
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
			fs := systemmocks.NewFileSystem(t)
			runtime := systemmocks.NewRuntime(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callRuntimePlatform {
				runtime.EXPECT().Platform().
					Return(tc.mockRuntimePlatform).
					Once()
			}

			if tc.callRuntimeVersion {
				runtime.EXPECT().Version().
					Return(tc.mockRuntimeVersion).
					Once()
			}

			if tc.callLocateBinaryInPath {
				fs.EXPECT().LocateBinaryInPath(filepath.Base(tc.path)).
					Return(tc.mockLocateBinaryInPath).
					Once()
			}

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

			if tc.callIsSymlinkToDir {
				fs.EXPECT().IsSymlinkToDir(tc.path, tc.mockGetInternalBinPath).
					Return(tc.mockIsSymlinkToDir, tc.mockIsSymlinkToDirErr).
					Once()
			}

			if tc.callGetModuleFile {
				toolchain.EXPECT().GetModuleFile(
					context.Background(),
					model.NewLatestModule(tc.mockGetBuildInfo.Main.Path),
				).Return(tc.mockGetModuleFile, tc.mockGetModuleFileErr).Once()
			}

			if tc.callVulnCheck {
				toolchain.EXPECT().VulnCheck(context.Background(), tc.path).
					Return(tc.mockVulnCheckVulns, tc.mockVulnCheckErr).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, runtime, toolchain, workspace)
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
		callGetGoBinPathTimes       int
		mockGetGoBinPath            string
		mockGetInternalBinPathTimes int
		mockGetInternalBinPath      string
		mockListBinariesCalls       []mockListBinariesCall
		mockGetBuildInfoCalls       []mockGetBuildInfoCall
		mockGetSymlinkTargetCalls   []mockGetSymlinkTargetCall
		expectedInfos               []model.BinaryInfo
		expectedErr                 error
	}{
		"success-go-bin-path-binaries": {
			managed:                     false,
			callGetGoBinPathTimes:       1,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockListBinariesCalls: []mockListBinariesCall{
				{
					path:     "/home/user/go/bin",
					binaries: []string{"/home/user/go/bin/bin1", "/home/user/go/bin/bin2"},
				},
			},
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{path: "/home/user/go/bin/bin1", info: getBuildInfo("bin1", "v0.1.0")},
				{path: "/home/user/go/bin/bin2", info: getBuildInfo("bin2", "v0.1.0")},
			},
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/go/bin/bin1", target: "/home/user/.gobin/bin/bin1@v0.1.0"},
				{path: "/home/user/go/bin/bin2", err: os.ErrNotExist},
			},
			expectedInfos: []model.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", false, true, false),
				getBinaryInfo("bin2", "v0.1.0", false, false, false),
			},
		},
		"success-internal-bin-path-binaries": {
			managed:                     true,
			callGetGoBinPathTimes:       3,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 3,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockListBinariesCalls: []mockListBinariesCall{
				{
					path:     "/home/user/.gobin/bin",
					binaries: []string{"/home/user/.gobin/bin/bin1@v0.1.0", "/home/user/.gobin/bin/bin2@v0.1.0"},
				},
				{
					path:     "/home/user/go/bin",
					binaries: []string{"/home/user/go/bin/bin1", "/home/user/go/bin/bin2"},
				},
				{
					path:     "/home/user/go/bin",
					binaries: []string{"/home/user/go/bin/bin1", "/home/user/go/bin/bin2"},
				},
			},
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{
					path: "/home/user/.gobin/bin/bin1@v0.1.0",
					info: getBuildInfo("bin1", "v0.1.0"),
				},
				{
					path: "/home/user/.gobin/bin/bin2@v0.1.0",
					info: getBuildInfo("bin2", "v0.1.0"),
				},
			},
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/.gobin/bin/bin1@v0.1.0", err: os.ErrNotExist},
				{path: "/home/user/.gobin/bin/bin2@v0.1.0", err: os.ErrNotExist},
				{path: "/home/user/go/bin/bin1", err: os.ErrNotExist},
				{path: "/home/user/go/bin/bin2", target: "/home/user/.gobin/bin/bin2@v0.1.0"},
				{path: "/home/user/go/bin/bin1", err: os.ErrNotExist},
				{path: "/home/user/go/bin/bin2", target: "/home/user/.gobin/bin/bin2@v0.1.0"},
			},
			expectedInfos: []model.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", true, true, false),
				getBinaryInfo("bin2", "v0.1.0", true, true, true),
			},
		},
		"success-skip-get-binary-info-error": {
			managed:                     false,
			callGetGoBinPathTimes:       1,
			mockGetGoBinPath:            "/home/user/go/bin",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockListBinariesCalls: []mockListBinariesCall{
				{
					path:     "/home/user/go/bin",
					binaries: []string{"/home/user/go/bin/bin1", "/home/user/go/bin/bin2"},
				},
			},
			mockGetBuildInfoCalls: []mockGetBuildInfoCall{
				{
					path: "/home/user/go/bin/bin1",
					info: getBuildInfo("bin1", "v0.1.0"),
				},
				{
					path: "/home/user/go/bin/bin2",
					err:  toolchain.ErrBinaryBuiltWithoutGoModules,
				},
			},
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/go/bin/bin1", target: "/home/user/.gobin/bin/bin1@v0.1.0"},
			},
			expectedInfos: []model.BinaryInfo{
				getBinaryInfo("bin1", "v0.1.0", false, true, false),
			},
		},
		"error-list-binaries": {
			managed:               false,
			callGetGoBinPathTimes: 1,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockListBinariesCalls: []mockListBinariesCall{
				{
					path: "/home/user/go/bin",
					err:  os.ErrNotExist,
				},
			},
			expectedErr: os.ErrNotExist,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Times(tc.callGetGoBinPathTimes)

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			for _, call := range tc.mockListBinariesCalls {
				fs.EXPECT().ListBinaries(call.path).
					Return(call.binaries, call.err).
					Once()
			}

			for _, call := range tc.mockGetBuildInfoCalls {
				toolchain.EXPECT().GetBuildInfo(call.path).
					Return(call.info, call.err).
					Once()
			}

			for _, call := range tc.mockGetSymlinkTargetCalls {
				fs.EXPECT().GetSymlinkTarget(call.path).
					Return(call.target, call.err).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			infos, err := binaryManager.GetAllBinaryInfos(tc.managed)
			assert.Equal(t, tc.expectedInfos, infos)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryInfo(t *testing.T) {
	cases := map[string]struct {
		path                      string
		mockGetBuildInfo          *buildinfo.BuildInfo
		mockGetBuildInfoErr       error
		mockGetSymlinkTargetCalls []mockGetSymlinkTargetCall
		callGetInternalBinPath    bool
		mockGetInternalBinPath    string
		callGetGoBinPath          bool
		mockGetGoBinPath          string
		callListBinaries          bool
		mockListBinaries          []string
		mockListBinariesErr       error
		expectedInfo              model.BinaryInfo
		expectedErr               error
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
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/go/bin/mockproj", err: os.ErrInvalid},
			},
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			expectedInfo: model.BinaryInfo{
				Name:        "mockproj",
				FullPath:    "/home/user/go/bin/mockproj",
				InstallPath: "/home/user/go/bin/mockproj",
				PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
				Module:      model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:   "go1.24.5",
				OS:          "darwin",
				Arch:        "arm64",
				Feature:     "v8.0",
				EnvVars:     []string{"CGO_ENABLED=1"},
				IsManaged:   false,
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
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/.gobin/bin/mockproj@v0.1.0", err: os.ErrInvalid},
				{path: "/home/user/go/bin/mockproj", err: os.ErrNotExist},
				{path: "/home/user/go/bin/mockproj-v0", target: "/home/user/.gobin/bin/mockproj@v0.1.0"},
			},
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetGoBinPath:       true,
			mockGetGoBinPath:       "/home/user/go/bin",
			callListBinaries:       true,
			mockListBinaries: []string{
				"/home/user/go/bin/mockproj",
				"/home/user/go/bin/mockproj-v0",
			},
			expectedInfo: model.BinaryInfo{
				Name:        "mockproj",
				FullPath:    "/home/user/.gobin/bin/mockproj@v0.1.0",
				InstallPath: "/home/user/.gobin/bin/mockproj@v0.1.0",
				PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
				Module:      model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
				GoVersion:   "go1.24.5",
				OS:          "darwin",
				Arch:        "arm64",
				Feature:     "v8.0",
				EnvVars:     []string{"CGO_ENABLED=1"},
				IsManaged:   true,
				IsPinned:    true,
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
			mockGetSymlinkTargetCalls: []mockGetSymlinkTargetCall{
				{path: "/home/user/go/bin/mockproj", target: "/home/user/.gobin/bin/mockproj@v0.1.2-0.20250729191454-dac745d99aac"},
			},
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			expectedInfo: model.BinaryInfo{
				Name:        "mockproj",
				FullPath:    "/home/user/go/bin/mockproj",
				InstallPath: "/home/user/.gobin/bin/mockproj@v0.1.2-0.20250729191454-dac745d99aac",
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
				IsManaged:      true,
				IsPinned:       false,
			},
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfoErr: toolchain.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         toolchain.ErrBinaryBuiltWithoutGoModules,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).Once()

			for _, call := range tc.mockGetSymlinkTargetCalls {
				fs.EXPECT().GetSymlinkTarget(call.path).
					Return(call.target, call.err).
					Once()
			}

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGetGoBinPath).
					Once()
			}

			if tc.callListBinaries {
				fs.EXPECT().ListBinaries(tc.mockGetGoBinPath).
					Return(tc.mockListBinaries, tc.mockListBinariesErr).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			info, err := binaryManager.GetBinaryInfo(tc.path)
			assert.Equal(t, tc.expectedInfo, info)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryRepository(t *testing.T) {
	cases := map[string]struct {
		binary                  model.Binary
		mockGetGoBinPath        string
		mockGetBuildInfo        *buildinfo.BuildInfo
		mockGetBuildInfoErr     error
		callGetSymlinkTarget    bool
		mockGetSymlinkTarget    string
		mockGetSymlinkTargetErr error
		callGetInternalBinPath  bool
		mockGetInternalBinPath  string
		callGetModuleOrigin     bool
		mockGetModuleOrigin     *model.ModuleOrigin
		mockGetModuleOriginErr  error
		expectedRepository      string
		expectedErr             error
	}{
		"success-module-origin": {
			binary:                 model.NewBinary("mockproj"),
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:   true,
			mockGetSymlinkTarget:   "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOrigin: &model.ModuleOrigin{
				URL: "https://github.com/mockorg/mockproj",
			},
			expectedRepository: "https://github.com/mockorg/mockproj",
		},
		"success-module-origin-not-available": {
			binary:                 model.NewBinary("mockproj"),
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:   true,
			mockGetSymlinkTarget:   "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: toolchain.ErrModuleOriginNotAvailable,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"success-module-not-found": {
			binary:                 model.NewBinary("mockproj"),
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:   true,
			mockGetSymlinkTarget:   "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: toolchain.ErrModuleNotFound,
			expectedRepository:     "https://example.com/mockorg/mockproj",
		},
		"error-get-build-info": {
			binary:              model.NewBinary("mockproj"),
			mockGetGoBinPath:    "/home/user/go/bin",
			mockGetBuildInfoErr: toolchain.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         toolchain.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-module-origin": {
			binary:                 model.NewBinary("mockproj"),
			mockGetGoBinPath:       "/home/user/go/bin",
			mockGetBuildInfo:       getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:   true,
			mockGetSymlinkTarget:   "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetInternalBinPath: true,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			callGetModuleOrigin:    true,
			mockGetModuleOriginErr: errors.New("unexpected error"),
			expectedErr:            errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			toolchain.EXPECT().
				GetBuildInfo(filepath.Join(tc.mockGetGoBinPath, tc.binary.Name)).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callGetSymlinkTarget {
				fs.EXPECT().GetSymlinkTarget(filepath.Join(tc.mockGetGoBinPath, tc.binary.Name)).
					Return(tc.mockGetSymlinkTarget, tc.mockGetSymlinkTargetErr).
					Once()
			}

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Once()
			}

			if tc.callGetModuleOrigin {
				toolchain.EXPECT().GetModuleOrigin(
					context.Background(),
					model.NewModule(
						tc.mockGetBuildInfo.Main.Path,
						model.NewVersion(tc.mockGetBuildInfo.Main.Version),
					),
				).Return(tc.mockGetModuleOrigin, tc.mockGetModuleOriginErr).Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			repository, err := binaryManager.GetBinaryRepository(context.Background(), tc.binary)
			assert.Equal(t, tc.expectedRepository, repository)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_GetBinaryUpgradeInfo(t *testing.T) {
	cases := map[string]struct {
		info                            model.BinaryInfo
		checkMajor                      bool
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		expectedInfo                    model.BinaryUpgradeInfo
		expectedErr                     error
	}{
		"success-check-minor-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				IsUpgradeAvailable: false,
			},
		},
		"success-check-major-no-upgrade-available-v2": {
			info:       getBinaryInfo("mockproj", "v2.0.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					latestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v3"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v2.0.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				IsUpgradeAvailable: false,
			},
		},
		"success-check-minor-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: false,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-upgrade-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				},
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					latestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v3"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-multiple-major-upgrades-available": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				},
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					latestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				},
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj/v3"),
					latestModule: model.NewModule("example.com/mockorg/mockproj/v3", model.NewVersion("v3.0.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v4"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj", "v0.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj/v3", model.NewVersion("v3.0.0")),
				IsUpgradeAvailable: true,
			},
		},
		"success-check-major-pinned-version-upgrade-available": {
			info:       getBinaryInfo("mockproj-v1", "v1.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1")),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.2.0")),
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj-v1", "v1.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.2.0")),
				IsUpgradeAvailable: true,
			},
		},
		"success-check-minor-pinned-version-upgrade-available": {
			info:       getBinaryInfo("mockproj-v1.1", "v1.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1")),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.2.0")),
				},
			},
			expectedInfo: model.BinaryUpgradeInfo{
				BinaryInfo:         getBinaryInfo("mockproj-v1.1", "v1.1.0", false, true, false),
				LatestModule:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.2.0")),
				IsUpgradeAvailable: true,
			},
		},
		"error-get-latest-module-minor-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj"),
					err:    toolchain.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: toolchain.ErrModuleInfoNotAvailable,
		},
		"error-get-latest-module-major-version": {
			info:       getBinaryInfo("mockproj", "v0.1.0", false, true, false),
			checkMajor: true,
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					err:    toolchain.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: toolchain.ErrModuleInfoNotAvailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toolchain := toolchainmocks.NewToolchain(t)

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(context.Background(), call.module).
					Return(call.latestModule, call.err).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(nil, nil, toolchain, nil)
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
		pkg                      model.Package
		kind                     model.Kind
		rebuild                  bool
		mockInternalTempPath     string
		callCreateTempDir        bool
		mockCreateTempDirPattern string
		mockCreateTempDirPath    string
		mockCreateTempDirErr     error
		callInstall              bool
		mockInstallPackage       model.Package
		mockInstallErr           error
		callGetBuildInfo         bool
		mockGetBuildInfoPath     string
		mockGetBuildInfo         *buildinfo.BuildInfo
		mockGetBuildInfoErr      error
		callGetInternalBinPath   bool
		mockInternalBinPath      string
		callMove                 bool
		mockMoveSrc              string
		mockMoveDst              string
		mockMoveErr              error
		callGetGoBinPath         bool
		mockGoBinPath            string
		callReplaceSymlink       bool
		mockReplaceSymlinkSrc    string
		mockReplaceSymlinkDst    string
		mockReplaceSymlinkErr    error
		expectedErr              error
	}{
		"success-package": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@latest"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-package-with-version": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-package-version-suffix-with-version": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/v2@v2.0.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/v2@v2.0.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v2.0.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v2.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v2.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-package-kind-major": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			kind:                     model.KindMajor,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj-v1",
		},
		"success-package-kind-minor": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			kind:                     model.KindMinor,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.0.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj-v1.0",
		},
		"error-mkdir-temp": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirErr:     os.ErrNotExist,
			expectedErr:              os.ErrNotExist,
		},
		"error-install": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			mockInstallErr:           errors.New("exit status 1: unexpected error"),
			expectedErr:              errors.New("exit status 1: unexpected error"),
		},
		"error-get-build-info": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.1.0"),
			mockGetBuildInfoErr:      os.ErrNotExist,
			expectedErr:              os.ErrNotExist,
		},
		"error-move": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.1.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockMoveErr:              os.ErrExist,
			expectedErr:              os.ErrExist,
		},
		"error-replace-symlink": {
			pkg:                      model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			kind:                     model.KindLatest,
			rebuild:                  false,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo:         true,
			mockGetBuildInfoPath:     "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo:         getBuildInfo("mockproj", "v1.1.0"),
			callGetInternalBinPath:   true,
			mockInternalBinPath:      "/home/user/.gobin/bin",
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
			mockReplaceSymlinkErr:    errors.New("unexpected error"),
			expectedErr:              errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			workspace.EXPECT().GetInternalTempPath().
				Return(tc.mockInternalTempPath).
				Once()

			if tc.callCreateTempDir {
				fs.EXPECT().CreateTempDir(tc.mockInternalTempPath, tc.mockCreateTempDirPattern).
					Return(tc.mockCreateTempDirPath, func() error { return nil }, tc.mockCreateTempDirErr).Once()
			}

			if tc.callInstall {
				toolchain.EXPECT().Install(
					context.Background(),
					tc.mockCreateTempDirPath,
					tc.pkg,
					tc.rebuild,
				).Return(tc.mockInstallErr).Once()
			}

			if tc.callGetBuildInfo {
				toolchain.EXPECT().GetBuildInfo(tc.mockGetBuildInfoPath).
					Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
					Once()
			}

			if tc.callGetInternalBinPath {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockInternalBinPath).
					Once()
			}

			if tc.callMove {
				fs.EXPECT().Move(tc.mockMoveSrc, tc.mockMoveDst).
					Return(tc.mockMoveErr).Once()
			}

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGoBinPath).
					Once()
			}

			if tc.callReplaceSymlink {
				fs.EXPECT().ReplaceSymlink(tc.mockReplaceSymlinkSrc, tc.mockReplaceSymlinkDst).
					Return(tc.mockReplaceSymlinkErr).Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			err := binaryManager.InstallPackage(context.Background(), tc.pkg, tc.kind, tc.rebuild)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_MigrateBinary(t *testing.T) {
	cases := map[string]struct {
		path                        string
		mockGetBuildInfo            *buildinfo.BuildInfo
		mockGetBuildInfoErr         error
		callGetSymlinkTarget        bool
		mockGetSymlinkTarget        string
		mockGetSymlinkTargetErr     error
		mockGetInternalBinPathTimes int
		mockGetInternalBinPath      string
		callMoveWithSymlink         bool
		mockMoveWithSymlinkSrc      string
		mockMoveWithSymlinkDst      string
		mockMoveWithSymlinkErr      error
		expectedErr                 error
	}{
		"success": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/go/bin/mockproj",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			callMoveWithSymlink:         true,
			mockMoveWithSymlinkSrc:      "/home/user/go/bin/mockproj",
			mockMoveWithSymlinkDst:      "/home/user/.gobin/bin/mockproj@v0.1.0",
		},
		"error-get-build-info": {
			path:                "/home/user/go/bin/mockproj",
			mockGetBuildInfo:    getBuildInfo("mockproj", "v0.1.0"),
			mockGetBuildInfoErr: toolchain.ErrBinaryNotFound,
			expectedErr:         toolchain.ErrBinaryNotFound,
		},
		"error-already-managed": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			expectedErr:                 manager.ErrBinaryAlreadyManaged,
		},
		"error-move-with-symlink": {
			path:                        "/home/user/go/bin/mockproj",
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/go/bin/mockproj",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			callMoveWithSymlink:         true,
			mockMoveWithSymlinkSrc:      "/home/user/go/bin/mockproj",
			mockMoveWithSymlinkDst:      "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockMoveWithSymlinkErr:      errors.New("unexpected error"),
			expectedErr:                 errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.path).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callGetSymlinkTarget {
				fs.EXPECT().GetSymlinkTarget(tc.path).
					Return(tc.mockGetSymlinkTarget, tc.mockGetSymlinkTargetErr).
					Once()
			}

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			if tc.callMoveWithSymlink {
				fs.EXPECT().MoveWithSymlink(tc.mockMoveWithSymlinkSrc, tc.mockMoveWithSymlinkDst).
					Return(tc.mockMoveWithSymlinkErr).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			err := binaryManager.MigrateBinary(tc.path)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_PinBinary(t *testing.T) {
	cases := map[string]struct {
		bin                    model.Binary
		kind                   model.Kind
		mockGetInternalBinPath string
		mockListBinaries       []string
		mockListBinariesErr    error
		callGetGoBinPath       bool
		mockGetGoBinPath       string
		mockBinPath            string
		callReplaceSymlink     bool
		mockReplaceSymlinkSrc  string
		mockReplaceSymlinkDst  string
		mockReplaceSymlinkErr  error
		expectedErr            error
	}{
		"success-version-latest-kind-latest": {
			bin:                    model.NewBinary("mockproj2"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v2.2.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2",
		},
		"success-version-latest-kind-major": {
			bin:                    model.NewBinary("mockproj2"),
			kind:                   model.KindMajor,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2-v2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v2.2.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2-v2",
		},
		"success-version-latest-kind-minor": {
			bin:                    model.NewBinary("mockproj2"),
			kind:                   model.KindMinor,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2-v2.2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v2.2.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2-v2.2",
		},
		"success-version-v1-kind-latest": {
			bin:                    model.NewBinary("mockproj2@v1"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v1.3.1",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2",
		},
		"success-version-v1.2-kind-latest": {
			bin:                    model.NewBinary("mockproj2@v1.2"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v1.2.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2",
		},
		"success-version-v1.3.0-kind-latest": {
			bin:                    model.NewBinary("mockproj2@v1.3.0"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
				"/home/user/.gobin/bin/mockproj2@v0.4.0",
				"/home/user/.gobin/bin/mockproj2@v1.2.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.0",
				"/home/user/.gobin/bin/mockproj2@v1.3.1",
				"/home/user/.gobin/bin/mockproj3@v2.1.0",
				"/home/user/.gobin/bin/mockproj2@v2.2.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj2",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj2@v1.3.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj2",
		},
		"error-list-binaries": {
			bin:                    model.NewBinary("mockproj1"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinariesErr:    os.ErrNotExist,
			expectedErr:            os.ErrNotExist,
		},
		"error-binary-not-found": {
			bin:                    model.NewBinary("mockproj1@v0.4"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
			},
			expectedErr: toolchain.ErrBinaryNotFound,
		},
		"error-replace-symlink": {
			bin:                    model.NewBinary("mockproj1"),
			kind:                   model.KindLatest,
			mockGetInternalBinPath: "/home/user/.gobin/bin",
			mockListBinaries: []string{
				"/home/user/.gobin/bin/mockproj1@v0.3.0",
			},
			callGetGoBinPath:      true,
			mockGetGoBinPath:      "/home/user/go/bin",
			mockBinPath:           "/home/user/go/bin/mockproj1",
			callReplaceSymlink:    true,
			mockReplaceSymlinkSrc: "/home/user/.gobin/bin/mockproj1@v0.3.0",
			mockReplaceSymlinkDst: "/home/user/go/bin/mockproj1",
			mockReplaceSymlinkErr: errors.New("unexpected error"),
			expectedErr:           errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			workspace.EXPECT().GetInternalBinPath().
				Return(tc.mockGetInternalBinPath).
				Once()

			fs.EXPECT().ListBinaries(tc.mockGetInternalBinPath).
				Return(tc.mockListBinaries, tc.mockListBinariesErr).
				Once()

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGetGoBinPath).
					Once()
			}

			if tc.callReplaceSymlink {
				fs.EXPECT().ReplaceSymlink(tc.mockReplaceSymlinkSrc, tc.mockReplaceSymlinkDst).
					Return(tc.mockReplaceSymlinkErr).
					Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
			err := binaryManager.PinBinary(tc.bin, tc.kind)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGoBinaryManager_UninstallBinary(t *testing.T) {
	cases := map[string]struct {
		bin              model.Binary
		mockGetGoBinPath string
		mockRemoveErr    error
		expectedErr      error
	}{
		"success-unmanaged-binary": {
			bin:              model.NewBinary("mockproj"),
			mockGetGoBinPath: "/home/user/.gobin/bin",
		},
		"success-managed-binary": {
			bin:              model.NewBinary("mockproj"),
			mockGetGoBinPath: "/home/user/.gobin/bin",
		},
		"error-binary-not-found": {
			bin:              model.NewBinary("mockproj"),
			mockGetGoBinPath: "/home/user/.gobin/bin",
			mockRemoveErr:    os.ErrNotExist,
			expectedErr:      os.ErrNotExist,
		},
		"error-remove-binary": {
			bin:              model.NewBinary("mockproj"),
			mockGetGoBinPath: "/home/user/.gobin/bin",
			mockRemoveErr:    errors.New("unexpected error"),
			expectedErr:      errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)

			workspace.EXPECT().GetGoBinPath().
				Return(tc.mockGetGoBinPath).
				Once()

			fs.EXPECT().Remove(filepath.Join(tc.mockGetGoBinPath, tc.bin.Name)).
				Return(tc.mockRemoveErr).
				Once()

			binaryManager := manager.NewGoBinaryManager(fs, nil, nil, workspace)
			err := binaryManager.UninstallBinary(tc.bin)
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
		callGetSymlinkTarget            bool
		mockGetSymlinkTarget            string
		mockGetSymlinkTargetErr         error
		mockGetInternalBinPathTimes     int
		mockGetInternalBinPath          string
		mockGetLatestModuleVersionCalls []mockGetLatestModuleVersionCall
		callGetInternalTempPath         bool
		mockInternalTempPath            string
		callCreateTempDir               bool
		mockCreateTempDirPattern        string
		mockCreateTempDirPath           string
		mockCreateTempDirErr            error
		callInstall                     bool
		mockInstallPackage              model.Package
		mockInstallErr                  error
		callGetBuildInfo2               bool
		mockGetBuildInfo2Path           string
		mockGetBuildInfo2               *buildinfo.BuildInfo
		mockGetBuildInfo2Err            error
		callMove                        bool
		mockMoveSrc                     string
		mockMoveDst                     string
		mockMoveErr                     error
		callGetGoBinPath                bool
		mockGoBinPath                   string
		callReplaceSymlink              bool
		mockReplaceSymlinkSrc           string
		mockReplaceSymlinkDst           string
		mockReplaceSymlinkErr           error
		expectedErr                     error
	}{
		"success-no-minor-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				},
			},
		},
		"success-no-major-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
		},
		"success-no-upgrade-available-rebuild": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     true,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v0.1.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v0.1.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v0.1.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-minor-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v1.1.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-major-upgrade-available": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
				},
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj/v2"),
					latestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
				},
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj/v3"),
					err:    toolchain.ErrModuleNotFound,
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/v2/cmd/mockproj@v2.0.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v2.0.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v2.0.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v2.0.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
		},
		"success-upgrade-available-kind-major": {
			binFullPath:                 "/home/user/go/bin/mockproj-v0",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0")),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.2.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v0.2.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v0.2.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v0.2.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v0.2.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj-v0",
		},
		"success-upgrade-available-kind-minor": {
			binFullPath:                 "/home/user/go/bin/mockproj-v0.1",
			majorUpgrade:                true,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1")),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v0.1.1")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v0.1.1"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v0.1.1"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v0.1.1",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v0.1.1",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj-v0.1",
		},
		"error-get-build-info": {
			binFullPath:         "/home/user/go/bin/mockproj",
			majorUpgrade:        false,
			rebuild:             false,
			mockGetBuildInfoErr: toolchain.ErrBinaryBuiltWithoutGoModules,
			expectedErr:         toolchain.ErrBinaryBuiltWithoutGoModules,
		},
		"error-get-latest-module-version": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v0.1.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module: model.NewLatestModule("example.com/mockorg/mockproj"),
					err:    toolchain.ErrModuleInfoNotAvailable,
				},
			},
			expectedErr: toolchain.ErrModuleInfoNotAvailable,
		},
		"error-mkdir-temp": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirErr:     os.ErrNotExist,
			expectedErr:              os.ErrNotExist,
		},
		"error-install": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			mockInstallErr:           errors.New("exit status 1: unexpected error"),
			expectedErr:              errors.New("exit status 1: unexpected error"),
		},
		"error-get-build-info-2": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 1,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v1.1.0"),
			mockGetBuildInfo2Err:     os.ErrNotExist,
			expectedErr:              os.ErrNotExist,
		},
		"error-move": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v1.1.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockMoveErr:              os.ErrExist,
			expectedErr:              os.ErrExist,
		},
		"error-replace-symlink": {
			binFullPath:                 "/home/user/go/bin/mockproj",
			majorUpgrade:                false,
			rebuild:                     false,
			mockGetBuildInfo:            getBuildInfo("mockproj", "v1.0.0"),
			callGetSymlinkTarget:        true,
			mockGetSymlinkTarget:        "/home/user/.gobin/bin/mockproj@v0.1.0",
			mockGetInternalBinPathTimes: 2,
			mockGetInternalBinPath:      "/home/user/.gobin/bin",
			mockGetLatestModuleVersionCalls: []mockGetLatestModuleVersionCall{
				{
					module:       model.NewLatestModule("example.com/mockorg/mockproj"),
					latestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.1.0")),
				},
			},
			callGetInternalTempPath:  true,
			mockInternalTempPath:     "/home/user/.gobin/.tmp",
			callCreateTempDir:        true,
			mockCreateTempDirPattern: "mockproj-*",
			mockCreateTempDirPath:    "/home/user/.gobin/.tmp/mockproj-0123456789",
			callInstall:              true,
			mockInstallPackage:       model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.1.0"),
			callGetBuildInfo2:        true,
			mockGetBuildInfo2Path:    "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockGetBuildInfo2:        getBuildInfo("mockproj", "v1.1.0"),
			callMove:                 true,
			mockMoveSrc:              "/home/user/.gobin/.tmp/mockproj-0123456789/mockproj",
			mockMoveDst:              "/home/user/.gobin/bin/mockproj@v1.1.0",
			callGetGoBinPath:         true,
			mockGoBinPath:            "/home/user/go/bin",
			callReplaceSymlink:       true,
			mockReplaceSymlinkSrc:    "/home/user/.gobin/bin/mockproj@v1.1.0",
			mockReplaceSymlinkDst:    "/home/user/go/bin/mockproj",
			mockReplaceSymlinkErr:    errors.New("unexpected error"),
			expectedErr:              errors.New("unexpected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := systemmocks.NewFileSystem(t)
			workspace := systemmocks.NewWorkspace(t)
			toolchain := toolchainmocks.NewToolchain(t)

			toolchain.EXPECT().GetBuildInfo(tc.binFullPath).
				Return(tc.mockGetBuildInfo, tc.mockGetBuildInfoErr).
				Once()

			if tc.callGetSymlinkTarget {
				fs.EXPECT().GetSymlinkTarget(tc.binFullPath).
					Return(tc.mockGetSymlinkTarget, tc.mockGetSymlinkTargetErr).
					Once()
			}

			if tc.mockGetInternalBinPathTimes > 0 {
				workspace.EXPECT().GetInternalBinPath().
					Return(tc.mockGetInternalBinPath).
					Times(tc.mockGetInternalBinPathTimes)
			}

			for _, call := range tc.mockGetLatestModuleVersionCalls {
				toolchain.EXPECT().GetLatestModuleVersion(context.Background(), call.module).
					Return(call.latestModule, call.err).
					Once()
			}

			if tc.callGetInternalTempPath {
				workspace.EXPECT().GetInternalTempPath().
					Return(tc.mockInternalTempPath).
					Once()
			}

			if tc.callCreateTempDir {
				fs.EXPECT().CreateTempDir(tc.mockInternalTempPath, tc.mockCreateTempDirPattern).
					Return(tc.mockCreateTempDirPath, func() error { return nil }, tc.mockCreateTempDirErr).Once()
			}

			if tc.callInstall {
				toolchain.EXPECT().Install(
					context.Background(),
					tc.mockCreateTempDirPath,
					tc.mockInstallPackage,
					tc.rebuild,
				).Return(tc.mockInstallErr).Once()
			}

			if tc.callGetBuildInfo2 {
				toolchain.EXPECT().GetBuildInfo(tc.mockGetBuildInfo2Path).
					Return(tc.mockGetBuildInfo2, tc.mockGetBuildInfo2Err).
					Once()
			}

			if tc.callMove {
				fs.EXPECT().Move(tc.mockMoveSrc, tc.mockMoveDst).
					Return(tc.mockMoveErr).Once()
			}

			if tc.callGetGoBinPath {
				workspace.EXPECT().GetGoBinPath().
					Return(tc.mockGoBinPath).
					Once()
			}

			if tc.callReplaceSymlink {
				fs.EXPECT().ReplaceSymlink(tc.mockReplaceSymlinkSrc, tc.mockReplaceSymlinkDst).
					Return(tc.mockReplaceSymlinkErr).Once()
			}

			binaryManager := manager.NewGoBinaryManager(fs, nil, toolchain, workspace)
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
	internalBinPath, managed, pinned bool,
) model.BinaryInfo {
	packagePath := "example.com/mockorg/mockproj/cmd/" + name
	modulePath := "example.com/mockorg/mockproj"
	if major := semver.Major(version); major != "v0" && major != "v1" {
		packagePath = modulePath + "/" + major + "/cmd/" + name
		modulePath = modulePath + "/" + major
	}

	path := "/home/user/go/bin/" + name
	if internalBinPath {
		path = "/home/user/.gobin/bin/" + name + "@" + version
	}

	installPath := path
	if managed {
		installPath = "/home/user/.gobin/bin/" + name + "@" + version
	}

	return model.BinaryInfo{
		Name:        name,
		FullPath:    path,
		InstallPath: installPath,
		PackagePath: packagePath,
		Module:      model.NewModule(modulePath, model.NewVersion(version)),
		ModuleSum:   "h1:Zn6y0QZqqixH1kGqbYWR/Ce4eG9FD4xZ8buAi7rStQc=",
		GoVersion:   "go1.24.5",
		OS:          "darwin",
		Arch:        "arm64",
		Feature:     "v8.0",
		EnvVars:     []string{"CGO_ENABLED=1"},
		IsManaged:   managed,
		IsPinned:    pinned,
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
