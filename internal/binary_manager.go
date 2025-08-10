package internal

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	// ErrBinaryAlreadyManaged is returned when a binary is already managed.
	ErrBinaryAlreadyManaged = errors.New("binary already managed")
)

// BinaryInfo represents the information for a binary.
type BinaryInfo struct {
	Name           string
	FullPath       string
	InstallPath    string
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

	IsManaged bool
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
	Name                  string
	NotInPath             bool
	DuplicatesInPath      []string
	IsNotManaged          bool
	IsPseudoVersion       bool
	NotBuiltWithGoModules bool
	IsOrphaned            bool
	GoVersion             struct {
		Actual   string
		Expected string
	}
	Platform struct {
		Actual   string
		Expected string
	}
	Retracted       string
	Deprecated      string
	Vulnerabilities []Vulnerability
}

// HasIssues returns whether the binary has any issues.
func (d BinaryDiagnostic) HasIssues() bool {
	return d.NotInPath ||
		len(d.DuplicatesInPath) > 0 ||
		d.IsNotManaged ||
		d.IsPseudoVersion ||
		d.NotBuiltWithGoModules ||
		d.IsOrphaned ||
		d.GoVersion.Actual != d.GoVersion.Expected ||
		d.Platform.Actual != d.Platform.Expected ||
		d.Retracted != "" ||
		d.Deprecated != "" ||
		len(d.Vulnerabilities) > 0
}

// BinaryManager is an interface for a binary manager.
type BinaryManager interface {
	// DiagnoseBinary diagnoses issues in a binary.
	DiagnoseBinary(ctx context.Context, path string) (BinaryDiagnostic, error)
	// GetAllBinaryInfos gets all binary infos.
	GetAllBinaryInfos(managed bool) ([]BinaryInfo, error)
	// GetBinaryInfo gets the binary info for a given path.
	GetBinaryInfo(path string) (BinaryInfo, error)
	// GetBinaryRepository gets the repository URL for a given binary.
	GetBinaryRepository(ctx context.Context, binary string) (string, error)
	// GetBinaryUpgradeInfo gets the upgrade information for a given binary.
	GetBinaryUpgradeInfo(ctx context.Context, info BinaryInfo, checkMajor bool) (BinaryUpgradeInfo, error)
	// InstallPackage installs a package.
	InstallPackage(ctx context.Context, pkgVersion string) error
	// ListBinariesFullPaths lists all binary full paths in the Go binary directory.
	ListBinariesFullPaths(dir string) ([]string, error)
	// MigrateBinary migrates a binary to be managed internally.
	MigrateBinary(path string) error
	// PinBinary pins a binary to the Go binary directory with the given kind.
	PinBinary(bin string, kind Kind) error
	// UninstallBinary uninstalls a binary.
	UninstallBinary(bin string) error
	// UpgradeBinary upgrades a binary.
	UpgradeBinary(ctx context.Context, binFullPath string, majorUpgrade bool, rebuild bool) error
}

// GoBinaryManager is a manager for Go binaries.
type GoBinaryManager struct {
	system    System
	toolchain Toolchain
	workspace Workspace
}

