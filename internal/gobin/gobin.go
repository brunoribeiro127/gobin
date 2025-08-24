package gobin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/sync/errgroup"

	"github.com/brunoribeiro127/gobin/internal/manager"
	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/brunoribeiro127/gobin/internal/system"
	"github.com/brunoribeiro127/gobin/internal/toolchain"
)

const (
	// doctorTemplate is the template for the doctor command.
	doctorTemplate = `{{- range .DiagsWithIssues -}}
🛠️  {{ .Name }}
    {{- if .NotInPath }}
    ❗ not in PATH
    {{- end }}
    {{- if .DuplicatesInPath }}
    ❗ duplicated in PATH:
        {{- range .DuplicatesInPath }}
        • {{ . }}
        {{- end }}
    {{- end }}
	{{- if .IsNotManaged }}
    ❗ not managed by gobin
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
	{{- if ne .GoVersion.Actual .GoVersion.Expected }}
    ❗ go version mismatch: expected {{ .GoVersion.Expected }}, actual {{ .GoVersion.Actual }}
    {{- end }}
    {{- if ne .Platform.Actual .Platform.Expected }}
    ❗ platform mismatch: expected {{ .Platform.Expected }}, actual {{ .Platform.Actual }}
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
{{end -}}
{{- if gt .WithIssues 0 }}
{{""}}
{{- end -}}
{{ .Total }} binaries checked, {{ .WithIssues }} with issues
`

	// infoTemplate is the template for the info command.
	infoTemplate = `Path          {{.FullPath}}
Location      {{if eq .FullPath .InstallPath}}<unmanaged>{{else}}{{.InstallPath}}{{end}}
Package       {{.PackagePath}}
Module        {{.Module.String}}
Module Sum    {{if .ModuleSum}}{{.ModuleSum}}{{else}}<none>{{end}}
{{- if .CommitRevision}}
Commit        {{.CommitRevision}}{{if .CommitTime}} ({{.CommitTime}}){{end}}
{{- end}}
Go Version    {{.GoVersion}}
Platform      {{.OS}}/{{.Arch}}/{{.Feature}}
Env Vars      {{range $index, $env := .EnvVars}}{{if eq $index 0}}{{$env}}{{else}}
              {{$env}}{{end}}{{end}}
`

	// listInstalledTemplate is the template for the list command for installed
	// binaries.
	listInstalledTemplate = `{{printf "%-*s" $.NameWidth "Name"}} → {{printf "%-*s" $.ModulePathWidth "Module"}} @ {{printf "%-*s" $.ModuleVersionWidth "Version"}}
{{repeat "-" (add $.NameWidth $.ModulePathWidth $.ModuleVersionWidth 6)}}
{{range .Binaries -}}
{{if .IsManaged}}{{color (printf "%-*s" $.NameWidth .Binary.Name) "green"}}{{else}}{{printf "%-*s" $.NameWidth .Binary.Name}}{{end}} → {{printf "%-*s" $.ModulePathWidth .Module.Path}} @ {{printf "%-*s" $.ModuleVersionWidth .Module.Version.String}}
{{end -}}
`

	// listManagedTemplate is the template for the list command for managed
	// binaries.
	listManagedTemplate = `{{printf "%-*s" $.NameWidth "Name"}} → {{printf "%-*s" $.ModulePathWidth "Module"}} @ {{printf "%-*s" $.ModuleVersionWidth "Version"}}
{{repeat "-" (add $.NameWidth $.ModulePathWidth $.ModuleVersionWidth 6)}}
{{range .Binaries -}}
{{if .IsPinned}}{{color (printf "%-*s" $.NameWidth .Binary.Name) "green"}}{{else}}{{printf "%-*s" $.NameWidth .Binary.Name}}{{end}} → {{printf "%-*s" $.ModulePathWidth .Module.Path}} @ {{printf "%-*s" $.ModuleVersionWidth .Module.Version.String}}
{{end -}}
`

	// outdatedTemplate is the template for the outdated command.
	outdatedTemplate = `{{printf "%-*s" $.NameWidth "Name"}} → {{printf "%-*s" $.ModulePathWidth "Module"}} @ {{printf "%-*s" $.ModuleVersionWidth "Current"}} ↑ {{printf "%-*s" $.LatestVersionWidth "Latest"}}
{{repeat "-" (add $.NameWidth $.ModulePathWidth $.ModuleVersionWidth $.LatestVersionWidth 9)}}
{{range .Binaries -}}
{{printf "%-*s" $.NameWidth .Binary.Name}} → {{printf "%-*s" $.ModulePathWidth .Module.Path}} @ {{color (printf "%-*s" $.ModuleVersionWidth .Module.Version.String) "red"}} ↑ {{color (printf "%-*s" $.LatestVersionWidth .LatestModule.Version.String) "green"}}
{{end -}}
`
)

