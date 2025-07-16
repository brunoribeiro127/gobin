package internal

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
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

	outdatedTemplate = `+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .LatestVersionHeaderWidth}}+
| {{printf "%-*s" .BinaryNameWidth "Name"}} | {{printf "%-*s" .ModulePathWidth "Module Path"}} | {{printf "%-*s" .ModuleVersionWidth "Version"}} | {{printf "%-*s" .LatestVersionWidth "Latest Version"}} |
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .LatestVersionHeaderWidth}}+
{{- range .Binaries }}
| {{printf "%-*s" $.BinaryNameWidth .Name}} | {{printf "%-*s" $.ModulePathWidth .ModulePath}} | {{if .IsUpgradeAvailable}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "red"}}{{else}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "green"}}{{end}} | {{if .IsUpgradeAvailable}}{{color (printf "%-*s" $.LatestVersionWidth (print "↑ " .LatestVersion)) "green"}}{{else}}{{printf "%-*s" $.LatestVersionWidth .LatestVersion}}{{end}} |
{{- end }}
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .LatestVersionHeaderWidth}}+
`

	infoTemplate = `Path:          {{.FullPath}}
Package:       {{.PackagePath}}
Module:        {{.ModulePath}}@{{.ModuleVersion}}
Module Sum:    {{if .ModuleSum}}{{.ModuleSum}}{{else}}<none>{{end}}
{{- if .CommitRevision}}
Commit:        {{.CommitRevision}}{{if .CommitTime}} ({{.CommitTime}}){{end}}
{{- end}}
Go Version:    {{.GoVersion}}
Platform:      {{.OS}}/{{.Arch}}/{{.Feature}}
Env Vars:      {{range $index, $env := .EnvVars}}{{if eq $index 0}}{{$env}}{{else}}
               {{$env}}{{end}}{{end}}
`
)

var (
	ErrBinaryNotFound              = errors.New("binary not found")
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")
)

type BinInfo struct {
	Name               string
	FullPath           string
	PackagePath        string
	ModulePath         string
	ModuleVersion      string
	ModuleSum          string
	GoVersion          string
	Settings           map[string]string
	LatestVersion      string
	IsUpgradeAvailable bool
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

	for _, info := range binInfos {
		grp.Go(func() error {
			if enrErr := enrichBinInfoMinorVersionUpgrade(&info); enrErr != nil {
				return enrErr
			}

			if checkMajor {
				if enrErr := enrichBinInfoMajorVersionUpgrade(&info); enrErr != nil {
					return enrErr
				}
			}

			if info.IsUpgradeAvailable {
				mutex.Lock()
				outdated = append(outdated, info)
				mutex.Unlock()
			}

			return nil
		})
	}

	waitErr := grp.Wait()

	if len(outdated) == 0 {
		fmt.Fprintln(os.Stdout, "✅ All binaries are up to date.")
		return waitErr
	}

	if err := printOutdatedBinaries(outdated); err != nil {
		return err
	}

	return waitErr
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

	for _, bin := range bins {
		grp.Go(func() error {
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

	if info.IsUpgradeAvailable {
		return installGoBin(info)
	}

	return nil
}

func UninstallBinary(binary string) error {
	binPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	if err = os.Remove(filepath.Join(binPath, binary)); err != nil {
		slog.Default().Error("failed to remove binary", "binary", binary, "err", err)
		return err
	}

	return nil
}

func PrintBinaryInfo(binary string) error {
	binPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	info, err := getBinInfo(filepath.Join(binPath, binary))
	if err != nil {
		return err
	}

	data := struct {
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
	}{
		FullPath:      info.FullPath,
		PackagePath:   info.PackagePath,
		ModulePath:    info.ModulePath,
		ModuleVersion: info.ModuleVersion,
		ModuleSum:     info.ModuleSum,
		GoVersion:     info.GoVersion,
	}

	for k, v := range info.Settings {
		switch k {
		case "vcs.revision":
			data.CommitRevision = v
		case "vcs.time":
			data.CommitTime = v
		case "GOOS":
			data.OS = v
		case "GOARCH":
			data.Arch = v
		default:
			if strings.HasPrefix(k, "GO") {
				data.Feature = v
			}
			if strings.HasPrefix(k, "CGO_") && v != "" {
				data.EnvVars = append(data.EnvVars, k+"="+v)
			}
		}
	}

	tmplParsed := template.Must(template.New("info").Parse(infoTemplate))
	err = tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		slog.Default().Error("error executing template", "err", err)
	}

	return err
}

func PrintShortVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Default().Error("no build info available")
		return
	}

	fmt.Fprintln(os.Stdout, info.Main.Version)
}

