package gobin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/sync/errgroup"

	"github.com/brunoribeiro127/gobin/internal/binaries"
)

const (
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

	infoTemplate = `Path          {{.FullPath}}
Package       {{.PackagePath}}
Module        {{.ModulePath}}@{{.ModuleVersion}}
Module Sum    {{if .ModuleSum}}{{.ModuleSum}}{{else}}<none>{{end}}
{{- if .CommitRevision}}
Commit        {{.CommitRevision}}{{if .CommitTime}} ({{.CommitTime}}){{end}}
{{- end}}
Go Version    {{.GoVersion}}
Platform      {{.OS}}/{{.Arch}}/{{.Feature}}
Env Vars      {{range $index, $env := .EnvVars}}{{if eq $index 0}}{{$env}}{{else}}
              {{$env}}{{end}}{{end}}
`

	listTemplate = `{{printf "%-*s" $.NameWidth "Name"}} → {{printf "%-*s" $.ModulePathWidth "Module"}} @ {{printf "%-*s" $.ModuleVersionWidth "Version"}}
{{repeat "-" (add $.NameWidth $.ModulePathWidth $.ModuleVersionWidth 6)}}
{{range .Binaries -}}
{{printf "%-*s" $.NameWidth .Name}} → {{printf "%-*s" $.ModulePathWidth .ModulePath}} @ {{printf "%-*s" $.ModuleVersionWidth .ModuleVersion}}
{{end -}}
`

	outdatedTemplate = `{{printf "%-*s" $.NameWidth "Name"}} → {{printf "%-*s" $.ModulePathWidth "Module"}} @ {{printf "%-*s" $.ModuleVersionWidth "Current"}} ↑ {{printf "%-*s" $.LatestVersionWidth "Latest"}}
{{repeat "-" (add $.NameWidth $.ModulePathWidth $.ModuleVersionWidth $.LatestVersionWidth 9)}}
{{range .Binaries -}}
{{printf "%-*s" $.NameWidth .Name}} → {{printf "%-*s" $.ModulePathWidth .ModulePath}} @ {{color (printf "%-*s" $.ModuleVersionWidth .ModuleVersion) "red"}} ↑ {{color (printf "%-*s" $.LatestVersionWidth .LatestVersion) "green"}}
{{end -}}
`
)

func DiagnoseBinaries(parallelism int) error {
	binFullPath, err := binaries.GetBinFullPath()
	if err != nil {
		return err
	}

	bins, err := binaries.ListBinariesFullPaths(binFullPath)
	if err != nil {
		return err
	}

	var (
		mutex sync.Mutex
		diags []binaries.BinaryDiagnostic
		grp   = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			diag, diagErr := binaries.DiagnoseBinary(bin)
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
	binInfos, err := binaries.GetAllBinaryInfos()
	if err != nil {
		return err
	}

	return printInstalledBinaries(binInfos)
}

func ListOutdatedBinaries(checkMajor bool, parallelism int) error {
	binInfos, err := binaries.GetAllBinaryInfos()
	if err != nil {
		return err
	}

	var (
		mutex    sync.Mutex
		outdated []binaries.BinaryUpgradeInfo
		grp      = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, info := range binInfos {
		grp.Go(func() error {
			binUpInfo, infoErr := binaries.GetBinaryUpgradeInfo(info, checkMajor)
			if infoErr != nil {
				return infoErr
			}

			if binUpInfo.IsUpgradeAvailable {
				mutex.Lock()
				outdated = append(outdated, binUpInfo)
				mutex.Unlock()
			}

			return nil
		})
	}

	waitErr := grp.Wait()

	if len(outdated) == 0 {
		fmt.Fprintln(os.Stdout, "✅ All binaries are up to date")
		return waitErr
	}

	if err = printOutdatedBinaries(outdated); err != nil {
		return err
	}

	return waitErr
}

func PrintBinaryInfo(binary string) error {
	binPath, err := binaries.GetBinFullPath()
	if err != nil {
		return err
	}

	binInfo, err := binaries.GetBinaryInfo(filepath.Join(binPath, binary))
	if err != nil {
		if errors.Is(err, binaries.ErrBinaryNotFound) {
			fmt.Fprintf(os.Stderr, "❌ binary %q not found\n", binary)
		}

		return err
	}

	tmplParsed := template.Must(template.New("info").Parse(infoTemplate))
	if err = tmplParsed.Execute(os.Stdout, binInfo); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func PrintShortVersion(path string) error {
	binInfo, err := binaries.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, binInfo.ModuleVersion)

	return nil
}

func PrintVersion(path string) error {
	binInfo, err := binaries.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stdout,
		"%s (%s %s/%s)\n",
		binInfo.ModuleVersion,
		binInfo.GoVersion,
		binInfo.OS,
		binInfo.Arch,
	)

	return nil
}

func UninstallBinary(binary string) error {
	binPath, err := binaries.GetBinFullPath()
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(binPath, binary))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "❌ binary %q not found\n", binary)
		return err
	} else if err != nil {
		slog.Default().Error("failed to remove binary", "binary", binary, "err", err)
		return err
	}

	return nil
}

