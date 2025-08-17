package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/gobin"
	"github.com/brunoribeiro127/gobin/internal/manager"
	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/brunoribeiro127/gobin/internal/system"
	"github.com/brunoribeiro127/gobin/internal/toolchain"
)

// exitCodeSignalOffset is the offset for signal exit codes when terminates via
// signal.
const exitCodeSignalOffset = 128

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	resChan := make(chan int, 1)
	go func() {
		resChan <- run(ctx)
	}()

	select {
	case exitCode := <-resChan:
		cancel()
		os.Exit(exitCode)
	case <-sigChan:
		cancel()

		select {
		case exitCode := <-resChan:
			os.Exit(exitCode)
		case sig := <-sigChan:
			exitWithSignal(sig)
		}
	}
}

// exitWithSignal exits the program with the appropriate signal exit code.
func exitWithSignal(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok {
		os.Exit(exitCodeSignalOffset + int(s))
	}

	os.Exit(1)
}

// run inits and runs the gobin command. It creates a new Gobin application,
// configures the command-line interface, and runs the requested command. It
// propagates the context to ensure a graceful shutdown.
func run(ctx context.Context) int {
	env := system.NewEnvironment()
	exec := system.NewExec()
	rt := system.NewRuntime()
	fs := system.NewFileSystem(rt)

	workspace, err := system.NewWorkspace(env, fs, rt)
	if err != nil {
		return 1
	}

	gobin := gobin.NewGobin(
		manager.NewGoBinaryManager(
			fs,
			rt,
			toolchain.NewGoToolchain(
				system.NewBuildInfo(),
				exec,
				toolchain.NewScanExecCombinedOutput,
			),
			workspace,
		),
		fs,
		system.NewResource(exec, rt),
		os.Stderr,
		os.Stdout,
		workspace,
	)

	var verbose bool
	var parallelism int

	cmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin - CLI to manage Go binaries",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			level := slog.LevelError
			if verbose {
				level = slog.LevelInfo
			}

			slog.SetDefault(internal.NewLoggerWithLevel(level))

			if parallelism < 1 {
				parallelismErr := errors.New("parallelism must be greater than 0")
				fmt.Fprintf(os.Stderr, "error: %s\n\n", parallelismErr.Error())
				return parallelismErr
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false,
		"enable verbose output",
	)

	cmd.PersistentFlags().IntVarP(
		&parallelism,
		"parallelism",
		"p",
		runtime.NumCPU(),
		"number of concurrent operations (default: number of CPU cores)",
	)

	cmd.AddCommand(newDoctorCmd(gobin))
	cmd.AddCommand(newInfoCmd(gobin))
	cmd.AddCommand(newInstallCmd(gobin))
	cmd.AddCommand(newListCmd(gobin))
	cmd.AddCommand(newMigrateCmd(gobin))
	cmd.AddCommand(newOutdatedCmd(gobin))
	cmd.AddCommand(newPinCmd(gobin))
	cmd.AddCommand(newRepoCmd(gobin))
	cmd.AddCommand(newUninstallCmd(gobin))
	cmd.AddCommand(newUpgradeCmd(gobin))
	cmd.AddCommand(newVersionCmd(gobin))

	if err = cmd.ExecuteContext(ctx); err != nil {
		return 1
	}

	return 0
}

