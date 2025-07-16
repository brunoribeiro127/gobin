package internal

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

	"golang.org/x/vuln/scan"
)

type Vulnerability struct {
	ID  string
	URL string
}

var ErrModuleNotFound = errors.New("module not found")

func GoGetLatestVersion(module string) (string, error) {
	logger := slog.Default().With("module", module)

	modLatest := fmt.Sprintf("%s@latest", module)
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", modLatest)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
		if strings.Contains(outputStr, "no matching versions for query") {
			return "", ErrModuleNotFound
		}

		err = errors.New(outputStr)
		logger.Error("error getting latest version for module", "err", err)
		return "", err
	}

	return outputStr, nil
}

func GoModDownload(module string, version string) (io.ReadCloser, error) {
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

	return os.Open(res.GoMod)
}

func GoInstall(pkg string, version string) error {
	logger := slog.Default().With("package", pkg)

	pkgVersion := fmt.Sprintf("%s@%s", pkg, version)
	cmd := exec.Command("go", "install", pkgVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		logger.Error("error installing binary", "err", err)
	}

	return nil
}

func GoVulnCheck(path string) ([]Vulnerability, error) {
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
