package internal

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	listTemplate = `+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+
| {{printf "%-*s" .BinaryNameWidth "Name"}} | {{printf "%-*s" .ModulePathWidth "Module Path"}} | {{printf "%-*s" .ModuleVersionWidth "Version"}} |
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+
{{- range .Binaries }}
| {{printf "%-*s" $.BinaryNameWidth .Name}} | {{printf "%-*s" $.ModulePathWidth .ModulePath}} | {{printf "%-*s" $.ModuleVersionWidth .ModuleVersion}} |
{{- end }}
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}
`

	outdatedTemplate = `+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
| {{printf "%-*s" .BinaryNameWidth "Name"}} | {{printf "%-*s" .ModulePathWidth "Module Path"}} | {{printf "%-*s" .ModuleVersionWidth "Version"}} | {{printf "%-*s" .ModuleLatestVersionWidth "Latest Version"}} |
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
{{- range .Binaries }}
| {{printf "%-*s" $.BinaryNameWidth .Name}} | {{printf "%-*s" $.ModulePathWidth .ModulePath}} | {{if .NeedsUpgrade}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "red"}}{{else}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "green"}}{{end}} | {{if .NeedsUpgrade}}{{color (printf "%-*s" $.ModuleLatestVersionWidth (print "↑ " .ModuleLatestVersion)) "green"}}{{else}}{{printf "%-*s" $.ModuleLatestVersionWidth .ModuleLatestVersion}}{{end}} |
{{- end }}
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
`

	versionTemplate = `Main Path:       {{.MainPath}}
Main Version:    {{.MainVersion}}
Main Sum:        {{.MainSum}}
Go Version:      {{.GoVersion}}
OS/Arch/Feat:    {{.OS}}/{{.Arch}}/{{.Feature}}
Compiler:        {{.Compiler}}
Build Mode:      {{.BuildMode}}
Env Vars:        {{range $index, $env := .EnvVars}}{{if eq $index 0}}{{$env}}{{else}}
                 {{$env}}{{end}}{{end}}
Commit Hash:     {{.VCSRevision}}{{.VCSModified}}
Commit Time:     {{.VCSTime}}
`

	maxParallelism = 5
)

var (
	ErrNotFound = errors.New("not found")
)

type BinInfo struct {
	Name                string
	FullPath            string
	PackagePath         string
	ModulePath          string
	ModuleVersion       string
	ModuleLatestVersion string
	NeedsUpgrade        bool
}

func ListInstalledBinaries() error {
	binInfos, err := getAllBinInfos()
	if err != nil {
		return err
	}

	return printInstalledBinaries(binInfos)
}

func ListOutdatedBinaries(checkMajor bool) error {
	binInfos, err := getAllBinInfos()
	if err != nil {
		return err
	}

	var (
		mutex    sync.Mutex
		outdated []BinInfo
	)

	grp := new(errgroup.Group)
	sem := semaphore.NewWeighted(maxParallelism)

	for _, info := range binInfos {
		grp.Go(func() error {
			if acqErr := sem.Acquire(context.Background(), 1); acqErr != nil {
				slog.Default().Error("failed to acquire semaphore", "err", acqErr)
				return acqErr
			}
			defer sem.Release(1)

			if enrErr := enrichBinInfoMinorVersionUpgrade(&info); enrErr != nil {
				return enrErr
			}

			if checkMajor {
				if enrErr := enrichBinInfoMajorVersionUpgrade(&info); enrErr != nil {
					return enrErr
				}
			}

			if info.NeedsUpgrade {
				mutex.Lock()
				outdated = append(outdated, info)
				mutex.Unlock()
			}

			return nil
		})
	}

	if err = grp.Wait(); err != nil {
		return err
	}

	if len(outdated) == 0 {
		fmt.Fprintln(os.Stdout, "✅ All binaries are up to date.")
		return nil
	}

	return printOutdatedBinaries(outdated)
}

func UpgradeAllBinaries(majorUpgrade bool) error {
	binFullPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	bins, err := listBinaryFullPaths(binFullPath)
	if err != nil {
		return err
	}

	grp := new(errgroup.Group)
	sem := semaphore.NewWeighted(maxParallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			if acqErr := sem.Acquire(context.Background(), 1); acqErr != nil {
				slog.Default().Error("failed to acquire semaphore", "err", acqErr)
				return acqErr
			}
			defer sem.Release(1)

			return UpgradeBinary(filepath.Base(bin), majorUpgrade)
		})
	}

	return grp.Wait()
}

func UpgradeBinary(binary string, majorUpgrade bool) error {
	binPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	info, err := getBinInfo(filepath.Join(binPath, binary))
	if err != nil {
		return err
	}

	if err = enrichBinInfoMinorVersionUpgrade(&info); err != nil {
		return err
	}

	if majorUpgrade {
		if err = enrichBinInfoMajorVersionUpgrade(&info); err != nil {
			return err
		}
	}

	if info.NeedsUpgrade {
		return installGoBin(info)
	}

	return nil
}

func PrintShortVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Default().Error("no build info available")
		return
	}

	fmt.Fprintln(os.Stdout, info.Main.Version)
}

func PrintVersion() error {
	logger := slog.Default()

	info, ok := debug.ReadBuildInfo()
	if !ok {
		logger.Error("no build info available")
		return nil
	}

	data := struct {
		MainPath    string
		MainVersion string
		MainSum     string
		GoVersion   string
		BuildMode   string
		Compiler    string
		VCSRevision string
		VCSTime     string
		VCSModified string
		OS          string
		Arch        string
		Feature     string
		EnvVars     []string
	}{
		MainPath:    info.Main.Path,
		MainVersion: info.Main.Version,
		MainSum:     info.Main.Sum,
		GoVersion:   info.GoVersion,
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			data.VCSRevision = s.Value
		case "vcs.time":
			data.VCSTime = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				data.VCSModified = " (dirty)"
			}
		case "-buildmode":
			data.BuildMode = s.Value
		case "-compiler":
			data.Compiler = s.Value
		case "GOOS":
			data.OS = s.Value
		case "GOARCH":
			data.Arch = s.Value
		default:
			if strings.HasPrefix(s.Key, "GO") {
				data.Feature = s.Value
			}
			if strings.HasPrefix(s.Key, "CGO_") {
				data.EnvVars = append(data.EnvVars, s.Key+"="+s.Value)
			}
		}
	}

	tmplParsed := template.Must(template.New("version").Parse(versionTemplate))
	err := tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		logger.Error("error executing template", "err", err)
	}

	return err
}

func getBinFullPath() (string, error) {
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

func listBinaryFullPaths(dir string) ([]string, error) {
	logger := slog.Default().With("dir", dir)
	var binaries []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error("error while reading binaries directory", "err", err)
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		if runtime.GOOS == "windows" {
			if filepath.Ext(entry.Name()) == ".exe" {
				binaries = append(binaries, fullPath)
			}
		} else {
			info, infoErr := entry.Info()
			if infoErr != nil {
				logger.Error("error while reading file info", "file", entry.Name(), "err", infoErr)
				continue
			}

			mode := info.Mode()
			if mode.IsRegular() && mode&0111 != 0 {
				binaries = append(binaries, fullPath)
			}
		}
	}

	return binaries, nil
}