// newDoctorCmd creates a doctor command to diagnose issues with installed
// binaries.
func newDoctorCmd(gobin *gobin.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose issues for installed binaries",
		Long: `Diagnose common issues with installed Go binaries.

Checks for:
  • Binaries not in PATH
  • Duplicate binaries in PATH
  • Binaries not managed by gobin
  • Pseudo-versions and orphaned binaries
  • Binaries built without Go modules
  • Go version mismatches
  • Platform mismatches (OS/architecture)
  • Retracted or deprecated modules
  • Known security vulnerabilities

Run this command regularly to make sure everything is ok with your installed binaries.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			return gobin.DiagnoseBinaries(cmd.Context(), parallelism)
		},
	}
}

// newInfoCmd creates a info command to print information about a binary.
func newInfoCmd(gobin *gobin.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:           "info [binary]",
		Short:         "Print information about a binary",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			bin := model.NewBinary(args[0])
			if !bin.IsValid() {
				err := fmt.Errorf("invalid binary argument: %s", args[0])
				fmt.Fprintln(os.Stderr, err.Error())
				return err
			}

			return gobin.PrintBinaryInfo(bin)
		},
	}
}

// newInstallCmd creates a install command to install packages.
func newInstallCmd(gobin *gobin.Gobin) *cobra.Command {
	kind := model.KindLatest
	var rebuild bool

	cmd := &cobra.Command{
		Use:   "install [packages]",
		Short: "Install packages",
		Long: `Install compiles and installs the packages named by the import paths. You can specify the pin kind to create
[latest (default), major, minor] and whether to rebuild the package and its dependencies.

Examples:
  gobin install github.com/go-delve/delve/cmd/dlv                      # Install latest version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@latest               # Install latest version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1                   # Install latest v1 minor version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25                # Install latest v1.25 patch version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25.1              # Install specific version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25.1 --rebuild    # Force package and dependencies rebuild (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25.1 --kind major # Install and pin latest version (dlv)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25.1 --kind major # Install and pin major version (dlv-v1)
  gobin install github.com/go-delve/delve/cmd/dlv@v1.25.1 --kind minor # Install and pin minor version (dlv-v1.25)

The package version is optional, defaulting to "latest".
The GOFLAGS environment variable can be used to define build flags.`,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			packages := make([]model.Package, len(args))
			for i, arg := range args {
				pkg := model.NewPackage(arg)
				if !pkg.IsValid() {
					err := fmt.Errorf("invalid package argument: %s", arg)
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}

				packages[i] = pkg
			}

			return gobin.InstallPackages(cmd.Context(), parallelism, kind, rebuild, packages...)
		},
	}

	cmd.Flags().VarP(
		&kind,
		"kind",
		"k",
		"pin kind [latest (default), major, minor]",
	)

	cmd.Flags().BoolVarP(
		&rebuild,
		"rebuild",
		"r",
		false,
		"forces package and dependencies rebuild",
	)

	return cmd
}

// newListCmd creates a list command to list installed binaries.
func newListCmd(gobin *gobin.Gobin) *cobra.Command {
	var managed bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed binaries",
		Long: `List installed binaries.

Examples:
  gobin list                   # List binaries in the Go binary path
  gobin list --managed         # List all managed binaries

By default, only binaries in the GO bin path are shown. Use -m to list all
managed binaries.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			return gobin.ListInstalledBinaries(managed)
		},
	}

	cmd.Flags().BoolVarP(
		&managed,
		"managed",
		"m",
		false,
		"list all managed binaries",
	)

	return cmd
}

// newMigrateCmd creates a migrate command to migrate binaries to be managed
// internally.
func newMigrateCmd(gobin *gobin.Gobin) *cobra.Command {
	var migrateAll bool

	cmd := &cobra.Command{
		Use:   "migrate [binaries]",
		Short: "Migrate specific binaries or all with --all",
		Long: `Migrate binaries to be managed internally. You can migrate specific binaries or all binaries.

Examples:
  gobin migrate dlv                        # Migrate specific binary
  gobin migrate dlv golangci-lint mockery  # Migrate multiple binaries  
  gobin migrate --all                 	   # Migrate all binaries`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			bins := make([]model.Binary, len(args))
			for i, arg := range args {
				bin := model.NewBinary(arg)
				if !bin.IsValid() {
					err := fmt.Errorf("invalid binary argument: %s", arg)
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}

				bins[i] = bin
			}

			switch {
			case migrateAll && len(args) > 0:
				err := errors.New("cannot use --all with specific binaries")
				fmt.Fprintln(os.Stderr, err.Error())
				return err

			case migrateAll:
				return gobin.MigrateBinaries()

			case len(args) == 0:
				err := errors.New("no binaries specified (use --all to migrate all)")
				fmt.Fprintln(os.Stderr, err.Error())
				return err

			default:
				return gobin.MigrateBinaries(bins...)
			}
		},
	}

	cmd.Flags().BoolVarP(
		&migrateAll,
		"all",
		"a",
		false,
		"migrates all binaries",
	)

	return cmd
}

// newOutdatedCmd creates a outdated command to list outdated binaries.
func newOutdatedCmd(gobin *gobin.Gobin) *cobra.Command {
	var checkMajor bool

	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "List outdated binaries",
		Long: `List binaries that have newer versions available.

Examples:
  gobin outdated                       # Show outdated binaries (minor/patch only)
  gobin outdated --major               # Include major version upgrades

By default, only minor and patch updates are shown. Use --major to include
potentially breaking major version upgrades.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			return gobin.ListOutdatedBinaries(cmd.Context(), checkMajor, parallelism)
		},
	}

	cmd.Flags().BoolVarP(
		&checkMajor,
		"major",
		"m",
		false,
		"checks for major versions",
	)

	return cmd
}

// newPinCmd creates a pin command to pin managed binaries.
func newPinCmd(gobin *gobin.Gobin) *cobra.Command {
	kind := model.KindLatest

	cmd := &cobra.Command{
		Use:   "pin [binaries]",
		Short: "Pin binaries to the Go binary path",
		Long: `Pin managed binaries to the Go binary path.

Examples:
  gobin pin dlv                              # Pin latest version (dlv)
  gobin pin dlv@v1                           # Pin latest v1 minor version (dlv)
  gobin pin dlv@v1.25                        # Pin latest v1.25 patch version (dlv)
  gobin pin dlv@v1.25.1                      # Pin specific version (dlv)
  gobin pin dlv mockery@3.5                  # Pin multiple binaries to latest version (dlv, mockery)
  gobin pin dlv@v1 --kind major              # Pin latest v1 minor version (dlv-v1)
  gobin pin dlv@v1.25 --kind minor           # Pin latest v1.25 patch version (dlv-v1.25)`,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			bins := make([]model.Binary, len(args))
			for i, arg := range args {
				bin := model.NewBinary(arg)
				if !bin.IsValid() {
					err := fmt.Errorf("invalid binary argument: %s", arg)
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}

				bins[i] = bin
			}

			return gobin.PinBinaries(kind, bins...)
		},
	}

	cmd.Flags().VarP(
		&kind,
		"kind",
		"k",
		"pin kind [latest (default), major, minor]",
	)

	return cmd
}

