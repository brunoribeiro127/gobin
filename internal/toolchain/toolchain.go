package toolchain

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/brunoribeiro127/gobin/internal/system"
)

var (
	// ErrBinaryBuiltWithoutGoModules indicates the binary was built without
	// module support.
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")

	// ErrBinaryNotFound indicates the binary was not found.
	ErrBinaryNotFound = errors.New("binary not found")

	// ErrGoModFileNotAvailable indicates the go mod file is not available.
	ErrGoModFileNotAvailable = errors.New("go mod file not available")

	// ErrModuleNotFound indicates the module was not found.
	ErrModuleNotFound = errors.New("module not found")

	// ErrModuleInfoNotAvailable indicates the module info is not available.
	ErrModuleInfoNotAvailable = errors.New("module info not available")

	// ErrModuleOriginNotAvailable indicates the module origin is not available.
	ErrModuleOriginNotAvailable = errors.New("module origin not available")
)

// Toolchain is an interface for a toolchain.
type Toolchain interface {
	// GetBuildInfo gets the build info for a binary.
	GetBuildInfo(
		path string,
	) (*buildinfo.BuildInfo, error)
	// GetLatestModuleVersion gets the latest module version for a given module.
	GetLatestModuleVersion(
		ctx context.Context,
		module model.Module,
	) (model.Module, error)
	// GetModuleFile gets the module file for a given module and version.
	GetModuleFile(
		ctx context.Context,
		module model.Module,
	) (*modfile.File, error)
	// GetModuleOrigin gets the module origin for a given module and version.
	GetModuleOrigin(
		ctx context.Context,
		module model.Module,
	) (*model.ModuleOrigin, error)
	// Install installs a package in the target path.
	Install(
		ctx context.Context,
		path string,
		pkg model.Package,
	) error
	// VulnCheck checks for vulnerabilities in a binary.
	VulnCheck(
		ctx context.Context,
		path string,
	) ([]model.Vulnerability, error)
}

// GoToolchain is a toolchain to interact with the Go toolchain.
type GoToolchain struct {
	buildInfo system.BuildInfo
	exec      system.Exec
	scanExec  ScanExecCombinedOutputFunc
}

// NewGoToolchain creates a new GoToolchain to interact with the Go toolchain.
func NewGoToolchain(
	buildInfo system.BuildInfo,
	exec system.Exec,
	scanExec ScanExecCombinedOutputFunc,
) *GoToolchain {
	return &GoToolchain{
		buildInfo: buildInfo,
		exec:      exec,
		scanExec:  scanExec,
	}
}

// GetBuildInfo returns the build info for a binary. It fails if the binary does
// not exist or was not built with Go modules.
func (t *GoToolchain) GetBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	logger := slog.Default().With("path", path)
	logger.Info("getting build info")

	info, err := t.buildInfo.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrBinaryNotFound
		}

		logger.Error("error reading binary build info", "err", err)
		return nil, err
	}

	if info.Main.Path == "" {
		err = ErrBinaryBuiltWithoutGoModules
		logger.Warn(err.Error())
		return nil, err
	}

	return info, nil
}

// GetLatestModuleVersion returns the latest module path and version of a module.
// It uses the go list command with the option -m -json to get a json response with
// the path to the go.mod file and the latest version. It fails if the module is
// not found, the go list command fails or the go.mod file does not contain the
// module information.
func (t *GoToolchain) GetLatestModuleVersion(
	ctx context.Context,
	module model.Module,
) (model.Module, error) {
	logger := slog.Default().With("module", module.Path)
	logger.InfoContext(ctx, "getting latest module version")

	modLatest := module.Path + "@latest"
	cmd := t.exec.CombinedOutput(ctx, "go", "list", "-m", "-json", modLatest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			err = fmt.Errorf("%w: %s", err, outputStr)
		}

		if isModuleNotFound(err.Error()) {
			logger.WarnContext(ctx, "module not found", "err", err)
			return model.Module{}, ErrModuleNotFound
		}

		logger.ErrorContext(ctx, "error getting latest version for module", "err", err)
		return model.Module{}, err
	}

	var res struct {
		GoMod   *string `json:"GoMod"`
		Version string  `json:"Version"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.ErrorContext(ctx, "error parsing module latest version response", "err", err)
		return model.Module{}, err
	}

	if res.GoMod == nil {
		err = ErrGoModFileNotAvailable
		logger.WarnContext(ctx, err.Error())
		return model.Module{}, err
	}

	logger = logger.With("go_mod_file", *res.GoMod, "go_mod_version", res.Version)

	bytes, err := os.ReadFile(*res.GoMod)
	if err != nil {
		logger.ErrorContext(ctx, "error reading go mod file", "err", err)
		return model.Module{}, err
	}

	modFile, err := modfile.Parse("go.mod", bytes, nil)
	if err != nil {
		logger.ErrorContext(ctx, "error parsing go mod file", "err", err)
		return model.Module{}, err
	}

	if modFile.Module == nil {
		err = ErrModuleInfoNotAvailable
		logger.WarnContext(ctx, err.Error())
		return model.Module{}, err
	}

	return model.NewModule(modFile.Module.Mod.Path, model.NewVersion(res.Version)), nil
}

// GetModuleFile returns the go.mod file for a module. It uses the go mod download
// command with the module path and version to download the go.mod file and retrive
// the location of it. It then parses the go.mod file and returns it. It fails if
// the module is not found or the go mod download command fails.
func (t *GoToolchain) GetModuleFile(
	ctx context.Context,
	module model.Module,
) (*modfile.File, error) {
	logger := slog.Default().With("module", module.String())
	logger.InfoContext(ctx, "getting module file")

	cmd := t.exec.CombinedOutput(ctx, "go", "mod", "download", "-json", module.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if jsonErr := json.Unmarshal(output, &res); jsonErr == nil {
			err = errors.New(res.Error)
		}

		logger.ErrorContext(ctx, "error downloading module", "err", err)
		return nil, err
	}

	var res struct {
		GoMod string `json:"GoMod"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.ErrorContext(ctx, "error parsing module download response", "err", err)
		return nil, err
	}

	logger = logger.With("go_mod_file", res.GoMod)

	bytes, err := os.ReadFile(res.GoMod)
	if err != nil {
		logger.ErrorContext(ctx, "error reading go mod file", "err", err)
		return nil, err
	}

	modFile, err := modfile.Parse("go.mod", bytes, nil)
	if err != nil {
		logger.ErrorContext(ctx, "error parsing go mod file", "err", err)
		return nil, err
	}

	return modFile, nil
}

