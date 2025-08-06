package internal

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
)

var (
	// ErrBinaryBuiltWithoutGoModules indicates the binary was built without
	// module support.
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")

	// ErrBinaryNotFound indicates the binary was not found.
	ErrBinaryNotFound = errors.New("binary not found")

	// ErrModuleNotFound indicates the module was not found.
	ErrModuleNotFound = errors.New("module not found")

	// ErrModuleInfoNotAvailable indicates the module info is not available.
	ErrModuleInfoNotAvailable = errors.New("module info not available")

	// ErrModuleOriginNotAvailable indicates the module origin is not available.
	ErrModuleOriginNotAvailable = errors.New("module origin not available")
)

// ModuleOrigin represents the origin of a module containing the version control
// system, the URL, the hash and optionally the reference.
type ModuleOrigin struct {
	VCS  string  `json:"VCS"`
	URL  string  `json:"URL"`
	Hash string  `json:"Hash"`
	Ref  *string `json:"Ref"`
}

// Vulnerability represents a vulnerability found in a binary.
type Vulnerability struct {
	ID  string
	URL string
}

// GoToolchain is a toolchain to interact with the Go toolchain.
type GoToolchain struct {
	execCombinedOutput ExecCombinedOutputFunc
	execRun            ExecRunFunc
	scanCombinedOutput ScanExecCombinedOutputFunc
	system             System
}

// NewGoToolchain creates a new GoToolchain to interact with the Go toolchain.
func NewGoToolchain(
	execCombinedOutput ExecCombinedOutputFunc,
	execRun ExecRunFunc,
	scanCombinedOutput ScanExecCombinedOutputFunc,
	system System,
) *GoToolchain {
	return &GoToolchain{
		execCombinedOutput: execCombinedOutput,
		execRun:            execRun,
		scanCombinedOutput: scanCombinedOutput,
		system:             system,
	}
}

// GetBuildInfo returns the build info for a binary. It fails if the binary does
// not exist or was not built with Go modules.
func (t *GoToolchain) GetBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	logger := slog.Default().With("path", path)

	info, err := t.system.ReadBuildInfo(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrBinaryNotFound
		}

		logger.Error("error reading binary build info", "err", err)
		return nil, err
	}

	if info.Main.Path == "" {
		logger.Error(ErrBinaryBuiltWithoutGoModules.Error())
		return nil, ErrBinaryBuiltWithoutGoModules
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
	module string,
) (string, string, error) {
	logger := slog.Default().With("module", module)

	modLatest := fmt.Sprintf("%s@latest", module)
	cmd := t.execCombinedOutput(ctx, "go", "list", "-m", "-json", modLatest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			err = fmt.Errorf("%w: %s", err, outputStr)
		}

		if isModuleNotFound(err.Error()) {
			return "", "", ErrModuleNotFound
		}

		logger.Error("error getting latest version for module", "err", err)
		return "", "", err
	}

	var res struct {
		GoMod   string `json:"GoMod"`
		Version string `json:"Version"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.Error("error parsing module latest version response", "err", err)
		return "", "", err
	}

	logger = logger.With("go_mod_file", res.GoMod, "go_mod_version", res.Version)

	bytes, err := os.ReadFile(res.GoMod)
	if err != nil {
		logger.Error("error reading go mod file", "err", err)
		return "", "", err
	}

	modFile, err := modfile.Parse("go.mod", bytes, nil)
	if err != nil {
		logger.Error("error parsing go mod file", "err", err)
		return "", "", err
	}

	if modFile.Module == nil {
		err = ErrModuleInfoNotAvailable
		logger.Error("module info not available in go mod file", "err", err)
		return "", "", err
	}

	return modFile.Module.Mod.Path, res.Version, nil
}

// GetModuleFile returns the go.mod file for a module. It uses the go mod download
// command with the module path and version to download the go.mod file and retrive
// the location of it. It then parses the go.mod file and returns it. It fails if
// the module is not found or the go mod download command fails.
func (t *GoToolchain) GetModuleFile(
	ctx context.Context,
	module, version string,
) (*modfile.File, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := t.execCombinedOutput(ctx, "go", "mod", "download", "-json", modVersion)

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if jsonErr := json.Unmarshal(output, &res); jsonErr == nil {
			err = errors.New(res.Error)
		}

		logger.Error("error downloading module", "err", err)
		return nil, err
	}

	var res struct {
		GoMod string `json:"GoMod"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.Error("error parsing module download response", "err", err)
		return nil, err
	}

	logger = logger.With("go_mod_file", res.GoMod)

	bytes, err := os.ReadFile(res.GoMod)
	if err != nil {
		logger.Error("error reading go mod file", "err", err)
		return nil, err
	}

	modFile, err := modfile.Parse("go.mod", bytes, nil)
	if err != nil {
		logger.Error("error parsing go mod file", "err", err)
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
	module, version string,
) (*ModuleOrigin, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := t.execCombinedOutput(ctx, "go", "mod", "download", "-json", modVersion)

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if jsonErr := json.Unmarshal(output, &res); jsonErr == nil {
			err = errors.New(res.Error)
		}

		if isModuleNotFound(err.Error()) {
			return nil, ErrModuleNotFound
		}

		logger.Error("error downloading module", "err", err)
		return nil, err
	}

	var res struct {
		Origin *ModuleOrigin `json:"Origin"`
	}

	if err = json.Unmarshal(output, &res); err != nil {
		logger.Error("error parsing module download response", "err", err)
		return nil, err
	}

	if res.Origin == nil {
		err = ErrModuleOriginNotAvailable
		logger.Error("module origin not available", "err", err)
		return nil, err
	}

	return res.Origin, nil
}

// Install installs a package and its dependencies for the specified version.
// It uses the go install command with the -a option to force the rebuild
// of the package and its dependencies, even if they are already up to date.
// It fails if the go install command fails.
func (t *GoToolchain) Install(
	ctx context.Context,
	pkg, version string,
) error {
	logger := slog.Default().With("package", pkg, "version", version)

	pkgVersion := fmt.Sprintf("%s@%s", pkg, version)
	cmd := t.execRun(ctx, "go", "install", "-a", pkgVersion)

	if err := cmd.Run(); err != nil {
		logger.Error("error installing binary", "err", err)
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
) ([]Vulnerability, error) {
	logger := slog.Default().With("path", path)

	cmd := t.scanCombinedOutput(ctx, "-mode", "binary", "-format", "openvex", path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			err = fmt.Errorf("%w: %s", err, outputStr)
		}

		logger.Error("error running govulncheck command", "err", err)
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
		logger.Error("error parsing govulncheck response", "err", err)
		return nil, err
	}

	var vulns = make([]Vulnerability, 0, len(res.Statements))
	for _, stmt := range res.Statements {
		if stmt.Status == "affected" {
			vulns = append(vulns, Vulnerability{
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
