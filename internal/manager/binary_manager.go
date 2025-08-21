package manager

import (
	"context"
	"debug/buildinfo"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"

	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/brunoribeiro127/gobin/internal/system"
	"github.com/brunoribeiro127/gobin/internal/toolchain"
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

// BinaryManager is an interface for a binary manager.
type BinaryManager interface {
	// DiagnoseBinary diagnoses issues in a binary.
	DiagnoseBinary(
		ctx context.Context,
		path string,
	) (model.BinaryDiagnostic, error)
	// GetAllBinaryInfos gets all binary infos.
	GetAllBinaryInfos(
		managed bool,
	) ([]model.BinaryInfo, error)
	// GetBinaryInfo gets the binary info for a given path.
	GetBinaryInfo(
		path string,
	) (model.BinaryInfo, error)
	// GetBinaryRepository gets the repository URL for a given binary.
	GetBinaryRepository(
		ctx context.Context,
		bin model.Binary,
	) (string, error)
	// GetBinaryUpgradeInfo gets the upgrade information for a given binary.
	GetBinaryUpgradeInfo(
		ctx context.Context,
		info model.BinaryInfo,
		checkMajor bool,
	) (model.BinaryUpgradeInfo, error)
	// InstallPackage installs a package.
	InstallPackage(
		ctx context.Context,
		pkg model.Package,
		kind model.Kind,
		rebuild bool,
	) error
	// MigrateBinary migrates a binary to be managed internally.
	MigrateBinary(
		path string,
	) error
	// PinBinary pins a binary to the Go binary directory with the given kind.
	PinBinary(
		bin model.Binary,
		kind model.Kind,
	) error
	// UninstallBinary uninstalls a binary.
	UninstallBinary(
		bin model.Binary,
	) error
	// UpgradeBinary upgrades a binary.
	UpgradeBinary(
		ctx context.Context,
		binFullPath string,
		majorUpgrade bool,
		rebuild bool,
	) error
}

// GoBinaryManager is a manager for Go binaries.
type GoBinaryManager struct {
	fs        system.FileSystem
	runtime   system.Runtime
	toolchain toolchain.Toolchain
	workspace system.Workspace
}

// NewGoBinaryManager creates a new GoBinaryManager.
func NewGoBinaryManager(
	fs system.FileSystem,
	runtime system.Runtime,
	toolchain toolchain.Toolchain,
	workspace system.Workspace,
) *GoBinaryManager {
	return &GoBinaryManager{
		fs:        fs,
		runtime:   runtime,
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
) (model.BinaryDiagnostic, error) {
	binaryName := filepath.Base(path)
	diagnostic := model.BinaryDiagnostic{
		Name: binaryName,
	}

	buildInfo, err := m.toolchain.GetBuildInfo(path)
	if err != nil {
		if errors.Is(err, toolchain.ErrBinaryBuiltWithoutGoModules) {
			diagnostic.NotBuiltWithGoModules = true
			return diagnostic, nil
		}

		return model.BinaryDiagnostic{}, err
	}

	binPlatform := getBinaryPlatform(buildInfo)
	runtimePlatform := m.runtime.Platform()
	diagnostic.IsPseudoVersion = module.IsPseudoVersion(buildInfo.Main.Version)
	diagnostic.IsOrphaned = buildInfo.Main.Sum == ""
	diagnostic.GoVersion.Actual = buildInfo.GoVersion
	diagnostic.GoVersion.Expected = m.runtime.Version()
	diagnostic.Platform.Actual = binPlatform
	diagnostic.Platform.Expected = runtimePlatform

	locations := m.fs.LocateBinaryInPath(binaryName)
	if len(locations) > 1 {
		diagnostic.DuplicatesInPath = locations
	}
	diagnostic.NotInPath = len(locations) == 0

	isSymlinkToDir, _ := m.fs.IsSymlinkToDir(path, m.workspace.GetInternalBinPath())
	diagnostic.IsNotManaged = !isSymlinkToDir

	if buildInfo.Main.Sum != "" {
		retracted, deprecated, modErr := m.diagnoseGoModFile(
			ctx, model.NewModule(buildInfo.Main.Path, model.NewVersion(buildInfo.Main.Version)),
		)
		if modErr != nil {
			return model.BinaryDiagnostic{}, modErr
		}

		diagnostic.Retracted = retracted
		diagnostic.Deprecated = deprecated
	}

	diagnostic.Vulnerabilities, err = m.toolchain.VulnCheck(ctx, path)
	if err != nil {
		return model.BinaryDiagnostic{}, err
	}

	return diagnostic, nil
}

// GetAllBinaryInfos gets all binary infos in the Go binary directory or managed
// binaries only if managed is true. It returns a list of binary infos, or an
// error if the binary directory cannot be determined or listed. It skips
// silently failures to get the binary info.
func (m *GoBinaryManager) GetAllBinaryInfos(managed bool) ([]model.BinaryInfo, error) {
	path := m.workspace.GetGoBinPath()
	if managed {
		path = m.workspace.GetInternalBinPath()
	}

	bins, err := m.fs.ListBinaries(path)
	if err != nil {
		return nil, err
	}

	binInfos := make([]model.BinaryInfo, 0, len(bins))
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
func (m *GoBinaryManager) GetBinaryInfo(path string) (model.BinaryInfo, error) {
	info, err := m.toolchain.GetBuildInfo(path)
	if err != nil {
		return model.BinaryInfo{}, err
	}

	installPath := path
	target, err := m.fs.GetSymlinkTarget(path)
	if err == nil {
		installPath = target
	}

	internalBinPath := m.workspace.GetInternalBinPath()

	binInfo := model.BinaryInfo{
		Name:        strings.Split(filepath.Base(path), "@")[0],
		FullPath:    path,
		InstallPath: installPath,
		PackagePath: info.Path,
		Module:      model.NewModule(info.Main.Path, model.NewVersion(info.Main.Version)),
		ModuleSum:   info.Main.Sum,
		GoVersion:   info.GoVersion,
		IsManaged:   strings.HasPrefix(installPath, internalBinPath),
	}

	if strings.HasPrefix(path, internalBinPath) {
		binPaths, err := m.fs.ListBinaries(m.workspace.GetGoBinPath())
		if err != nil {
			return model.BinaryInfo{}, err
		}

		for _, bin := range binPaths {
			if target, err := m.fs.GetSymlinkTarget(bin); err == nil {
				if target == path {
					binInfo.IsPinned = true
					break
				}
			}
		}
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
	bin model.Binary,
) (string, error) {
	binInfo, err := m.GetBinaryInfo(filepath.Join(m.workspace.GetGoBinPath(), bin.Name))
	if err != nil {
		return "", err
	}

	modOrigin, err := m.toolchain.GetModuleOrigin(ctx, binInfo.Module)
	if err != nil &&
		!errors.Is(err, toolchain.ErrModuleNotFound) &&
		!errors.Is(err, toolchain.ErrModuleOriginNotAvailable) {
		return "", err
	}

	repoURL := "https://" + binInfo.Module.Path
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
	info model.BinaryInfo,
	checkMajor bool,
) (model.BinaryUpgradeInfo, error) {
	binUpInfo := model.BinaryUpgradeInfo{
		BinaryInfo: info,
	}

	version := info.GetPinnedVersion()
	mod, err := m.toolchain.GetLatestModuleVersion(ctx, model.NewModule(binUpInfo.Module.Path, version))
	if err != nil {
		return model.BinaryUpgradeInfo{}, err
	}

	binUpInfo.LatestModule = mod

	if checkMajor && version.IsLatest() {
		for {
			mod, err = m.toolchain.GetLatestModuleVersion(ctx, mod.NextMajorModule())
			if errors.Is(err, toolchain.ErrModuleNotFound) {
				break
			} else if err != nil {
				return model.BinaryUpgradeInfo{}, err
			}

			binUpInfo.LatestModule = mod
		}
	}

	binUpInfo.IsUpgradeAvailable = binUpInfo.Module.Version.Compare(binUpInfo.LatestModule.Version) < 0

	return binUpInfo, nil
}

// InstallPackage installs a package leveraging the toolchain. If kind is major
// or minor, it pins the binary to the Go binary directory with the given kind.
// If rebuild is true, it rebuilds the binary.
func (m *GoBinaryManager) InstallPackage(
	ctx context.Context,
	pkg model.Package,
	kind model.Kind,
	rebuild bool,
) error {
	logger := slog.Default().With("pkg", pkg.String())

	tempDir := m.workspace.GetInternalTempPath()
	binName := pkg.GetBinaryName()

	logger.InfoContext(ctx, "creating internal binary temp directory")

	binTempDir, cleanup, err := m.fs.CreateTempDir(tempDir, binName+"-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = cleanup()
	}()

	if err = m.toolchain.Install(ctx, binTempDir, pkg, rebuild); err != nil {
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

	if err = m.fs.Move(tempBinPath, binPath); err != nil {
		logger.ErrorContext(
			ctx, "error while moving binary from temp path to bin path",
			"err", err, "src", tempBinPath, "dst", binPath,
		)
		return err
	}

	goBinPath := kind.GetTargetBinPath(
		m.workspace.GetGoBinPath(),
		binName,
		model.NewVersion(buildInfo.Main.Version),
	)

	logger.InfoContext(
		ctx, "replacing existing symlink for binary",
		"go_bin_path", goBinPath,
	)

	if err = m.fs.ReplaceSymlink(binPath, goBinPath); err != nil {
		return err
	}

	return nil
}

// MigrateBinary migrates a binary to be managed internally. It gets the binary
// info, moves the binary from the go bin path to the internal bin path, and
// creates a symlink to the go bin path.
func (m *GoBinaryManager) MigrateBinary(path string) error {
	logger := slog.Default().With("path", path)

	info, err := m.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	if info.IsManaged {
		return ErrBinaryAlreadyManaged
	}

	internalBinPath := filepath.Join(
		m.workspace.GetInternalBinPath(),
		info.Name+"@"+info.Module.Version.String(),
	)

	logger.Info(
		"moving binary from go bin path to internal bin path",
		"go_bin_path", path, "internal_bin_path", internalBinPath,
	)

	if err = m.fs.MoveWithSymlink(path, internalBinPath); err != nil {
		return err
	}

	return nil
}

// PinBinary pins a binary to the Go binary directory with the given kind. It
// creates a symlink to the binary in the Go binary directory with names binary,
// binary-major, or binary-major.minor if kind is latest, major, or minor
// respectively.
func (m *GoBinaryManager) PinBinary(bin model.Binary, kind model.Kind) error {
	logger := slog.Default().With("bin", bin.String(), "kind", kind.String())

	binPaths, err := m.fs.ListBinaries(m.workspace.GetInternalBinPath())
	if err != nil {
		return err
	}

	var matchPath string
	var matchVersion model.Version
	for _, binPath := range binPaths {
		intBin := model.NewBinary(filepath.Base(binPath))

		if !intBin.IsPartOf(bin) {
			continue
		}

		if matchPath == "" || intBin.Version.Compare(matchVersion) > 0 {
			matchPath = binPath
			matchVersion = intBin.Version
		}
	}

	if matchPath == "" {
		logger.Warn("binary not found")
		return toolchain.ErrBinaryNotFound
	}

	logger.Info("found binary to pin", "path", matchPath)

	targetPath := kind.GetTargetBinPath(m.workspace.GetGoBinPath(), bin.Name, matchVersion)

	logger.Info("removing existing symlink for binary", "path", targetPath)

	if err = m.fs.ReplaceSymlink(matchPath, targetPath); err != nil {
		return err
	}

	return nil
}

// UninstallBinary uninstalls a binary by removing the binary file. It removes
// the binary from the go bin path for unmanaged binaries, or removes the
// symlink for managed binaries. It returns an error if the binary cannot be
// found or removed.
func (m *GoBinaryManager) UninstallBinary(bin model.Binary) error {
	logger := slog.Default().With("bin", bin.Name)

	err := m.fs.Remove(filepath.Join(m.workspace.GetGoBinPath(), bin.Name))
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
		kind := model.GetKindFromName(binUpInfo.Name)
		return m.InstallPackage(ctx, binUpInfo.GetUpgradePackage(), kind, rebuild)
	}

	return nil
}

// diagnoseGoModFile diagnoses the Go module file for a given module and
// version leveraging the toolchain. It returns the retracted and deprecated
// information if available.
func (m *GoBinaryManager) diagnoseGoModFile(
	ctx context.Context,
	module model.Module,
) (string, string, error) {
	modFile, err := m.toolchain.GetModuleFile(ctx, model.NewLatestModule(module.Path))
	if err != nil {
		return "", "", err
	}

	var retracted string
	for _, r := range modFile.Retract {
		if model.NewVersion(r.Low).Compare(module.Version) <= 0 &&
			model.NewVersion(r.High).Compare(module.Version) >= 0 {
			retracted = r.Rationale
		}
	}

	var deprecated string
	if modFile.Module != nil && modFile.Module.Deprecated != "" {
		deprecated = modFile.Module.Deprecated
	}

	return retracted, deprecated, nil
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