// GetModuleOrigin returns the origin of a module. It uses the go mod download
// command with the module path and version to download the go.mod file and retrive
// the origin of the module. It fails if the module is not found or the go mod
// download command fails.
func (t *GoToolchain) GetModuleOrigin(
	ctx context.Context,
	module model.Module,
) (*model.ModuleOrigin, error) {
	logger := slog.Default().With("module", module.String())
	logger.InfoContext(ctx, "getting module origin")

	cmd := t.exec.CombinedOutput(ctx, "go", "mod", "download", "-json", module.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if jsonErr := json.Unmarshal(output, &res); jsonErr == nil {
			err = errors.New(res.Error)
		}

		if isModuleNotFound(err.Error()) {
			logger.WarnContext(ctx, "module not found", "err", err)
			return nil, ErrModuleNotFound
		}

		logger.ErrorContext(ctx, "error downloading module", "err", err)
		return nil, err
	}

	var res struct {
		Origin *model.ModuleOrigin `json:"Origin"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.ErrorContext(ctx, "error parsing module download response", "err", err)
		return nil, err
	}

	if res.Origin == nil {
		err = ErrModuleOriginNotAvailable
		logger.WarnContext(ctx, err.Error())
		return nil, err
	}

	return res.Origin, nil
}

// Install installs a package and its dependencies for the specified version in
// the target path. It uses the go install command with the -a option to force
// the rebuild of the package and its dependencies, even if they are already
// up to date. It fails if the go install command fails.
func (t *GoToolchain) Install(
	ctx context.Context,
	path string,
	pkg model.Package,
) error {
	logger := slog.Default().With("path", path, "package", pkg.String())
	logger.InfoContext(ctx, "installing package")

	cmd := t.exec.Run(ctx, "go", "install", "-a", pkg.String())
	cmd.InjectEnv("GOBIN=" + path)

	if err := cmd.Run(); err != nil {
		logger.ErrorContext(ctx, "error installing binary", "err", err)
		return err
	}

	return nil
}

// VulnCheck runs the govulncheck command to check for vulnerabilities in the
// target binary. It returns a list of vulnerabilities found in the binary. It
// uses the OpenVEX format and filters for affected vulnerabilities. It fails if
// the govulncheck command fails.
func (t *GoToolchain) VulnCheck(
	ctx context.Context,
	path string,
) ([]model.Vulnerability, error) {
	logger := slog.Default().With("path", path)
	logger.InfoContext(ctx, "running govulncheck")

	cmd := t.scanExec(ctx, "-mode", "binary", "-format", "openvex", path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			err = fmt.Errorf("%w: %s", err, outputStr)
		}

		logger.ErrorContext(ctx, "error running govulncheck command", "err", err)
		return nil, err
	}

	var res struct {
		Statements []struct {
			Vulnerability struct {
				ID   string `json:"@id"`
				Name string `json:"name"`
			} `json:"vulnerability"`
			Status string `json:"status"`
		} `json:"statements"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.ErrorContext(ctx, "error parsing govulncheck response", "err", err)
		return nil, err
	}

	var vulns = make([]model.Vulnerability, 0, len(res.Statements))
	for _, stmt := range res.Statements {
		if stmt.Status == "affected" {
			vulns = append(vulns, model.Vulnerability{
				ID:  stmt.Vulnerability.Name,
				URL: stmt.Vulnerability.ID,
			})
		}
	}

	return vulns, nil
}

// isModuleNotFound checks if the output contains a message indicating that a
// module was not found by a go command.
func isModuleNotFound(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "no matching versions for query") ||
		strings.Contains(output, "not found") ||
		strings.Contains(output, "unknown revision")
}
