package internal

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const (
	GOARCHEnvVar = "GOARCH"
	GOOSEnvVar   = "GOOS"
)

var (
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")
	ErrBinaryNotFound              = errors.New("binary not found")
	ErrInvalidModuleVersion        = errors.New("invalid module version")
)

type Toolchain interface {
	GetLatestModuleVersion(module string) (string, string, error)
	GetModuleFile(module, version string) (*modfile.File, error)
	GetModuleOrigin(module, version string) (*ModuleOrigin, error)
	Install(pkg, version string) error
	VulnCheck(path string) ([]Vulnerability, error)
}

type BinaryInfo struct {
	Name           string
	FullPath       string
	PackagePath    string
	ModulePath     string
	ModuleVersion  string
	ModuleSum      string
	GoVersion      string
	CommitRevision string
	CommitTime     string
	OS             string
	Arch           string
	Feature        string
	EnvVars        []string
}

type BinaryUpgradeInfo struct {
	BinaryInfo

	LatestModulePath   string
	LatestVersion      string
	IsUpgradeAvailable bool
}

type BinaryDiagnostic struct {
	Name             string
	NotInPath        bool
	DuplicatesInPath []string
	GoVersion        struct {
		Actual   string
		Expected string
	}
	Platform struct {
		Actual   string
		Expected string
	}
	IsPseudoVersion       bool
	NotBuiltWithGoModules bool
	IsOrphaned            bool
	Retracted             string
	Deprecated            string
	Vulnerabilities       []Vulnerability
}

func (d BinaryDiagnostic) HasIssues() bool {
	return d.NotInPath ||
		len(d.DuplicatesInPath) > 0 ||
		d.GoVersion.Actual != d.GoVersion.Expected ||
		d.Platform.Actual != d.Platform.Expected ||
		d.IsPseudoVersion ||
		d.NotBuiltWithGoModules ||
		d.IsOrphaned ||
		d.Retracted != "" ||
		d.Deprecated != "" ||
		len(d.Vulnerabilities) > 0
}

type GoBinaryManager struct {
	toolchain Toolchain
}

func NewGoBinaryManager(toolchain Toolchain) *GoBinaryManager {
	return &GoBinaryManager{toolchain: toolchain}
}

func (m *GoBinaryManager) DiagnoseBinary(path string) (BinaryDiagnostic, error) {
	binaryName := filepath.Base(path)
	logger := slog.Default().With("binary", binaryName, "path", path)

	buildInfo, err := buildinfo.ReadFile(path)
	if err != nil {
		logger.Error("error reading binary build info", "err", err)
		return BinaryDiagnostic{}, err
	}

	binPlatform := getBinaryPlatform(buildInfo)
	runtimePlatform := runtime.GOOS + "/" + runtime.GOARCH

	diagnostic := BinaryDiagnostic{
		Name:                  binaryName,
		DuplicatesInPath:      checkBinaryDuplicatesInPath(binaryName),
		IsPseudoVersion:       module.IsPseudoVersion(buildInfo.Main.Version),
		NotBuiltWithGoModules: buildInfo.Main.Path == "",
		IsOrphaned:            buildInfo.Main.Sum == "",
	}

	diagnostic.GoVersion.Actual = buildInfo.GoVersion
	diagnostic.GoVersion.Expected = runtime.Version()
	diagnostic.Platform.Actual = binPlatform
	diagnostic.Platform.Expected = runtimePlatform

	_, err = exec.LookPath(binaryName)
	diagnostic.NotInPath = err != nil

	if buildInfo.Main.Sum != "" {
		retracted, deprecated, modErr := m.diagnoseGoModFile(
			buildInfo.Main.Path, buildInfo.Main.Version,
		)
		if modErr != nil {
			return diagnostic, modErr
		}

		diagnostic.Retracted = retracted
		diagnostic.Deprecated = deprecated
	}

	diagnostic.Vulnerabilities, err = m.toolchain.VulnCheck(path)
	if err != nil {
		return diagnostic, err
	}

	return diagnostic, nil
}

func (m *GoBinaryManager) GetAllBinaryInfos() ([]BinaryInfo, error) {
	binFullPath, err := m.GetBinFullPath()
	if err != nil {
		return nil, err
	}

	bins, err := m.ListBinariesFullPaths(binFullPath)
	if err != nil {
		return nil, err
	}

	binInfos := make([]BinaryInfo, 0, len(bins))
	for _, bin := range bins {
		info, infoErr := m.GetBinaryInfo(bin)
		if infoErr == nil {
			binInfos = append(binInfos, info)
		}
	}

	return binInfos, nil
}

