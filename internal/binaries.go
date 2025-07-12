package internal

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/mod/semver"
)

const (
	BinaryListTemplate = `+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
| {{printf "%-*s" .BinaryNameWidth "Name"}} | {{printf "%-*s" .ModulePathWidth "Module Path"}} | {{printf "%-*s" .ModuleVersionWidth "Version"}} | {{printf "%-*s" .ModuleLatestVersionWidth "Latest Version"}} |
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
{{- range .Binaries }}
| {{printf "%-*s" $.BinaryNameWidth .Name}} | {{printf "%-*s" $.ModulePathWidth .ModulePath}} | {{if .NeedsUpgrade}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "red"}}{{else}}{{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "green"}}{{end}} | {{if .NeedsUpgrade}}{{color (printf "%-*s" $.ModuleLatestVersionWidth (print "↑ " .ModuleLatestVersion)) "green"}}{{else}}{{printf "%-*s" $.ModuleLatestVersionWidth .ModuleLatestVersion}}{{end}} |
{{- end }}
+{{repeat "-" .BinaryNameHeaderWidth}}+{{repeat "-" .ModulePathHeaderWidth}}+{{repeat "-" .ModuleVersionHeaderWidth}}+{{repeat "-" .ModuleLatestVersionHeaderWidth}}+
`

	VersionTemplate = `Main Path:       {{.MainPath}}
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

func ListBinaries(checkMajor bool) error {
	binInfos, err := getAllBinInfos(checkMajor)
	if err != nil {
		return err
	}

	return printTabularBinInfos(binInfos)
}

func UpgradeAllBinaries(majorUpgrade bool) error {
	binInfos, err := getAllBinInfos(majorUpgrade)
	if err != nil {
		return err
	}

	for _, info := range binInfos {
		if info.NeedsUpgrade {
			if err = installGoBin(info); err != nil {
				return err
			}
		}
	}

	return nil
}

func UpgradeBinary(binary string, majorUpgrade bool) error {
	binPath, err := getBinFullPath()
	if err != nil {
		return err
	}

	info, err := getBinInfo(filepath.Join(binPath, binary), majorUpgrade)
	if err != nil {
		return err
	}

	if info.NeedsUpgrade {
		if err = installGoBin(info); err != nil {
			return err
		}
	}

	return nil
}

func PrintShortVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Println("no build info available")
		return
	}

	fmt.Fprintln(os.Stdout, info.Main.Version)
}

func PrintVersion() error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Println("no build info available")
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

	tmplParsed := template.Must(template.New("version").Parse(VersionTemplate))
	err := tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		log.Printf("error executing template: %v\n", err)
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
		log.Printf("error getting user home directory: %v\n", err)
		return "", err
	}

	return filepath.Join(home, "go", "bin"), nil
}

func listBinaryFullPaths(dir string) ([]string, error) {
	var binaries []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("error while reading binaries directory: %v\n", err)
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
				log.Printf("error while reading file info '%s': %v\n", dir, infoErr)
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
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("https://proxy.golang.org/%s/@latest", module),
		nil,
	)
	if err != nil {
		log.Printf("error creating request for module '%s': %v\n", module, err)
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error fetching latest version for module '%s': %v\n", module, err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("error reading response body: %v\n", readErr)
			return "", readErr
		}

		if resp.StatusCode == http.StatusNotFound {
			return "", ErrNotFound
		}

		err = fmt.Errorf("unexpected response [code=%s, body=%s]", resp.Status, string(bytes))
		log.Printf("error fetching latest version for module '%s': %v\n", module, err)
		return "", err
	}

	var response struct {
		Version string `json:"Version"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("error decoding response body: %v\n", err)
		return "", err
	}

	return response.Version, nil
}

