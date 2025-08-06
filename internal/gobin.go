package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/sync/errgroup"
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

type BinaryManager interface {
	DiagnoseBinary(ctx context.Context, path string) (BinaryDiagnostic, error)
	GetAllBinaryInfos() ([]BinaryInfo, error)
	GetBinaryInfo(path string) (BinaryInfo, error)
	GetBinaryRepository(ctx context.Context, binary string) (string, error)
	GetBinaryUpgradeInfo(ctx context.Context, info BinaryInfo, checkMajor bool) (BinaryUpgradeInfo, error)
	GetBinFullPath() (string, error)
	ListBinariesFullPaths(dir string) ([]string, error)
	UpgradeBinary(ctx context.Context, binFullPath string, majorUpgrade bool, rebuild bool) error
}

type Gobin struct {
	binaryManager BinaryManager
	execCmd       ExecCombinedOutputFunc
	stdErr        io.Writer
	stdOut        io.Writer
	system        System
}

func NewGobin(
	binaryManager BinaryManager,
	execCmd ExecCombinedOutputFunc,
	stdErr io.Writer,
	stdOut io.Writer,
	system System,
) *Gobin {
	return &Gobin{
		binaryManager: binaryManager,
		execCmd:       execCmd,
		stdErr:        stdErr,
		stdOut:        stdOut,
		system:        system,
	}
}

func (g *Gobin) DiagnoseBinaries(ctx context.Context, parallelism int) error {
	binFullPath, err := g.binaryManager.GetBinFullPath()
	if err != nil {
		return err
	}

	bins, err := g.binaryManager.ListBinariesFullPaths(binFullPath)
	if err != nil {
		return err
	}

	var (
		mutex sync.Mutex
		diags = make([]BinaryDiagnostic, 0, len(bins))
		grp   = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			diag, diagErr := g.binaryManager.DiagnoseBinary(ctx, bin)
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

	if err = g.printBinaryDiagnostics(diags); err != nil {
		return err
	}

	return waitErr
}

func (g *Gobin) ListInstalledBinaries() error {
	binInfos, err := g.binaryManager.GetAllBinaryInfos()
	if err != nil {
		return err
	}

	return g.printInstalledBinaries(binInfos)
}

func (g *Gobin) ListOutdatedBinaries(
	ctx context.Context,
	checkMajor bool,
	parallelism int,
) error {
	binInfos, err := g.binaryManager.GetAllBinaryInfos()
	if err != nil {
		return err
	}

	var (
		mutex    sync.Mutex
		outdated = make([]BinaryUpgradeInfo, 0, len(binInfos))
		grp      = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, info := range binInfos {
		grp.Go(func() error {
			binUpInfo, infoErr := g.binaryManager.GetBinaryUpgradeInfo(
				ctx, info, checkMajor,
			)
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
		fmt.Fprintln(g.stdOut, "✅ All binaries are up to date")
		return waitErr
	}

	if err = g.printOutdatedBinaries(outdated); err != nil {
		return err
	}

	return waitErr
}

func (g *Gobin) PrintBinaryInfo(binary string) error {
	binPath, err := g.binaryManager.GetBinFullPath()
	if err != nil {
		return err
	}

	binInfo, err := g.binaryManager.GetBinaryInfo(filepath.Join(binPath, binary))
	if err != nil {
		if errors.Is(err, ErrBinaryNotFound) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", binary)
		}

		return err
	}

	tmplParsed := template.Must(template.New("info").Parse(infoTemplate))
	if err = tmplParsed.Execute(g.stdOut, binInfo); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func (g *Gobin) PrintShortVersion(path string) error {
	binInfo, err := g.binaryManager.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintln(g.stdOut, binInfo.ModuleVersion)

	return nil
}

func (g *Gobin) PrintVersion(path string) error {
	binInfo, err := g.binaryManager.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		g.stdOut,
		"%s (%s %s/%s)\n",
		binInfo.ModuleVersion,
		binInfo.GoVersion,
		binInfo.OS,
		binInfo.Arch,
	)

	return nil
}

func (g *Gobin) ShowBinaryRepository(ctx context.Context, binary string, open bool) error {
	repoURL, err := g.binaryManager.GetBinaryRepository(ctx, binary)
	if err != nil {
		if errors.Is(err, ErrBinaryNotFound) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", binary)
		}

		return err
	}

	if open {
		return g.openURL(ctx, repoURL, g.execCmd)
	}

	fmt.Fprintln(g.stdOut, repoURL)
	return nil
}

func (g *Gobin) UninstallBinary(binary string) error {
	binPath, err := g.binaryManager.GetBinFullPath()
	if err != nil {
		return err
	}

	err = g.system.Remove(filepath.Join(binPath, binary))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", binary)
		return err
	} else if err != nil {
		slog.Default().Error("failed to remove binary", "binary", binary, "err", err)
		return err
	}

	return nil
}

func (g *Gobin) UpgradeBinaries(
	ctx context.Context,
	majorUpgrade bool,
	rebuild bool,
	parallelism int,
	bins ...string,
) error {
	binFullPath, err := g.binaryManager.GetBinFullPath()
	if err != nil {
		return err
	}

	var binPaths []string
	if len(bins) == 0 {
		binPaths, err = g.binaryManager.ListBinariesFullPaths(binFullPath)
		if err != nil {
			return err
		}
	} else {
		for _, bin := range bins {
			binPaths = append(binPaths, filepath.Join(binFullPath, bin))
		}
	}

	grp := new(errgroup.Group)
	grp.SetLimit(parallelism)

	for _, bin := range binPaths {
		grp.Go(func() error {
			upErr := g.binaryManager.UpgradeBinary(ctx, bin, majorUpgrade, rebuild)
			if errors.Is(upErr, ErrBinaryNotFound) {
				fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", filepath.Base(bin))
			}
			return upErr
		})
	}

	return grp.Wait()
}

func (g *Gobin) openURL(ctx context.Context, url string, execCmd ExecCombinedOutputFunc) error {
	logger := slog.Default().With("url", url)

	var cmd ExecCombinedOutput
	runtimeOS := g.system.RuntimeOS()
	switch runtimeOS {
	case "darwin":
		cmd = execCmd(ctx, "open", url)
	case "linux":
		cmd = execCmd(ctx, "xdg-open", url)
	case "windows":
		cmd = execCmd(ctx, "cmd", "/c", "start", url)
	default:
		err := fmt.Errorf("unsupported platform: %s", runtimeOS)
		logger.Error("error opening url", "err", err)
		return err
	}

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
		err = errors.New(outputStr)
		logger.Error("error opening url", "err", err)
		return err
	}

	return nil
}

