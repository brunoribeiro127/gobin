package internal

import (
	"context"
	"debug/buildinfo"
	"errors"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const (
	// GOARCHEnvVar is the environment variable for the Go runtime architecture.
	GOARCHEnvVar = "GOARCH"
	// GOOSEnvVar is the environment variable for the Go runtime operating
	// system.
	GOOSEnvVar = "GOOS"
)

var (
	ErrInvalidPackageVersion = errors.New("invalid package version")
)

// BinaryInfo represents the information for a binary.
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

// BinaryUpgradeInfo represents the upgrade information for a binary.
type BinaryUpgradeInfo struct {
	BinaryInfo

	LatestModulePath   string
	LatestVersion      string
	IsUpgradeAvailable bool
}

// BinaryDiagnostic represents the diagnostic results for a binary.
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

// HasIssues returns whether the binary has any issues.
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

// BinaryManager is an interface for a binary manager.
type BinaryManager interface {
	// DiagnoseBinary diagnoses issues in a binary.
	DiagnoseBinary(ctx context.Context, path string) (BinaryDiagnostic, error)
	// GetAllBinaryInfos gets all binary infos.
	GetAllBinaryInfos() ([]BinaryInfo, error)
	// GetBinaryInfo gets the binary info for a given path.
	GetBinaryInfo(path string) (BinaryInfo, error)
	// GetBinaryRepository gets the repository URL for a given binary.
	GetBinaryRepository(ctx context.Context, binary string) (string, error)
	// GetBinaryUpgradeInfo gets the upgrade information for a given binary.
	GetBinaryUpgradeInfo(ctx context.Context, info BinaryInfo, checkMajor bool) (BinaryUpgradeInfo, error)
	// GetBinFullPath gets the full path to the Go binary directory.
	GetBinFullPath() (string, error)
	// InstallPackage installs a package.
	InstallPackage(ctx context.Context, pkgVersion string) error
	// ListBinariesFullPaths lists all binary full paths in the Go binary directory.
	ListBinariesFullPaths(dir string) ([]string, error)
	// UpgradeBinary upgrades a binary.
	UpgradeBinary(ctx context.Context, binFullPath string, majorUpgrade bool, rebuild bool) error
}

// GoBinaryManager is a manager for Go binaries.
type GoBinaryManager struct {
	system    System
	toolchain Toolchain
}

// NewGoBinaryManager creates a new GoBinaryManager.
func NewGoBinaryManager(
	system System,
	toolchain Toolchain,
) *GoBinaryManager {
	return &GoBinaryManager{
		system:    system,
		toolchain: toolchain,
	}
}

// DiagnoseBinary diagnoses a binary leveraging the toolchain. It returns the
// diagnostic results, or an error if the binary cannot be diagnosed (e.g. the
// binary is not a Go binary, the build info cannot be read, or the binary was
// built without Go modules). It also checks for vulnerabilities in the binary.
func (m *GoBinaryManager) DiagnoseBinary(
	ctx context.Context,
	path string,
) (BinaryDiagnostic, error) {
	binaryName := filepath.Base(path)
	diagnostic := BinaryDiagnostic{
		Name: binaryName,
	}

	buildInfo, err := m.toolchain.GetBuildInfo(path)
	if err != nil {
		if errors.Is(err, ErrBinaryBuiltWithoutGoModules) {
			diagnostic.NotBuiltWithGoModules = true
			return diagnostic, nil
		}

		return BinaryDiagnostic{}, err
	}

	binPlatform := getBinaryPlatform(buildInfo)
	runtimePlatform := m.system.RuntimeOS() + "/" + m.system.RuntimeARCH()

	diagnostic.DuplicatesInPath = m.checkBinaryDuplicatesInPath(binaryName)
	diagnostic.IsPseudoVersion = module.IsPseudoVersion(buildInfo.Main.Version)
	diagnostic.IsOrphaned = buildInfo.Main.Sum == ""
	diagnostic.GoVersion.Actual = buildInfo.GoVersion
	diagnostic.GoVersion.Expected = m.system.RuntimeVersion()
	diagnostic.Platform.Actual = binPlatform
	diagnostic.Platform.Expected = runtimePlatform

	_, err = m.system.LookPath(binaryName)
	diagnostic.NotInPath = err != nil

	if buildInfo.Main.Sum != "" {
		retracted, deprecated, modErr := m.diagnoseGoModFile(
			ctx, buildInfo.Main.Path, buildInfo.Main.Version,
		)
		if modErr != nil {
			return BinaryDiagnostic{}, modErr
		}

		diagnostic.Retracted = retracted
		diagnostic.Deprecated = deprecated
	}

	diagnostic.Vulnerabilities, err = m.toolchain.VulnCheck(ctx, path)
	if err != nil {
		return BinaryDiagnostic{}, err
	}

	return diagnostic, nil
}

// GetAllBinaryInfos gets all binary infos in the Go binary directory. It
// returns a list of binary infos, or an error if the binary directory cannot be
// determined or listed. It skips silently failures to get the binary info.
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

// GetBinaryInfo gets the binary info for a given path leveraging the toolchain.
// It constructs the binary info from the binary's build info. It fails if the
// binary does not exist, is not a Go binary, or the binary was built without
// Go modules.
func (m *GoBinaryManager) GetBinaryInfo(path string) (BinaryInfo, error) {
	info, err := m.toolchain.GetBuildInfo(path)
	if err != nil {
		return BinaryInfo{}, err
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

// GetBinaryRepository gets the repository URL for a binary leveraging the
// toolchain. It returns the repository URL from the module origin, falling back
// to the default repository URL if the module origin is not available.
func (m *GoBinaryManager) GetBinaryRepository(
	ctx context.Context,
	binary string,
) (string, error) {
	binPath, err := m.GetBinFullPath()
	if err != nil {
		return "", err
	}

	binInfo, err := m.GetBinaryInfo(filepath.Join(binPath, binary))
	if err != nil {
		return "", err
	}

	modOrigin, err := m.toolchain.GetModuleOrigin(ctx, binInfo.ModulePath, binInfo.ModuleVersion)
	if err != nil &&
		!errors.Is(err, ErrModuleNotFound) &&
		!errors.Is(err, ErrModuleOriginNotAvailable) {
		return "", err
	}

	repoURL := "https://" + binInfo.ModulePath
	if modOrigin != nil {
		repoURL = modOrigin.URL
	}

	return repoURL, nil
}

// GetBinaryUpgradeInfo gets the upgrade information for a binary leveraging the
// toolchain. It first checks if the binary has a minor version upgrade
// available. Then, if the checkMajor flag is set, it checks if the binary has
// a major version upgrade available. It returns the upgrade information, or an
// error if the upgrade information cannot be determined (e.g. the module is
// not found).
func (m *GoBinaryManager) GetBinaryUpgradeInfo(
	ctx context.Context,
	info BinaryInfo,
	checkMajor bool,
) (BinaryUpgradeInfo, error) {
	binUpInfo := BinaryUpgradeInfo{
		BinaryInfo:         info,
		LatestVersion:      info.ModuleVersion,
		IsUpgradeAvailable: false,
	}

	modulePath, version, err := m.toolchain.GetLatestModuleVersion(ctx, binUpInfo.ModulePath)
	if err != nil {
		return BinaryUpgradeInfo{}, err
	}

	binUpInfo.LatestModulePath = modulePath
	binUpInfo.LatestVersion = version
	binUpInfo.IsUpgradeAvailable = semver.Compare(binUpInfo.ModuleVersion, version) < 0

	if checkMajor {
		modulePath, version, err = m.checkModuleMajorUpgrade(
			ctx, binUpInfo.ModulePath, binUpInfo.LatestVersion,
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

// GetBinFullPath gets the full path to the Go binary directory. It determines
// the full path to the Go binary directory by the following order:
//  1. $GOBIN
//  2. $GOPATH/bin
//  3. $HOME/go/bin
//
// It returns an error if the user's home directory cannot be determined.
func (m *GoBinaryManager) GetBinFullPath() (string, error) {
	if gobin, ok := m.system.GetEnvVar("GOBIN"); ok {
		return gobin, nil
	}

	if gopath, ok := m.system.GetEnvVar("GOPATH"); ok {
		return filepath.Join(gopath, "bin"), nil
	}

	home, err := m.system.UserHomeDir()
	if err != nil {
		slog.Default().Error("error getting user home directory", "err", err)
		return "", err
	}

	return filepath.Join(home, "go", "bin"), nil
}

// InstallPackage installs a package leveraging the toolchain. If version is
// not provided, it installs the latest version.
func (m *GoBinaryManager) InstallPackage(ctx context.Context, pkgVersion string) error {
	pkg := pkgVersion
	version := "latest"

	//nolint:mnd // expected package version format: package@version
	if parts := strings.Split(pkgVersion, "@"); len(parts) == 2 {
		pkg = parts[0]
		version = parts[1]
	}

	return m.toolchain.Install(ctx, pkg, version)
}

// ListBinariesFullPaths lists all binaries in a directory. It returns the list
// of full paths to the binaries.
func (m *GoBinaryManager) ListBinariesFullPaths(dir string) ([]string, error) {
	logger := slog.Default().With("dir", dir)

	entries, err := m.system.ReadDir(dir)
	if err != nil {
		logger.Error("error while reading binaries directory", "err", err)
		return nil, err
	}

	binaries := make([]string, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if m.isBinary(fullPath) {
			binaries = append(binaries, fullPath)
		}
	}

	return binaries, nil
}

// UpgradeBinary upgrades a binary leveraging the toolchain. It gets the binary
// info and upgrade info, and installs the binary if an upgrade is available or
// if the rebuild flag is set.
func (m *GoBinaryManager) UpgradeBinary(
	ctx context.Context,
	binFullPath string,
	majorUpgrade bool,
	rebuild bool,
) error {
	info, err := m.GetBinaryInfo(binFullPath)
	if err != nil {
		return err
	}

	binUpInfo, err := m.GetBinaryUpgradeInfo(ctx, info, majorUpgrade)
	if err != nil {
		return err
	}

	if binUpInfo.IsUpgradeAvailable || rebuild {
		return m.installBinary(ctx, binUpInfo)
	}

	return nil
}

// checkBinaryDuplicatesInPath checks for duplicate binaries in the PATH
// environment variable. It returns a list of full paths to the duplicate
// binaries, or nil if there are no duplicates.
func (m *GoBinaryManager) checkBinaryDuplicatesInPath(name string) []string {
	duplicates := []string{}
	seen := make(map[string]struct{})

	path, _ := m.system.GetEnvVar("PATH")
	for dir := range strings.SplitSeq(path, string(m.system.PathListSeparator())) {
		fullPath := filepath.Join(dir, name)
		if m.isBinary(fullPath) {
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

// checkModuleMajorUpgrade checks if a module has a major version upgrade
// available leveraging the toolchain. It returns the latest module path and
// major version if an upgrade is available, otherwise it returns the original
// module path and version. It adjusts the package path to include the major
// version, following the Go module versioning rules.
func (m *GoBinaryManager) checkModuleMajorUpgrade(
	ctx context.Context,
	module, version string,
) (string, string, error) {
	latestModulePath := module
	latestMajorVersion := version

	pkg := module
	if major := semver.Major(version); major != "v0" && major != "v1" {
		pkg = stripVersionSuffix(module)
	}

	for {
		pkgMajor := pkg + "/" + nextMajorVersion(latestMajorVersion)
		modulePath, majorVersion, err := m.toolchain.GetLatestModuleVersion(ctx, pkgMajor)
		if errors.Is(err, ErrModuleNotFound) {
			break
		} else if err != nil {
			return "", "", err
		}

		latestModulePath = modulePath
		latestMajorVersion = majorVersion
	}

	return latestModulePath, latestMajorVersion, nil
}

// diagnoseGoModFile diagnoses the Go module file for a given module and
// version leveraging the toolchain. It returns the retracted and deprecated
// information if available.
func (m *GoBinaryManager) diagnoseGoModFile(
	ctx context.Context,
	module, version string,
) (string, string, error) {
	modFile, err := m.toolchain.GetModuleFile(ctx, module, "latest")
	if err != nil {
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

	return retracted, deprecated, nil
}

// installBinary installs a binary leveraging the toolchain. If the latest
// version is a major version v2 or higher, it adjusts the package path
// to include the major version, following the Go module versioning rules.
func (m *GoBinaryManager) installBinary(
	ctx context.Context,
	info BinaryUpgradeInfo,
) error {
	baseModule := stripVersionSuffix(info.LatestModulePath)
	packageSuffix := strings.TrimPrefix(info.PackagePath, info.ModulePath)

	pkg := baseModule + packageSuffix
	if major := semver.Major(info.LatestVersion); major != "v0" && major != "v1" {
		pkg = baseModule + "/" + major + packageSuffix
	}

	return m.toolchain.Install(ctx, pkg, info.LatestVersion)
}

// isBinary checks if a path is a binary file. It returns true if the path is a
// regular file and executable for Unix, or if it is a Windows executable.
func (m *GoBinaryManager) isBinary(path string) bool {
	info, err := m.system.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if m.system.RuntimeOS() == "windows" {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}

	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// getBinaryPlatform returns the platform of a binary based on the build info.
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

// nextMajorVersion returns the next major version for a given version.
func nextMajorVersion(version string) string {
	major := semver.Major(version)
	if major == "v0" || major == "v1" {
		return "v2"
	}

	majorNumStr := strings.TrimPrefix(major, "v")
	majorNum, _ := strconv.Atoi(majorNumStr)
	return "v" + strconv.Itoa(majorNum+1)
}

// stripVersionSuffix removes the version suffix from a module path, or returns
// the module path unchanged if it does not have a version suffix.
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