func (m *GoBinaryManager) GetBinaryInfo(path string) (BinaryInfo, error) {
	logger := slog.Default().With("path", path)

	info, err := buildinfo.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return BinaryInfo{}, ErrBinaryNotFound
		}

		logger.Error("error reading binary build info", "err", err)
		return BinaryInfo{}, err
	}

	if info.Main.Path == "" {
		logger.Error(ErrBinaryBuiltWithoutGoModules.Error())
		return BinaryInfo{}, ErrBinaryBuiltWithoutGoModules
	}

	binInfo := BinaryInfo{
		Name:          filepath.Base(path),
		FullPath:      path,
		PackagePath:   info.Path,
		ModulePath:    info.Main.Path,
		ModuleVersion: info.Main.Version,
		ModuleSum:     info.Main.Sum,
		GoVersion:     info.GoVersion,
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			binInfo.CommitRevision = s.Value
		case "vcs.time":
			binInfo.CommitTime = s.Value
		case GOOSEnvVar:
			binInfo.OS = s.Value
		case GOARCHEnvVar:
			binInfo.Arch = s.Value
		default:
			if strings.HasPrefix(s.Key, "GO") {
				binInfo.Feature = s.Value
			}
			if strings.HasPrefix(s.Key, "CGO_") && s.Value != "" {
				binInfo.EnvVars = append(binInfo.EnvVars, s.Key+"="+s.Value)
			}
		}
	}

	return binInfo, nil
}

func (m *GoBinaryManager) GetBinaryRepository(binary string) (string, error) {
	binPath, err := m.GetBinFullPath()
	if err != nil {
		return "", err
	}

	binInfo, err := m.GetBinaryInfo(filepath.Join(binPath, binary))
	if err != nil {
		return "", err
	}

	modOrigin, err := m.toolchain.GetModuleOrigin(
		binInfo.ModulePath, binInfo.ModuleVersion,
	)
	if err != nil && !errors.Is(err, ErrModuleNotFound) {
		return "", err
	}

	repoURL := "https://" + binInfo.ModulePath
	if modOrigin != nil {
		repoURL = modOrigin.URL
	}

	return repoURL, nil
}

func (m *GoBinaryManager) GetBinaryUpgradeInfo(
	info BinaryInfo,
	checkMajor bool,
) (BinaryUpgradeInfo, error) {
	binUpInfo := BinaryUpgradeInfo{
		BinaryInfo:         info,
		LatestVersion:      info.ModuleVersion,
		IsUpgradeAvailable: false,
	}

	modulePath, version, err := m.toolchain.GetLatestModuleVersion(binUpInfo.ModulePath)
	if err != nil {
		return BinaryUpgradeInfo{}, err
	}

	binUpInfo.LatestModulePath = modulePath
	binUpInfo.LatestVersion = version
	binUpInfo.IsUpgradeAvailable = semver.Compare(binUpInfo.ModuleVersion, version) < 0

	if checkMajor {
		modulePath, version, err = m.checkModuleMajorUpgrade(
			binUpInfo.ModulePath, binUpInfo.LatestVersion,
		)
		if err != nil {
			return BinaryUpgradeInfo{}, err
		}

		binUpInfo.LatestModulePath = modulePath
		binUpInfo.LatestVersion = version
		binUpInfo.IsUpgradeAvailable = semver.Compare(binUpInfo.ModuleVersion, version) < 0
	}

	return binUpInfo, nil
}

func (m *GoBinaryManager) GetBinFullPath() (string, error) {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return gobin, nil
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(gopath, "bin"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Default().Error("error getting user home directory", "err", err)
		return "", err
	}

	return filepath.Join(home, "go", "bin"), nil
}

func (m *GoBinaryManager) ListBinariesFullPaths(dir string) ([]string, error) {
	logger := slog.Default().With("dir", dir)
	var binaries []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error("error while reading binaries directory", "err", err)
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if isBinary(fullPath) {
			binaries = append(binaries, fullPath)
		}
	}

	return binaries, nil
}