func nextMajorVersion(version string) (string, error) {
	if !semver.IsValid(version) {
		err := fmt.Errorf("invalid module version '%s'", version)
		log.Println(err)
		return "", err
	}

	major := semver.Major(version)
	if major == "v0" || major == "v1" {
		return "v2", nil
	}

	majorNumStr := strings.TrimPrefix(major, "v")
	majorNum, err := strconv.Atoi(majorNumStr)
	if err != nil {
		log.Printf("error parsing major version number '%s': %v\n", majorNumStr, err)
		return "", err
	}

	return fmt.Sprintf("v%d", majorNum+1), nil
}

func checkModuleMajorUpgrade(module, version string) (string, error) {
	latestMajorVersion := version

	for {
		nextMajorVersion, err := nextMajorVersion(latestMajorVersion)
		if err != nil {
			return "", err
		}

		majorVersion, err := fetchModuleLatestVersion(fmt.Sprintf("%s/%s", module, nextMajorVersion))
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

func getBinInfo(fullPath string, checkMajorUpgrade bool) (BinInfo, error) {
	info, err := buildinfo.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("binary not found '%s': %v\n", fullPath, err)
			return BinInfo{}, ErrNotFound
		}

		log.Printf("error reading binary build info '%s': %v\n", fullPath, err)
		return BinInfo{}, err
	}

	latestVersion, err := fetchModuleLatestVersion(info.Main.Path)
	if err != nil {
		return BinInfo{}, err
	}

	if checkMajorUpgrade {
		latestVersion, err = checkModuleMajorUpgrade(info.Main.Path, latestVersion)
		if err != nil {
			return BinInfo{}, err
		}
	}

	return BinInfo{
		Name:                filepath.Base(fullPath),
		FullPath:            fullPath,
		PackagePath:         info.Path,
		ModulePath:          info.Main.Path,
		ModuleVersion:       info.Main.Version,
		ModuleLatestVersion: latestVersion,
		NeedsUpgrade:        semver.Compare(info.Main.Version, latestVersion) < 0,
	}, nil
}

func getAllBinInfos(checkMajorUpgrade bool) ([]BinInfo, error) {
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
		info, infoErr := getBinInfo(bin, checkMajorUpgrade)
		if infoErr != nil {
			continue
		}

		binInfos = append(binInfos, info)
	}

	return binInfos, nil
}

func printTabularBinInfos(binInfos []BinInfo) error {
	getMaxWidth := func(header string, binaries []BinInfo, f func(BinInfo) string) int {
		maxWidth := len(header)
		for _, bin := range binaries {
			width := len(f(bin))
			if width > maxWidth {
				maxWidth = width
			}
		}
		return maxWidth
	}

	maxBinaryNameWidth := getMaxWidth(
		"Name",
		binInfos,
		func(bin BinInfo) string { return bin.Name },
	)
	maxModulePathWidth := getMaxWidth(
		"Module",
		binInfos,
		func(bin BinInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getMaxWidth(
		"Version",
		binInfos,
		func(bin BinInfo) string { return bin.ModuleVersion },
	)
	maxModuleLatestVersionWidth := getMaxWidth(
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

	tmplParsed := template.Must(template.New("table").Funcs(template.FuncMap{
		"repeat": strings.Repeat,
		"color":  colorize,
	}).Parse(BinaryListTemplate))

	err := tmplParsed.Execute(os.Stdout, data)
	if err != nil {
		log.Printf("error executing template: %v\n", err)
	}

	return err
}

func installGoBin(info BinInfo) error {
	if !semver.IsValid(info.ModuleLatestVersion) {
		err := fmt.Errorf("invalid module version '%s'", info.ModuleLatestVersion)
		log.Println(err)
		return err
	}

	var pkg string
	major := semver.Major(info.ModuleLatestVersion)
	switch major {
	case "v0", "v1":
		pkg = fmt.Sprintf("%s@%s", info.PackagePath, info.ModuleLatestVersion)
	default:
		pkg = fmt.Sprintf(
			"%s/%s%s@%s",
			info.ModulePath,
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
		log.Printf("error installing binaries: %v\n", err)
	}

	return err
}
