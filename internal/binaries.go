package internal

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
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
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+
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

	doctorTemplate = `{{- range .DiagsWithIssues -}}
🛠️  {{ .Name }}
{{- if .HasIssues }}
    {{- if .NotInPath }}
    ❗ not in PATH
    {{- end }}
    {{- if .DuplicatesInPath }}
    ❗ duplicated in PATH:
        {{- range .DuplicatesInPath }}
        • {{ . }}
        {{- end }}
    {{- end }}
    {{- if ne .GoVersion.Actual .GoVersion.Expected }}
    ❗ go version mismatch: expected {{ .GoVersion.Expected }}, actual {{ .GoVersion.Actual }}
    {{- end }}
    {{- if ne .Platform.Actual .Platform.Expected }}
    ❗ platform mismatch: expected {{ .Platform.Expected }}, actual {{ .Platform.Actual }}
    {{- end }}
    {{- if .IsPseudoVersion }}
    ❗ pseudo-version
    {{- end }}
    {{- if .NotBuiltWithGoModules }}
    ❗ built without Go modules (GO111MODULE=off)
    {{- end }}
    {{- if .IsOrphaned }}
    ❗ orphaned: unknown source, likely built locally
    {{- end }}
    {{- if .Retracted }}
    ❗ retracted module version: {{ .Retracted }}
    {{- end }}
    {{- if .Deprecated }}
    ❗ deprecated module: {{ .Deprecated }}
    {{- end }}
    {{- if .Vulnerabilities }}
    ❗ found {{ len .Vulnerabilities }} {{if gt (len .Vulnerabilities) 1}}vulnerabilities{{else}}vulnerability{{end}}:
        {{- range .Vulnerabilities }}
        • {{ .ID }} ({{ .URL }})
        {{- end }}
    {{- end }}
{{- else }}
    ✅ no issues
{{- end }}
{{end -}}
{{- if gt .WithIssues 0 }}
{{""}}
{{- end -}}
{{ .Total }} binaries checked, {{ .WithIssues }} with issues
`

	GOOSEnvVar   = "GOOS"
	GOARCHEnvVar = "GOARCH"
)

var (
	ErrBinaryNotFound              = errors.New("binary not found")
	ErrBinaryBuiltWithoutGoModules = errors.New("binary built without go modules")
	ErrInvalidModuleVersion        = errors.New("invalid module version")
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

type BinDiagnostic struct {
	Name             string
	NotInPath        bool
	DuplicatesInPath []string
	GoVersion        struct {
		Actual   string
		Expected string
	}
	Platform struct {
		Actual   string
		Expected string
	}
	IsPseudoVersion       bool
	NotBuiltWithGoModules bool
	IsOrphaned            bool
	Retracted             string
	Deprecated            string
	Vulnerabilities       []Vulnerability
}

func (d BinDiagnostic) HasIssues() bool {
	return d.NotInPath ||
		len(d.DuplicatesInPath) > 0 ||
		d.GoVersion.Actual != d.GoVersion.Expected ||
		d.Platform.Actual != d.Platform.Expected ||
		d.IsPseudoVersion ||
		d.NotBuiltWithGoModules ||
		d.IsOrphaned ||
		d.Retracted != "" ||
		d.Deprecated != "" ||
		len(d.Vulnerabilities) > 0
}

func DiagnoseBinaries() error {
	binFullPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	bins, err := listBinaryFullPaths(binFullPath)
	if err != nil {
		return err
	}

	var (
		mutex sync.Mutex
		diags []BinDiagnostic
		grp   = new(errgroup.Group)
	)

	for _, bin := range bins {
		grp.Go(func() error {
			diag, diagErr := diagnoseBinary(bin)
			if diagErr != nil {
				return diagErr
			}

			mutex.Lock()
			diags = append(diags, diag)
			mutex.Unlock()

			return nil
		})
	}

	waitErr := grp.Wait()

	if err = printBinaryDiagnostics(diags); err != nil {
		return err
	}

	return waitErr
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
		grp      = new(errgroup.Group)
	)

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

	if err = printOutdatedBinaries(outdated); err != nil {
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
		case GOOSEnvVar:
			data.OS = v
		case GOARCHEnvVar:
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
	if err = tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
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
		case GOOSEnvVar:
			goOS = s.Value
		case GOARCHEnvVar:
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
		fullPath := filepath.Join(dir, entry.Name())
		if isBinary(fullPath) {
			binaries = append(binaries, fullPath)
		}
	}

	return binaries, nil
}