func (m *GoBinaryManager) UpgradeBinary(binFullPath string, majorUpgrade bool, rebuild bool) error {
	info, err := m.GetBinaryInfo(binFullPath)
	if err != nil {
		return err
	}

	binUpInfo, err := m.GetBinaryUpgradeInfo(info, majorUpgrade)
	if err != nil {
		return err
	}

	if binUpInfo.IsUpgradeAvailable || rebuild {
		return m.installBinary(binUpInfo)
	}

	return nil
}

func (m *GoBinaryManager) checkModuleMajorUpgrade(
	module, version string,
) (string, string, error) {
	latestModulePath := module
	latestMajorVersion := version

	pkg := module
	if major := semver.Major(version); major != "v0" && major != "v1" {
		pkg = stripVersionSuffix(module)
	}

	for {
		nextVersion, err := nextMajorVersion(latestMajorVersion)
		if err != nil {
			return "", "", err
		}

		pkgMajor := pkg + "/" + nextVersion
		modulePath, majorVersion, err := m.toolchain.GetLatestModuleVersion(pkgMajor)
		if err != nil {
			if errors.Is(err, ErrModuleNotFound) {
				break
			}
			return "", "", err
		}

		latestModulePath = modulePath
		latestMajorVersion = majorVersion
	}

	return latestModulePath, latestMajorVersion, nil
}

func (m *GoBinaryManager) diagnoseGoModFile(
	module, version string,
) (string, string, error) {
	logger := slog.Default().With("module", module, "version", version)

	modFile, err := m.toolchain.GetModuleFile(module, "latest")
	if err != nil {
		logger.Error("error downloading go.mod", "err", err)
		return "", "", err
	}

	var retracted string
	for _, r := range modFile.Retract {
		if semver.Compare(r.Low, version) <= 0 &&
			semver.Compare(r.High, version) >= 0 {
			retracted = r.Rationale
		}
	}

	var deprecated string
	if modFile.Module != nil && modFile.Module.Deprecated != "" {
		deprecated = modFile.Module.Deprecated
	}

	return retracted, deprecated, err
}

func (m *GoBinaryManager) installBinary(info BinaryUpgradeInfo) error {
	baseModule := stripVersionSuffix(info.LatestModulePath)
	packageSuffix := strings.TrimPrefix(info.PackagePath, info.ModulePath)
	major := semver.Major(info.LatestVersion)

	var pkg string
	if major == "v0" || major == "v1" {
		pkg = baseModule + packageSuffix
	} else {
		pkg = baseModule + "/" + major + packageSuffix
	}

	return m.toolchain.Install(pkg, info.LatestVersion)
}

func checkBinaryDuplicatesInPath(name string) []string {
	var (
		seen       = make(map[string]struct{})
		duplicates []string
	)

	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		fullPath := filepath.Join(dir, name)
		if isBinary(fullPath) {
			if _, ok := seen[fullPath]; !ok {
				seen[fullPath] = struct{}{}
				duplicates = append(duplicates, fullPath)
			}
		}
	}

	if len(duplicates) > 1 {
		return duplicates
	}

	return nil
}

func getBinaryPlatform(info *buildinfo.BuildInfo) string {
	var goOS, goArch string
	for _, s := range info.Settings {
		switch s.Key {
		case GOOSEnvVar:
			goOS = s.Value
		case GOARCHEnvVar:
			goArch = s.Value
		}
	}

	return goOS + "/" + goArch
}

func isBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}

	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func nextMajorVersion(version string) (string, error) {
	logger := slog.Default().With("version", version)

	if !semver.IsValid(version) {
		err := ErrInvalidModuleVersion
		logger.Error(err.Error())
		return "", err
	}

	major := semver.Major(version)
	if major == "v0" || major == "v1" {
		return "v2", nil
	}

	majorNumStr := strings.TrimPrefix(major, "v")
	majorNum, err := strconv.Atoi(majorNumStr)
	if err != nil {
		logger.Error("error parsing major version number", "err", err)
		return "", err
	}

	return fmt.Sprintf("v%d", majorNum+1), nil
}

func stripVersionSuffix(module string) string {
	parts := strings.Split(module, "/")
	lastPart := parts[len(parts)-1]

	if strings.HasPrefix(lastPart, "v") {
		if _, err := strconv.Atoi(lastPart[1:]); err == nil {
			return strings.Join(parts[:len(parts)-1], "/")
		}
	}

	return module
}