// Gobin is an application that manages Go binaries.
type Gobin struct {
	binaryManager manager.BinaryManager
	fs            system.FileSystem
	resource      system.Resource
	stdErr        io.Writer
	stdOut        io.Writer
	workspace     system.Workspace
}

// NewGobin creates a new Gobin application.
func NewGobin(
	binaryManager manager.BinaryManager,
	fs system.FileSystem,
	resource system.Resource,
	stdErr io.Writer,
	stdOut io.Writer,
	workspace system.Workspace,
) *Gobin {
	return &Gobin{
		binaryManager: binaryManager,
		fs:            fs,
		resource:      resource,
		stdErr:        stdErr,
		stdOut:        stdOut,
		workspace:     workspace,
	}
}

// DiagnoseBinaries diagnoses issues in all binaries in the Go binary directory.
// It prints a template with the diagnostic results to the standard output (or
// another defined io.Writer), or an error if the binary directory cannot be
// determined or listed. The command runs in parallel, launching go routines to
// diagnose binaries up to the given parallelism.
func (g *Gobin) DiagnoseBinaries(ctx context.Context, parallelism int) error {
	bins, err := g.fs.ListBinaries(g.workspace.GetGoBinPath())
	if err != nil {
		return err
	}

	var (
		mutex sync.Mutex
		diags = make([]model.BinaryDiagnostic, 0, len(bins))
		grp   = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, bin := range bins {
		grp.Go(func() error {
			diag, diagErr := g.binaryManager.DiagnoseBinary(ctx, bin)
			if diagErr != nil {
				fmt.Fprintf(g.stdErr, "❌ error diagnosing binary %q\n", filepath.Base(bin))
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

// InstallPackages installs the given packages. It returns an error if any of
// the packages cannot be installed. The command runs in parallel, launching go
// routines to install the packages up to the given parallelism.
func (g *Gobin) InstallPackages(
	ctx context.Context,
	parallelism int,
	kind model.Kind,
	rebuild bool,
	packages ...model.Package,
) error {
	grp := new(errgroup.Group)
	grp.SetLimit(parallelism)

	for _, pkg := range packages {
		grp.Go(func() error {
			return g.binaryManager.InstallPackage(ctx, pkg, kind, rebuild)
		})
	}

	return grp.Wait()
}

// ListBinaries lists all binaries in the Go binary directory, or if managed is
// true, it lists all binaries in the internal binary directory. It prints a
// template with the binaries to the standard output (or another defined
// io.Writer), or an error if the binary directory cannot be determined or
// listed.
func (g *Gobin) ListBinaries(managed bool) error {
	binInfos, err := g.binaryManager.GetAllBinaryInfos(managed)
	if err != nil {
		return err
	}

	return g.printBinaries(binInfos, managed)
}

// ListOutdatedBinaries lists all outdated binaries in the Go binary directory.
// It prints a template with the outdated binaries to the standard output (or
// another defined io.Writer), or an error if the binary directory cannot be
// determined or listed. The command runs in parallel, launching go routines to
// check the upgrade information of the binaries up to the given parallelism.
func (g *Gobin) ListOutdatedBinaries(
	ctx context.Context,
	checkMajor bool,
	parallelism int,
) error {
	binInfos, err := g.binaryManager.GetAllBinaryInfos(false)
	if err != nil {
		return err
	}

	var (
		mutex    sync.Mutex
		outdated = make([]model.BinaryUpgradeInfo, 0, len(binInfos))
		grp      = new(errgroup.Group)
	)

	grp.SetLimit(parallelism)

	for _, info := range binInfos {
		grp.Go(func() error {
			binUpInfo, infoErr := g.binaryManager.GetBinaryUpgradeInfo(
				ctx, info, checkMajor,
			)
			if errors.Is(infoErr, toolchain.ErrBinaryBuiltWithoutGoModules) {
				return nil
			} else if infoErr != nil {
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
		if waitErr == nil {
			fmt.Fprintln(g.stdOut, "✅ All binaries are up to date")
			return nil
		}

		return waitErr
	}

	if err = g.printOutdatedBinaries(outdated); err != nil {
		return err
	}

	return waitErr
}

// MigrateBinaries migrates the given binaries to be managed internally. It
// returns an error if any of the binaries cannot be migrated due to the binary
// being not found or the binary being already managed or any other error.
func (g *Gobin) MigrateBinaries(bins ...model.Binary) error {
	var err error
	var binPaths []string

	goBinPath := g.workspace.GetGoBinPath()

	if len(bins) == 0 {
		binPaths, err = g.fs.ListBinaries(goBinPath)
		if err != nil {
			return err
		}
	} else {
		for _, bin := range bins {
			binPaths = append(binPaths, filepath.Join(goBinPath, bin.String()))
		}
	}

	for _, path := range binPaths {
		if migrateErr := g.binaryManager.MigrateBinary(path); migrateErr != nil {
			switch {
			case errors.Is(migrateErr, manager.ErrBinaryAlreadyManaged):
				fmt.Fprintf(g.stdErr, "❌ binary %q already managed\n", filepath.Base(path))
			case errors.Is(migrateErr, toolchain.ErrBinaryNotFound):
				fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", filepath.Base(path))
			default:
				fmt.Fprintf(g.stdErr, "❌ error migrating binary %q\n", filepath.Base(path))
			}

			err = migrateErr
			continue
		}
	}

	return err
}

// PinBinaries pins the given binaries to the Go binary directory. It returns an
// error if any of the binaries cannot be pinned.
func (g *Gobin) PinBinaries(kind model.Kind, bins ...model.Binary) error {
	var err error
	for _, bin := range bins {
		pinErr := g.binaryManager.PinBinary(bin, kind)
		if errors.Is(pinErr, toolchain.ErrBinaryNotFound) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", bin.String())
		} else if pinErr != nil {
			fmt.Fprintf(g.stdErr, "❌ error pinning binary %q\n", bin.String())
		}

		err = pinErr
	}

	return err
}

// PrintBinaryInfo prints the binary info for a given binary. It prints a
// template with the binary info to the standard output (or another defined
// io.Writer), or an error if the binary cannot be found.
func (g *Gobin) PrintBinaryInfo(bin model.Binary) error {
	binInfo, err := g.binaryManager.GetBinaryInfo(
		filepath.Join(g.workspace.GetGoBinPath(), bin.String()),
	)
	if err != nil {
		if errors.Is(err, toolchain.ErrBinaryNotFound) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", bin.String())
		} else {
			fmt.Fprintf(g.stdErr, "❌ error getting info for binary %q\n", bin.String())
		}

		return err
	}

	tmplParsed := template.Must(template.New("info").Parse(infoTemplate))
	if err = tmplParsed.Execute(g.stdOut, binInfo); err != nil {
		slog.Default().Error("error executing template", "template", tmplParsed.Name(), "err", err)
		return err
	}

	return nil
}

// PrintShortVersion prints the short version of a given binary. It prints the
// module version to the standard output (or another defined io.Writer), or an
// error if the binary cannot be found.
func (g *Gobin) PrintShortVersion(path string) error {
	binInfo, err := g.binaryManager.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintln(g.stdOut, binInfo.Module.Version.String())

	return nil
}

// PrintVersion prints the version of a given binary. It prints the module
// version, Go version, OS, and architecture to the standard output (or another
// defined io.Writer), or an error if the binary cannot be found.
func (g *Gobin) PrintVersion(path string) error {
	binInfo, err := g.binaryManager.GetBinaryInfo(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		g.stdOut,
		"%s (%s %s/%s)\n",
		binInfo.Module.Version.String(),
		binInfo.GoVersion,
		binInfo.OS,
		binInfo.Arch,
	)

	return nil
}

// PruneBinaries prunes the given binaries or the whole internal binary
// directory. It returns an error if the binary directory cannot be determined
// or listed, or if the binaries failed to be pruned.
func (g *Gobin) PruneBinaries(bins ...model.Binary) error {
	if len(bins) == 0 {
		binPaths, err := g.fs.ListBinaries(g.workspace.GetInternalBinPath())
		if err != nil {
			return err
		}

		for _, path := range binPaths {
			bin := model.NewBinaryFromString(filepath.Base(path))
			bins = append(bins, model.NewBinaryFromString(bin.Name+bin.Extension))
		}

		bins = slices.Compact(bins)
	}

	var err error
	for _, bin := range bins {
		if pruneErr := g.binaryManager.PruneBinary(bin); pruneErr != nil {
			err = pruneErr
			continue
		}
	}

	return err
}

// ShowBinaryRepository shows the repository URL for a given binary. It prints
// the repository URL to the standard output (or another defined io.Writer), or
// an error if the binary cannot be found. If the open flag is set, it opens the
// repository URL in the default system browser.
func (g *Gobin) ShowBinaryRepository(ctx context.Context, bin model.Binary, open bool) error {
	repoURL, err := g.binaryManager.GetBinaryRepository(ctx, bin)
	if err != nil {
		if errors.Is(err, toolchain.ErrBinaryNotFound) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", bin.String())
		} else {
			fmt.Fprintf(g.stdErr, "❌ error getting repository for binary %q\n", bin.String())
		}

		return err
	}

	if open {
		return g.resource.Open(ctx, repoURL)
	}

	fmt.Fprintln(g.stdOut, repoURL)
	return nil
}

// UninstallBinaries uninstalls the given binaries by removing the binary files.
// It returns an error if the binary cannot be found or removed.
func (g *Gobin) UninstallBinaries(bins ...model.Binary) error {
	var err error
	for _, bin := range bins {
		removeErr := g.binaryManager.UninstallBinary(bin)
		if errors.Is(removeErr, os.ErrNotExist) {
			fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", bin)
		} else if removeErr != nil {
			fmt.Fprintf(g.stdErr, "❌ error uninstalling binary %q\n", bin)
		}

		err = removeErr
	}

	return err
}

// UpgradeBinaries upgrades the given binaries or all binaries in the Go binary
// directory. If majorUpgrade is set, it upgrades the major version of the
// binaries. If rebuild is set, it rebuilds the binaries. It returns an error if
// the binary directory cannot be determined or listed. The command runs in
// parallel, launching go routines to upgrade the binaries up to the given
// parallelism.
func (g *Gobin) UpgradeBinaries(
	ctx context.Context,
	majorUpgrade bool,
	rebuild bool,
	parallelism int,
	bins ...model.Binary,
) error {
	binFullPath := g.workspace.GetGoBinPath()

	var binPaths []string
	if len(bins) == 0 {
		var err error
		binPaths, err = g.fs.ListBinaries(binFullPath)
		if err != nil {
			return err
		}
	} else {
		for _, bin := range bins {
			binPaths = append(binPaths, filepath.Join(binFullPath, bin.String()))
		}
	}

	grp := new(errgroup.Group)
	grp.SetLimit(parallelism)

	for _, bin := range binPaths {
		grp.Go(func() error {
			upErr := g.binaryManager.UpgradeBinary(ctx, bin, majorUpgrade, rebuild)
			if errors.Is(upErr, toolchain.ErrBinaryNotFound) {
				fmt.Fprintf(g.stdErr, "❌ binary %q not found\n", filepath.Base(bin))
			} else if upErr != nil {
				fmt.Fprintf(g.stdErr, "❌ error upgrading binary %q\n", filepath.Base(bin))
			}

			return upErr
		})
	}

	return grp.Wait()
}

// printBinaryDiagnostics prints the binary diagnostics to the standard output
// (or another defined io.Writer).
func (g *Gobin) printBinaryDiagnostics(diags []model.BinaryDiagnostic) error {
	var diagWithIssues = make([]model.BinaryDiagnostic, 0, len(diags))
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
		DiagsWithIssues []model.BinaryDiagnostic
	}{
		Total:           len(diags),
		WithIssues:      len(diagWithIssues),
		DiagsWithIssues: diagWithIssues,
	}

	tmplParsed := template.Must(template.New("doctor").Parse(doctorTemplate))
	if err := tmplParsed.Execute(g.stdOut, data); err != nil {
		slog.Default().Error("error executing template", "template", tmplParsed.Name(), "err", err)
		return err
	}

	return nil
}

// printBinaries prints the binaries to the standard output (or another defined
// io.Writer). If managed is false, it prints the installed binaries, highlighting
// the managed binaries in green. If managed is true, it prints the managed
// binaries, highlighting the pinned binaries in green.
func (g *Gobin) printBinaries(binInfos []model.BinaryInfo, managed bool) error {
	sort.Slice(binInfos, func(i, j int) bool {
		if binInfos[i].Binary.Name != binInfos[j].Binary.Name {
			return binInfos[i].Binary.Name < binInfos[j].Binary.Name
		}

		return binInfos[i].Module.Version.Compare(binInfos[j].Module.Version) > 0
	})

	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin model.BinaryInfo) string { return bin.Binary.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin model.BinaryInfo) string { return bin.Module.Path },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Version",
		binInfos,
		func(bin model.BinaryInfo) string { return bin.Module.Version.String() },
	)

	data := struct {
		Binaries           []model.BinaryInfo
		NameWidth          int
		ModulePathWidth    int
		ModuleVersionWidth int
	}{
		Binaries:           binInfos,
		NameWidth:          maxNameWidth,
		ModulePathWidth:    maxModulePathWidth,
		ModuleVersionWidth: maxModuleVersionWidth,
	}

	tmpl := listInstalledTemplate
	if managed {
		tmpl = listManagedTemplate
	}

	tmplParsed := template.Must(template.New("list").Funcs(template.FuncMap{
		"add":    add,
		"color":  colorize,
		"repeat": strings.Repeat,
	}).Parse(tmpl))

	if err := tmplParsed.Execute(g.stdOut, data); err != nil {
		slog.Default().Error("error executing template", "template", tmplParsed.Name(), "err", err)
		return err
	}

	return nil
}

// printOutdatedBinaries prints the outdated binaries to the standard output
// (or another defined io.Writer).
func (g *Gobin) printOutdatedBinaries(binInfos []model.BinaryUpgradeInfo) error {
	sort.Slice(binInfos, func(i, j int) bool {
		return binInfos[i].Binary.Name < binInfos[j].Binary.Name
	})

	maxNameWidth := getColumnMaxWidth(
		"Name",
		binInfos,
		func(bin model.BinaryUpgradeInfo) string { return bin.Binary.Name },
	)
	maxModulePathWidth := getColumnMaxWidth(
		"Module",
		binInfos,
		func(bin model.BinaryUpgradeInfo) string { return bin.Module.Path },
	)
	maxModuleVersionWidth := getColumnMaxWidth(
		"Current",
		binInfos,
		func(bin model.BinaryUpgradeInfo) string { return bin.Module.Version.String() },
	)
	maxLatestVersionWidth := getColumnMaxWidth(
		"Latest",
		binInfos,
		func(bin model.BinaryUpgradeInfo) string { return bin.LatestModule.Version.String() },
	)

	data := struct {
		Binaries           []model.BinaryUpgradeInfo
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

// add adds the given integers.
func add(args ...int) int {
	sum := 0
	for _, v := range args {
		sum += v
	}
	return sum
}

// colorize colorizes a given string with a given color.
func colorize(s, color string) string {
	colors := map[string]string{
		"red":   "\033[31m",
		"green": "\033[32m",
		"reset": "\033[0m",
	}
	return colors[color] + s + colors["reset"]
}

// getColumnMaxWidth gets the maximum width of a column for a given header and
// items.
func getColumnMaxWidth[T any](header string, items []T, accessor func(T) string) int {
	maxWidth := len(header)
	for _, item := range items {
		if width := len(accessor(item)); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