func (g *Gobin) printBinaryDiagnostics(diags []BinaryDiagnostic) error {
	var diagWithIssues = make([]BinaryDiagnostic, 0, len(diags))
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
		DiagsWithIssues []BinaryDiagnostic
	}{
		Total:           len(diags),
		WithIssues:      len(diagWithIssues),
		DiagsWithIssues: diagWithIssues,
	}

	tmplParsed := template.Must(template.New("doctor").Parse(doctorTemplate))
	if err := tmplParsed.Execute(g.stdOut, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func (g *Gobin) printInstalledBinaries(binInfos []BinaryInfo) error {
	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin BinaryInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin BinaryInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Version",
		binInfos,
		func(bin BinaryInfo) string { return bin.ModuleVersion },
	)

	data := struct {
		Binaries           []BinaryInfo
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

	if err := tmplParsed.Execute(g.stdOut, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
	}

	return nil
}

func (g *Gobin) printOutdatedBinaries(binInfos []BinaryUpgradeInfo) error {
	sort.Slice(binInfos, func(i, j int) bool {
		return binInfos[i].Name < binInfos[j].Name
	})

	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin BinaryUpgradeInfo) string { return bin.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin BinaryUpgradeInfo) string { return bin.ModulePath },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Current",
		binInfos,
		func(bin BinaryUpgradeInfo) string { return bin.ModuleVersion },
	)
	maxLatestVersionWidth := getColumnMaxWidth(
		"Latest",
		binInfos,
		func(bin BinaryUpgradeInfo) string { return bin.LatestVersion },
	)

	data := struct {
		Binaries           []BinaryUpgradeInfo
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

	tmplParsed := template.Must(template.New("outdated").Funcs(template.FuncMap{
		"add":    add,
		"color":  colorize,
		"repeat": strings.Repeat,
	}).Parse(outdatedTemplate))

	if err := tmplParsed.Execute(g.stdOut, data); err != nil {
		slog.Default().Error("error executing template", "err", err)
		return err
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

func colorize(s, color string) string {
	colors := map[string]string{
		"red":   "\033[31m",
		"green": "\033[32m",
		"reset": "\033[0m",
	}
	return colors[color] + s + colors["reset"]
}

func getColumnMaxWidth[T any](header string, items []T, accessor func(T) string) int {
	maxWidth := len(header)
	for _, item := range items {
		if width := len(accessor(item)); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
