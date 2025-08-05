package internal

import (
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
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")
	ErrBinaryNotFound              = errors.New("binary not found")
	ErrModuleNotFound              = errors.New("module not found")
	ErrModuleInfoNotAvailable      = errors.New("module info not available")
	ErrModuleOriginNotAvailable    = errors.New("module origin not available")
)

type ModuleOrigin struct {
	VCS  string  `json:"VCS"`
	URL  string  `json:"URL"`
	Hash string  `json:"Hash"`
	Ref  *string `json:"Ref"`
}

type Vulnerability struct {
	ID  string
	URL string
}

type GoToolchain struct {
	execCombinedOutput ExecCombinedOutputFunc
	execRun            ExecRunFunc
	scanCombinedOutput ScanExecCombinedOutputFunc
	system             System
}

func NewGoToolchain(
	execCombinedOutput ExecCombinedOutputFunc,
	execRun ExecRunFunc,
	scanCombinedOutput ScanExecCombinedOutputFunc,
	system System,
) Toolchain {
	return &GoToolchain{
		execCombinedOutput: execCombinedOutput,
		execRun:            execRun,
		scanCombinedOutput: scanCombinedOutput,
		system:             system,
	}
}

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

func (t *GoToolchain) GetLatestModuleVersion(module string) (string, string, error) {
	logger := slog.Default().With("module", module)

	modLatest := fmt.Sprintf("%s@latest", module)
	cmd := t.execCombinedOutput("go", "list", "-m", "-json", modLatest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))

		if isModuleNotFound(outputStr) {
			return "", "", ErrModuleNotFound
		}

		err = errors.New(outputStr)
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

func (t *GoToolchain) GetModuleFile(module, version string) (*modfile.File, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := t.execCombinedOutput("go", "mod", "download", "-json", modVersion)

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if err = json.Unmarshal(output, &res); err != nil {
			logger.Error("error parsing module download response", "err", err)
			return nil, err
		}

		err = errors.New(res.Error)
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

func (t *GoToolchain) GetModuleOrigin(module, version string) (*ModuleOrigin, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := t.execCombinedOutput("go", "mod", "download", "-json", modVersion)

	output, err := cmd.CombinedOutput()
	if err != nil {
		var res struct {
			Error string `json:"Error"`
		}

		if err = json.Unmarshal(output, &res); err != nil {
			logger.Error("error parsing module download response", "err", err)
			return nil, err
		}

		if isModuleNotFound(res.Error) {
			return nil, ErrModuleNotFound
		}

		err = errors.New(res.Error)
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

func (t *GoToolchain) Install(pkg, version string) error {
	logger := slog.Default().With("package", pkg, "version", version)

	pkgVersion := fmt.Sprintf("%s@%s", pkg, version)
	cmd := t.execRun("go", "install", "-a", pkgVersion)

	if err := cmd.Run(); err != nil {
		logger.Error("error installing binary", "err", err)
		return err
	}

	return nil
}

func (t *GoToolchain) VulnCheck(path string) ([]Vulnerability, error) {
	logger := slog.Default().With("path", path)

	cmd := t.scanCombinedOutput("-mode", "binary", "-format", "openvex", path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		err = errors.New(outputStr)
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

	var vulns []Vulnerability
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

func isModuleNotFound(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "no matching versions for query") ||
		strings.Contains(output, "not found") ||
		strings.Contains(output, "unknown revision")
}
