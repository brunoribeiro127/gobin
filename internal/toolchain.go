package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

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