func nextMajorVersion(version string) (string, error) {
	logger := slog.Default().With("version", version)

	if !semver.IsValid(version) {
		err := ErrInvalidModuleVersion
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
	var (
		latestMajorVersion = version
		pkg                = module
		major              = semver.Major(version)
	)

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
	sort.Slice(binInfos, func(i, j int) bool {
		return binInfos[i].Name < binInfos[j].Name
	})

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

func diagnoseBinary(fullPath string) (BinDiagnostic, error) {
	binaryName := filepath.Base(fullPath)
	logger := slog.Default().With("binary", binaryName)

	buildInfo, err := buildinfo.ReadFile(fullPath)
	if err != nil {
		logger.Error("error reading binary build info", "err", err)
		return BinDiagnostic{}, err
	}

	binPlatform := getBinaryPlatform(buildInfo)
	runtimePlatform := runtime.GOOS + "/" + runtime.GOARCH

	diagnostic := BinDiagnostic{
		Name:                  binaryName,
		DuplicatesInPath:      checkBinaryDuplicatesInPath(binaryName),
		IsPseudoVersion:       module.IsPseudoVersion(buildInfo.Main.Version),
		NotBuiltWithGoModules: buildInfo.Main.Path == "",
		IsOrphaned:            buildInfo.Main.Sum == "",
	}

	diagnostic.GoVersion.Actual = buildInfo.GoVersion
	diagnostic.GoVersion.Expected = runtime.Version()
	diagnostic.Platform.Actual = binPlatform
	diagnostic.Platform.Expected = runtimePlatform

	_, err = exec.LookPath(binaryName)
	diagnostic.NotInPath = err != nil

	if buildInfo.Main.Sum != "" {
		retracted, deprecated, modErr := diagnoseGoModFile(buildInfo.Main.Path, buildInfo.Main.Version)
		if modErr != nil {
			return diagnostic, modErr
		}

		diagnostic.Retracted = retracted
		diagnostic.Deprecated = deprecated
	}

	diagnostic.Vulnerabilities, err = GoVulnCheck(fullPath)
	if err != nil {
		return diagnostic, err
	}

	return diagnostic, nil
}

func checkBinaryDuplicatesInPath(binaryName string) []string {
	var (
		seen       = make(map[string]struct{})
		duplicates []string
	)

	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		fullPath := filepath.Join(dir, binaryName)
		if isBinary(fullPath) {
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

func diagnoseGoModFile(module, version string) (string, string, error) {
	logger := slog.Default().With("module", module)

	file, err := GoModDownload(module, "latest")
	if err != nil {
		logger.Error("error downloading go.mod", "err", err)
		return "", "", err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		logger.Error("error reading go.mod", "err", err)
		return "", "", err
	}

	modFile, err := modfile.Parse("go.mod", bytes, nil)
	if err != nil {
		logger.Error("error parsing go.mod", "err", err)
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

	return retracted, deprecated, err
}

func printBinaryDiagnostics(diags []BinDiagnostic) error {
	var diagWithIssues []BinDiagnostic
	for _, d := range diags {
		if d.HasIssues() {
			diagWithIssues = append(diagWithIssues, d)
		}
	}

	sort.Slice(diagWithIssues, func(i, j int) bool {
		return diagWithIssues[i].Name < diagWithIssues[j].Name
	})

	data := struct {
		Total           int
		WithIssues      int
		DiagsWithIssues []BinDiagnostic
	}{
		Total:           len(diags),
		WithIssues:      len(diagWithIssues),
		DiagsWithIssues: diagWithIssues,
	}

	tmplParsed := template.Must(template.New("doctor").Parse(doctorTemplate))
	if err := tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func isBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}

	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}