// newRepoCmd creates a repo command to show/open the repository URL for a
// binary.
func newRepoCmd(gobin *gobin.Gobin) *cobra.Command {
	var open bool

	cmd := &cobra.Command{
		Use:   "repo [binary]",
		Short: "Show binary repository",
		Long: `Show the repository URL for a Go binary.

Examples:
  gobin repo dlv                       # Print repository URL
  gobin repo dlv --open                # Open repository in browser

The repository URL is determined from the module's origin information,
falling back to constructing the URL from the module path.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			bin := model.NewBinary(args[0])
			if !bin.IsValid() {
				err := fmt.Errorf("invalid binary argument: %s", args[0])
				fmt.Fprintln(os.Stderr, err.Error())
				return err
			}

			return gobin.ShowBinaryRepository(cmd.Context(), bin, open)
		},
	}

	cmd.Flags().BoolVarP(
		&open,
		"open",
		"o",
		false,
		"opens the repository in the default browser",
	)

	return cmd
}

// newUninstallCmd creates a uninstall command to uninstall a binary.
func newUninstallCmd(gobin *gobin.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:           "uninstall [binaries]",
		Short:         "Uninstall binaries",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			bins := make([]model.Binary, len(args))
			for i, arg := range args {
				bin := model.NewBinary(arg)
				if !bin.IsValid() {
					err := fmt.Errorf("invalid binary argument: %s", arg)
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}

				bins[i] = bin
			}

			return gobin.UninstallBinaries(bins...)
		},
	}
}

// newUpgradeCmd creates a upgrade command to upgrade a binary.
func newUpgradeCmd(gobin *gobin.Gobin) *cobra.Command {
	var upgradeAll bool
	var majorUpgrade bool
	var rebuild bool

	cmd := &cobra.Command{
		Use:   "upgrade [binaries]",
		Short: "Upgrade specific binaries or all with --all",
		Long: `Upgrade binaries to their latest versions. You can upgrade specific binaries or all outdated ones.

Examples:
  gobin upgrade dlv                        # Upgrade specific binary
  gobin upgrade dlv golangci-lint mockery  # Upgrade multiple binaries  
  gobin upgrade --all                 	   # Upgrade all outdated binaries
  gobin upgrade --all --major              # Include major version upgrades
  gobin upgrade dlv --rebuild              # Force rebuild even if up-to-date
  gobin upgrade --all --rebuild            # Rebuild all binaries with current Go version

The --rebuild flag is useful when binaries are up-to-date but compiled with older Go versions.`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			bins := make([]model.Binary, len(args))
			for i, arg := range args {
				bin := model.NewBinary(arg)
				if !bin.IsValid() {
					err := fmt.Errorf("invalid binary argument: %s", arg)
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}

				bins[i] = bin
			}

			switch {
			case upgradeAll && len(args) > 0:
				err := errors.New("cannot use --all with specific binaries")
				fmt.Fprintln(os.Stderr, err.Error())
				return err

			case upgradeAll:
				return gobin.UpgradeBinaries(
					cmd.Context(),
					majorUpgrade,
					rebuild,
					parallelism,
				)

			case len(args) == 0:
				err := errors.New("no binaries specified (use --all to upgrade all)")
				fmt.Fprintln(os.Stderr, err.Error())
				return err

			default:
				return gobin.UpgradeBinaries(
					cmd.Context(),
					majorUpgrade,
					rebuild,
					parallelism,
					bins...,
				)
			}
		},
	}

	cmd.Flags().BoolVarP(
		&upgradeAll,
		"all",
		"a",
		false,
		"upgrades all binaries",
	)

	cmd.Flags().BoolVarP(
		&majorUpgrade,
		"major",
		"m",
		false,
		"upgrades major version",
	)

	cmd.Flags().BoolVarP(
		&rebuild,
		"rebuild",
		"r",
		false,
		"forces binary rebuild",
	)

	return cmd
}

// newVersionCmd creates a version command to print the version of the package.
func newVersionCmd(gobin *gobin.Gobin) *cobra.Command {
	var short bool

	var cmd = &cobra.Command{
		Use:           "version",
		Short:         "Shows the package version",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			path, err := os.Executable()
			if err != nil {
				return err
			}

			if short {
				return gobin.PrintShortVersion(path)
			}

			return gobin.PrintVersion(path)
		},
	}

	cmd.Flags().BoolVarP(
		&short,
		"short",
		"s",
		false,
		"prints short version info",
	)

	return cmd
}