// NewGoBinaryManager creates a new GoBinaryManager.
func NewGoBinaryManager(
	system System,
	toolchain Toolchain,
	workspace Workspace,
) *GoBinaryManager {
	return &GoBinaryManager{
		system:    system,
		toolchain: toolchain,
		workspace: workspace,
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

	isSymlinkToDir, _ := m.isSymlinkToDir(path, m.workspace.GetInternalBinPath())
	diagnostic.IsNotManaged = !isSymlinkToDir

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

// GetAllBinaryInfos gets all binary infos in the Go binary directory or managed
// binaries only if managed is true. It returns a list of binary infos, or an
// error if the binary directory cannot be determined or listed. It skips
// silently failures to get the binary info.
func (m *GoBinaryManager) GetAllBinaryInfos(managed bool) ([]BinaryInfo, error) {
	path := m.workspace.GetGoBinPath()
	if managed {
		path = m.workspace.GetInternalBinPath()
	}

	bins, err := m.ListBinariesFullPaths(path)
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

	installPath := path
	target, err := m.system.Readlink(path)
	if err == nil {
		installPath = target
	}

	binInfo := BinaryInfo{
		Name:          strings.Split(filepath.Base(path), "@")[0],
		FullPath:      path,
		InstallPath:   installPath,
		PackagePath:   info.Path,
		ModulePath:    info.Main.Path,
		ModuleVersion: info.Main.Version,
		ModuleSum:     info.Main.Sum,
		GoVersion:     info.GoVersion,
		IsManaged:     strings.HasPrefix(installPath, m.workspace.GetInternalBinPath()),
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
	binInfo, err := m.GetBinaryInfo(filepath.Join(m.workspace.GetGoBinPath(), binary))
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

// InstallPackage installs a package leveraging the toolchain. If version is
// not provided, it installs the latest version.
func (m *GoBinaryManager) InstallPackage(ctx context.Context, pkgVersion string) error {
	pkg, version := pkgVersion, latest

	//nolint:mnd // expected package version format: package@version
	if parts := strings.Split(pkgVersion, "@"); len(parts) == 2 {
		pkg, version = parts[0], parts[1]
	}

	return m.installBinary(ctx, pkg, version)
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

// MigrateBinary migrates a binary to be managed internally. It gets the binary
// info, moves the binary from the go bin path to the internal bin path, and
// creates a symlink to the go bin path.
func (m *GoBinaryManager) MigrateBinary(path string) error {
	logger := slog.Default()

	info, err := m.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	if info.IsManaged {
		return ErrBinaryAlreadyManaged
	}

	internalBinPath := filepath.Join(
		m.workspace.GetInternalBinPath(),
		info.Name+"@"+info.ModuleVersion,
	)

	logger.Info(
		"moving binary from go bin path to internal bin path",
		"go_bin_path", path, "internal_bin_path", internalBinPath,
	)

	if err = m.system.Rename(path, internalBinPath); err != nil {
		logger.Error(
			"error while moving binary from go bin path to internal bin path",
			"err", err, "src", path, "dst", internalBinPath,
		)
		return err
	}

	logger.Info(
		"creating symlink for binary",
		"internal_bin_path", internalBinPath, "go_bin_path", path,
	)

	if err = m.system.Symlink(internalBinPath, path); err != nil {
		logger.Error(
			"error while creating symlink for binary",
			"err", err, "src", internalBinPath, "dst", path,
		)
		return err
	}

	return nil
}

// PinBinary pins a binary to the Go binary directory with the given kind. It
// creates a symlink to the binary in the Go binary directory with names binary,
// binary-major, or binary-major.minor if kind is latest, major, or minor
// respectively.
func (m *GoBinaryManager) PinBinary(bin string, kind Kind) error {
	logger := slog.Default().With("bin", bin, "kind", kind.String())

	parts := strings.Split(bin, "@")
	reqName, reqVersion := parts[0], latest
	//nolint:mnd // expected binary format: name@version
	if len(parts) == 2 {
		reqVersion = parts[1]
	}

	binPaths, err := m.ListBinariesFullPaths(m.workspace.GetInternalBinPath())
	if err != nil {
		return err
	}

	var matchPath, matchVersion string
	for _, binPath := range binPaths {
		parts = strings.Split(filepath.Base(binPath), "@")
		intName, intVersion := parts[0], parts[1]

		if reqName != intName {
			continue
		}

		if reqVersion == latest {
			if matchPath == "" {
				matchPath = binPath
				matchVersion = intVersion
				continue
			}

			if semver.Compare(intVersion, matchVersion) > 0 {
				matchPath = binPath
				matchVersion = intVersion
			}
			continue
		}

		if isVersionPartOf(intVersion, reqVersion) &&
			semver.Compare(intVersion, matchVersion) > 0 {
			matchPath = binPath
			matchVersion = intVersion
		}
	}

	if matchPath == "" {
		logger.Warn("binary not found")
		return ErrBinaryNotFound
	}

	logger.Info("found binary to pin", "path", matchPath)

	targetPath := getTargetPath(m.workspace.GetGoBinPath(), reqName, matchVersion, kind)

	logger.Info("removing existing symlink for binary", "path", targetPath)

	if err = m.system.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		logger.Error(
			"error while removing existing symlink for binary",
			"err", err, "path", targetPath,
		)
		return err
	}

	logger.Info("creating symlink for binary", "src", matchPath, "dst", targetPath)

	if err = m.system.Symlink(matchPath, targetPath); err != nil {
		logger.Error(
			"error while creating symlink",
			"err", err, "src", matchPath, "dst", targetPath,
		)
		return err
	}

	return nil
}

// UninstallBinary uninstalls a binary by removing the binary file. It removes
// the binary from the go bin path for unmanaged binaries, or removes the
// symlink for managed binaries. It returns an error if the binary cannot be
// found or removed.
func (m *GoBinaryManager) UninstallBinary(bin string) error {
	logger := slog.Default().With("bin", bin)

	err := m.system.Remove(filepath.Join(m.workspace.GetGoBinPath(), bin))
	if errors.Is(err, os.ErrNotExist) {
		logger.Warn("binary not found")
	} else if err != nil {
		logger.Error("failed to remove binary", "err", err)
	}

	return err
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
		pkg, version := getBinaryUpgradePackageVersion(binUpInfo)
		return m.installBinary(ctx, pkg, version)
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
	modFile, err := m.toolchain.GetModuleFile(ctx, module, latest)
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
func (m *GoBinaryManager) installBinary(ctx context.Context, pkg, version string) error {
	logger := slog.Default().With("pkg", pkg, "version", version)

	tempDir := m.workspace.GetInternalTempPath()
	parts := strings.Split(pkg, "/")
	binName := parts[len(parts)-1]
	if semver.Major(binName) != "" {
		binName = parts[len(parts)-2]
	}

	logger.InfoContext(ctx, "creating internal binary temp directory")

	binTempDir, err := m.system.MkdirTemp(tempDir, binName+"-*")
	if err != nil {
		logger.ErrorContext(
			ctx, "error while creating internal binary temp directory",
			"err", err, "temp_dir", tempDir,
		)
		return err
	}
	defer func() {
		logger.Info("removing internal binary temp directory")

		if err = m.system.RemoveAll(binTempDir); err != nil {
			logger.ErrorContext(
				ctx, "error while removing internal binary temp directory",
				"err", err, "temp_dir", binTempDir,
			)
		}
	}()

	if err = m.toolchain.Install(ctx, binTempDir, pkg, version); err != nil {
		return err
	}

	tempBinPath := filepath.Join(binTempDir, binName)
	buildInfo, err := m.toolchain.GetBuildInfo(tempBinPath)
	if err != nil {
		logger.ErrorContext(
			ctx, "error while getting build info for internal binary",
			"err", err, "temp_bin_path", tempBinPath,
		)
		return err
	}

	binDir := m.workspace.GetInternalBinPath()
	binPath := filepath.Join(binDir, binName+"@"+buildInfo.Main.Version)

	logger.InfoContext(
		ctx, "moving binary from temp path to bin path",
		"temp_path", tempBinPath, "bin_path", binPath,
	)

	if err = m.system.Rename(tempBinPath, binPath); err != nil {
		logger.ErrorContext(
			ctx, "error while moving binary from temp path to bin path",
			"err", err, "src", tempBinPath, "dst", binPath,
		)
		return err
	}

	goBinPath := filepath.Join(m.workspace.GetGoBinPath(), binName)

	logger.InfoContext(
		ctx, "removing existing symlink for binary",
		"go_bin_path", goBinPath,
	)

	if err = m.system.Remove(goBinPath); err != nil && !os.IsNotExist(err) {
		logger.ErrorContext(
			ctx, "error while removing existing symlink for binary",
			"err", err, "path", goBinPath,
		)
		return err
	}

	logger.InfoContext(
		ctx, "creating symlink for binary",
		"bin_path", binPath, "go_bin_path", goBinPath,
	)

	if err = m.system.Symlink(binPath, goBinPath); err != nil {
		logger.ErrorContext(
			ctx, "error while creating symlink for binary",
			"err", err, "src", binPath, "dst", goBinPath,
		)
		return err
	}

	return nil
}

// isBinary checks if a path is a binary file. It returns true if the path is a
// regular file and executable for Unix, or if it is a Windows executable.
func (m *GoBinaryManager) isBinary(path string) bool {
	info, err := m.system.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if m.system.RuntimeOS() == windowsOS {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}

	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// isSymlinkToDir checks if a path is a symlink to another directory.
func (m *GoBinaryManager) isSymlinkToDir(path string, baseDir string) (bool, error) {
	info, err := m.system.LStat(path)
	if err != nil {
		return false, err
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}

	target, err := m.system.Readlink(path)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(target, baseDir+string(os.PathSeparator)), nil
}

// getBinaryUpgradePackageVersion returns the package and version for a binary
// upgrade. If the latest version is a major version v2 or higher, it adjusts
// the package path to include the major version, following the Go module
// versioning rules.
func getBinaryUpgradePackageVersion(info BinaryUpgradeInfo) (string, string) {
	baseModule := stripVersionSuffix(info.LatestModulePath)
	packageSuffix := strings.TrimPrefix(info.PackagePath, info.ModulePath)

	pkg := baseModule + packageSuffix
	if major := semver.Major(info.LatestVersion); major != "v0" && major != "v1" {
		pkg = baseModule + "/" + major + packageSuffix
	}

	return pkg, info.LatestVersion
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

// getTargetPath returns the target path for a binary based on the base path,
// name, version, and kind.
func getTargetPath(basePath, name, version string, kind Kind) string {
	var targetPath string
	switch kind {
	case KindLatest:
		targetPath = filepath.Join(basePath, name)
	case KindMajor:
		targetPath = filepath.Join(basePath, name+"-"+semver.Major(version))
	case KindMinor:
		targetPath = filepath.Join(basePath, name+"-"+semver.MajorMinor(version))
	}

	return targetPath
}

// isVersionPartOf checks if a full version is part of a base version. If base
// version is a major or major.minor version, it checks if the full version is
// greater than or equal to the base version and less than the next major or
// major.minor version. Otherwise it performs a full version comparison.
func isVersionPartOf(fullVersion, baseVersion string) bool {
	parts := strings.Split(strings.TrimPrefix(baseVersion, "v"), ".")

	switch len(parts) {
	case 1:
		lower := fmt.Sprintf("v%d.0.0", Must(strconv.Atoi(parts[0])))
		upper := fmt.Sprintf("v%d.0.0", Must(strconv.Atoi(parts[0]))+1)
		return semver.Compare(fullVersion, lower) >= 0 && semver.Compare(fullVersion, upper) < 0

	case 2: //nolint:mnd // expected version format: major.minor
		major := Must(strconv.Atoi(parts[0]))
		minor := Must(strconv.Atoi(parts[1]))
		lower := fmt.Sprintf("v%d.%d.0", major, minor)
		upper := fmt.Sprintf("v%d.%d.0", major, minor+1)
		return semver.Compare(fullVersion, lower) >= 0 && semver.Compare(fullVersion, upper) < 0
	default:
		return semver.Compare(fullVersion, baseVersion) == 0
	}
}

// nextMajorVersion returns the next major version for a given version.
func nextMajorVersion(version string) string {
	major := semver.Major(version)
	if major == "v0" || major == "v1" {
		return "v2"
	}

	return "v" + strconv.Itoa(Must(strconv.Atoi(strings.TrimPrefix(major, "v")))+1)
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
