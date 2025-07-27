package toolchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/vuln/scan"
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

var (
	ErrModuleNotFound           = errors.New("module not found")
	ErrModuleInfoNotAvailable   = errors.New("module info not available")
	ErrModuleOriginNotAvailable = errors.New("module origin not available")
)

func GetLatestModuleVersion(module string) (string, string, error) {
	logger := slog.Default().With("module", module)

	modLatest := fmt.Sprintf("%s@latest", module)
	cmd := exec.Command("go", "list", "-m", "-json", modLatest)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
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

	file, err := os.Open(res.GoMod)
	if err != nil {
		logger.Error("error downloading go mod file", "err", err)
		return "", "", err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
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

func GetModuleFile(module string, version string) (*modfile.File, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := exec.Command("go", "mod", "download", "-json", modVersion)
	cmd.Env = os.Environ()

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

func GetModuleOrigin(module, version string) (*ModuleOrigin, error) {
	logger := slog.Default().With("module", module, "version", version)

	modVersion := fmt.Sprintf("%s@%s", module, version)
	cmd := exec.Command("go", "mod", "download", "-json", modVersion)
	cmd.Env = os.Environ()

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

func Install(pkg string, version string) error {
	logger := slog.Default().With("package", pkg, "version", version)

	pkgVersion := fmt.Sprintf("%s@%s", pkg, version)
	cmd := exec.Command("go", "install", "-a", pkgVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		logger.Error("error installing binary", "err", err)
		return err
	}

	return nil
}

func VulnCheck(path string) ([]Vulnerability, error) {
	logger := slog.Default().With("path", path)

	var output bytes.Buffer
	cmd := scan.Command(context.Background(), "-mode", "binary", "-format", "openvex", path)
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		err = errors.New(output.String())
		logger.Error("error starting govulncheck command", "err", err)
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		err = errors.New(output.String())
		logger.Error("error waiting for govulncheck command", "err", err)
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

	if err := json.Unmarshal(output.Bytes(), &res); err != nil {
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