func PrintVersion() {
	logger := slog.Default()

	info, ok := debug.ReadBuildInfo()
	if !ok {
		logger.Error("no build info available")
		return
	}

	var goOS, goArch string
	for _, s := range info.Settings {
		switch s.Key {
		case "GOOS":
			goOS = s.Value
		case "GOARCH":
			goArch = s.Value
		}
	}

	fmt.Fprintf(os.Stdout, "%s (%s %s/%s)\n", info.Main.Version, info.GoVersion, goOS, goArch)
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

		majorVersion, err := GoGetLatestVersion(fmt.Sprintf("%s/%s", pkg, nextMajorVersion))
		if err != nil {
			if errors.Is(err, ErrModuleNotFound) {
				break
			}

			return "", err
		}

		latestMajorVersion = majorVersion
	}

	return latestMajorVersion, nil
}

func getBinInfo(fullPath string) (BinInfo, error) {
	logger := slog.Default().With("path", fullPath)

	info, err := buildinfo.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Error("binary not found", "err", err)
			return BinInfo{}, ErrBinaryNotFound
		}

		logger.Error("error reading binary build info", "err", err)
		return BinInfo{}, err
	}

	if info.Main.Path == "" {
		logger.Error(ErrBinaryBuiltWithoutGoModules.Error())
		return BinInfo{}, ErrBinaryBuiltWithoutGoModules
	}

	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}

	return BinInfo{
		Name:               filepath.Base(fullPath),
		FullPath:           fullPath,
		PackagePath:        info.Path,
		ModulePath:         info.Main.Path,
		ModuleVersion:      info.Main.Version,
		ModuleSum:          info.Main.Sum,
		GoVersion:          info.GoVersion,
		Settings:           settings,
		LatestVersion:      info.Main.Version,
		IsUpgradeAvailable: false,
	}, nil
}

func enrichBinInfoMinorVersionUpgrade(info *BinInfo) error {
	version, err := GoGetLatestVersion(info.ModulePath)
	if err != nil {
		return err
	}

	info.LatestVersion = version
	info.IsUpgradeAvailable = semver.Compare(info.ModuleVersion, version) < 0

	return nil
}

func enrichBinInfoMajorVersionUpgrade(info *BinInfo) error {
	version, err := checkModuleMajorUpgrade(info.ModulePath, info.LatestVersion)
	if err != nil {
		return err
	}

	info.LatestVersion = version
	info.IsUpgradeAvailable = semver.Compare(info.ModuleVersion, version) < 0

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

	if err := tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
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
	maxLatestVersionWidth := getColumnMaxWidth(
		"Latest Version",
		binInfos,
		func(bin BinInfo) string { return bin.LatestVersion },
	)

	data := struct {
		Binaries                 []BinInfo
		BinaryNameWidth          int
		BinaryNameHeaderWidth    int
		ModulePathWidth          int
		ModulePathHeaderWidth    int
		ModuleVersionWidth       int
		ModuleVersionHeaderWidth int
		LatestVersionWidth       int
		LatestVersionHeaderWidth int
	}{
		Binaries:                 binInfos,
		BinaryNameWidth:          maxBinaryNameWidth,
		BinaryNameHeaderWidth:    maxBinaryNameWidth + 2,
		ModulePathWidth:          maxModulePathWidth,
		ModulePathHeaderWidth:    maxModulePathWidth + 2,
		ModuleVersionWidth:       maxModuleVersionWidth,
		ModuleVersionHeaderWidth: maxModuleVersionWidth + 2,
		LatestVersionWidth:       maxLatestVersionWidth,
		LatestVersionHeaderWidth: maxLatestVersionWidth + 2,
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

	if err := tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func installGoBin(info BinInfo) error {
	pkg := info.PackagePath
	major := semver.Major(info.LatestVersion)
	if major != "v0" && major != "v1" {
		pkg = fmt.Sprintf(
			"%s/%s%s",
			stripVersionSuffix(info.ModulePath),
			major,
			strings.TrimPrefix(info.PackagePath, info.ModulePath),
		)
	}

	return GoInstall(pkg, info.LatestVersion)
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