func UpgradeAllBinaries(majorUpgrade bool, parallelism int) error {
	binFullPath, err := binaries.GetBinFullPath()
	if err != nil {
		return err
	}

	bins, err := binaries.ListBinariesFullPaths(binFullPath)
	if err != nil {
		return err
	}

	grp := new(errgroup.Group)
	grp.SetLimit(parallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			return UpgradeBinary(filepath.Base(bin), majorUpgrade)
		})
	}

	return grp.Wait()
}

func UpgradeBinaries(majorUpgrade bool, parallelism int, bins ...string) error {
	binFullPath, err := binaries.GetBinFullPath()
	if err != nil {
		return err
	}

	grp := new(errgroup.Group)
	grp.SetLimit(parallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			return UpgradeBinary(filepath.Join(binFullPath, bin), majorUpgrade)
		})
	}

	return grp.Wait()
}

func UpgradeBinary(binFullPath string, majorUpgrade bool) error {
	info, err := binaries.GetBinaryInfo(binFullPath)
	if err != nil {
		if errors.Is(err, binaries.ErrBinaryNotFound) {
			fmt.Fprintf(os.Stderr, "❌ binary %q not found\n", filepath.Base(binFullPath))
		}

		return err
	}

	binUpInfo, err := binaries.GetBinaryUpgradeInfo(info, majorUpgrade)
	if err != nil {
		return err
	}

	if binUpInfo.IsUpgradeAvailable {
		return binaries.InstallBinary(binUpInfo)
	}

	return nil
}

func add(args ...int) int {
	sum := 0
	for _, v := range args {
		sum += v
	}
	return sum
}

func printBinaryDiagnostics(diags []binaries.BinaryDiagnostic) error {
	var diagWithIssues []binaries.BinaryDiagnostic
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
		DiagsWithIssues []binaries.BinaryDiagnostic
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

func printInstalledBinaries(binInfos []binaries.BinaryInfo) error {
	getColumnMaxWidth := func(
		header string,
		binaries []binaries.BinaryInfo,
		f func(binaries.BinaryInfo) string,
	) int {
		maxWidth := len(header)

		for _, bin := range binaries {
			width := len(f(bin))
			if width > maxWidth {
				maxWidth = width
			}
		}

		return maxWidth
	}

	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin binaries.BinaryInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin binaries.BinaryInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Version",
		binInfos,
		func(bin binaries.BinaryInfo) string { return bin.ModuleVersion },
	)

	data := struct {
		Binaries           []binaries.BinaryInfo
		NameWidth          int
		ModulePathWidth    int
		ModuleVersionWidth int
	}{
		Binaries:           binInfos,
		NameWidth:          maxNameWidth,
		ModulePathWidth:    maxModulePathWidth,
		ModuleVersionWidth: maxModuleVersionWidth,
	}

	tmplParsed := template.Must(template.New("list").Funcs(template.FuncMap{
		"add":    add,
		"repeat": strings.Repeat,
	}).Parse(listTemplate))

	if err := tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func printOutdatedBinaries(binInfos []binaries.BinaryUpgradeInfo) error {
	sort.Slice(binInfos, func(i, j int) bool {
		return binInfos[i].Name < binInfos[j].Name
	})

	getColumnMaxWidth := func(
		header string,
		binaries []binaries.BinaryUpgradeInfo,
		f func(binaries.BinaryUpgradeInfo) string,
	) int {
		maxWidth := len(header)

		for _, bin := range binaries {
			width := len(f(bin))
			if width > maxWidth {
				maxWidth = width
			}
		}

		return maxWidth
	}

	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin binaries.BinaryUpgradeInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin binaries.BinaryUpgradeInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Current",
		binInfos,
		func(bin binaries.BinaryUpgradeInfo) string { return bin.ModuleVersion },
	)
	maxLatestVersionWidth := getColumnMaxWidth(
		"Latest",
		binInfos,
		func(bin binaries.BinaryUpgradeInfo) string { return bin.LatestVersion },
	)

	data := struct {
		Binaries           []binaries.BinaryUpgradeInfo
		NameWidth          int
		ModulePathWidth    int
		ModuleVersionWidth int
		LatestVersionWidth int
	}{
		Binaries:           binInfos,
		NameWidth:          maxNameWidth,
		ModulePathWidth:    maxModulePathWidth,
		ModuleVersionWidth: maxModuleVersionWidth,
		LatestVersionWidth: maxLatestVersionWidth,
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
		"add":    add,
		"color":  colorize,
		"repeat": strings.Repeat,
	}).Parse(outdatedTemplate))

	if err := tmplParsed.Execute(os.Stdout, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}