func fetchModuleLatestVersion(module string) (string, error) {
	logger := slog.Default().With("module", module)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("https://proxy.golang.org/%s/@latest", module),
		nil,
	)
	if err != nil {
		logger.Error("error creating request for module", "err", err)
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("error fetching latest version for module", "err", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			logger.Error("error reading response body", "err", readErr)
			return "", readErr
		}

		if resp.StatusCode == http.StatusNotFound {
			return "", ErrNotFound
		}

		err = fmt.Errorf("unexpected response [code=%s, body=%s]", resp.Status, string(bytes))
		logger.Error("error fetching latest version for module", "err", err)
		return "", err
	}

	var response struct {
		Version string `json:"Version"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logger.Error("error decoding response body", "err", err)
		return "", err
	}

	return response.Version, nil
}

func nextMajorVersion(version string) (string, error) {
	logger := slog.Default().With("version", version)

	if !semver.IsValid(version) {
		err := errors.New("invalid module version")
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

func checkModuleMajorUpgrade(module, version string) (string, error) {
	latestMajorVersion := version

	pkg := module
	major := semver.Major(version)
	if major != "v0" && major != "v1" {
		pkg = stripVersionSuffix(module)
	}

	for {
		nextMajorVersion, err := nextMajorVersion(latestMajorVersion)
		if err != nil {
			return "", err
		}

		majorVersion, err := fetchModuleLatestVersion(fmt.Sprintf("%s/%s", pkg, nextMajorVersion))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				break
			}

			return "", err
		}

		latestMajorVersion = majorVersion
	}

	return latestMajorVersion, nil
}

func getBinInfo(fullPath string) (BinInfo, error) {
	logger := slog.Default().With("full_path", fullPath)

	info, err := buildinfo.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Error("binary not found", "err", err)
			return BinInfo{}, ErrNotFound
		}

		logger.Error("error reading binary build info", "err", err)
		return BinInfo{}, err
	}

	return BinInfo{
		Name:          filepath.Base(fullPath),
		FullPath:      fullPath,
		PackagePath:   info.Path,
		ModulePath:    info.Main.Path,
		ModuleVersion: info.Main.Version,
	}, nil
}

func enrichBinInfoMinorVersionUpgrade(info *BinInfo) error {
	version, err := fetchModuleLatestVersion(info.ModulePath)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			info.ModuleLatestVersion = info.ModuleVersion
			return nil
		}

		return err
	}

	info.ModuleLatestVersion = version
	info.NeedsUpgrade = semver.Compare(info.ModuleVersion, version) < 0

	return nil
}

func enrichBinInfoMajorVersionUpgrade(info *BinInfo) error {
	version, err := checkModuleMajorUpgrade(info.ModulePath, info.ModuleLatestVersion)
	if err != nil {
		return err
	}

	info.ModuleLatestVersion = version
	info.NeedsUpgrade = semver.Compare(info.ModuleVersion, version) < 0

	return nil
}

func getAllBinInfos() ([]BinInfo, error) {
	binFullPath, err := getBinFullPath()
	if err != nil {
		return nil, err
	}

	bins, err := listBinaryFullPaths(binFullPath)
	if err != nil {
		return nil, err
	}

	binInfos := make([]BinInfo, 0, len(bins))
	for _, bin := range bins {
		info, infoErr := getBinInfo(bin)
		if infoErr == nil {
			binInfos = append(binInfos, info)
		}
	}

	return binInfos, nil
}

func printInstalledBinaries(binInfos []BinInfo) error {
	maxBinaryNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin BinInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin BinInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Version",
		binInfos,
		func(bin BinInfo) string { return bin.ModuleVersion },
	)

	data := struct {
		Binaries                 []BinInfo
		BinaryNameWidth          int
		BinaryNameHeaderWidth    int
		ModulePathWidth          int
		ModulePathHeaderWidth    int
		ModuleVersionWidth       int
		ModuleVersionHeaderWidth int
	}{
		Binaries:                 binInfos,
		BinaryNameWidth:          maxBinaryNameWidth,
		BinaryNameHeaderWidth:    maxBinaryNameWidth + 2,
		ModulePathWidth:          maxModulePathWidth,
		ModulePathHeaderWidth:    maxModulePathWidth + 2,
		ModuleVersionWidth:       maxModuleVersionWidth,
		ModuleVersionHeaderWidth: maxModuleVersionWidth + 2,
	}

	tmplParsed := template.Must(template.New("list").Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	}).Parse(listTemplate))

	err := tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		slog.Default().Error("error executing template", "err", err)
	}

	return err
}

func printOutdatedBinaries(binInfos []BinInfo) error {
	maxBinaryNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin BinInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin BinInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Version",
		binInfos,
		func(bin BinInfo) string { return bin.ModuleVersion },
	)
	maxModuleLatestVersionWidth := getColumnMaxWidth(
		"Latest Version",
		binInfos,
		func(bin BinInfo) string { return bin.ModuleLatestVersion },
	)

	data := struct {
		Binaries                       []BinInfo
		BinaryNameWidth                int
		BinaryNameHeaderWidth          int
		ModulePathWidth                int
		ModulePathHeaderWidth          int
		ModuleVersionWidth             int
		ModuleVersionHeaderWidth       int
		ModuleLatestVersionWidth       int
		ModuleLatestVersionHeaderWidth int
	}{
		Binaries:                       binInfos,
		BinaryNameWidth:                maxBinaryNameWidth,
		BinaryNameHeaderWidth:          maxBinaryNameWidth + 2,
		ModulePathWidth:                maxModulePathWidth,
		ModulePathHeaderWidth:          maxModulePathWidth + 2,
		ModuleVersionWidth:             maxModuleVersionWidth,
		ModuleVersionHeaderWidth:       maxModuleVersionWidth + 2,
		ModuleLatestVersionWidth:       maxModuleLatestVersionWidth,
		ModuleLatestVersionHeaderWidth: maxModuleLatestVersionWidth + 2,
	}

	colorize := func(s, color string) string {
		colors := map[string]string{
			"red":   "\033[31m",
			"green": "\033[32m",
			"reset": "\033[0m",
		}
		return colors[color] + s + colors["reset"]
	}

	tmplParsed := template.Must(template.New("outdated").Funcs(template.FuncMap{
		"repeat": strings.Repeat,
		"color":  colorize,
	}).Parse(outdatedTemplate))

	err := tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		slog.Default().Error("error executing template", "err", err)
	}

	return err
}

func installGoBin(info BinInfo) error {
	logger := slog.Default().With(
		"package", info.PackagePath,
		"module", info.ModulePath,
		"latest_version", info.ModuleLatestVersion,
	)

	pkg := fmt.Sprintf("%s@%s", info.PackagePath, info.ModuleLatestVersion)
	major := semver.Major(info.ModuleLatestVersion)
	if major != "v0" && major != "v1" {
		pkg = fmt.Sprintf(
			"%s/%s%s@%s",
			stripVersionSuffix(info.ModulePath),
			major,
			strings.TrimPrefix(info.PackagePath, info.ModulePath),
			info.ModuleLatestVersion,
		)
	}

	cmd := exec.Command("go", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Run()
	if err != nil {
		logger.Error("error installing binaries", "err", err)
	}

	return err
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

func getColumnMaxWidth(header string, binaries []BinInfo, f func(BinInfo) string) int {
	maxWidth := len(header)
	for _, bin := range binaries {
		width := len(f(bin))
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
