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
)

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
			signal, _ := sig.(syscall.Signal)
			os.Exit(128 + int(signal))
		}
	}
}

// run inits and runs the gobin command. It creates a new Gobin application,
// configures the command-line interface, and runs the requested command. It
// propagates the context to ensure a graceful shutdown.
func run(ctx context.Context) int {
	execCombinedOutput := internal.NewExecCombinedOutput
	system := internal.NewSystem()
	toolchain := internal.NewGoToolchain(
		execCombinedOutput,
		internal.NewExecRun,
		internal.NewScanExecCombinedOutput,
		system,
	)

	gobin := internal.NewGobin(
		internal.NewGoBinaryManager(system, toolchain),
		execCombinedOutput,
		os.Stderr,
		os.Stdout,
		system,
	)

	var verbose bool
	var parallelism int

	cmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin - CLI to manage Go binaries",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			level := slog.LevelWarn
			if verbose {
				level = slog.LevelDebug
			}

			slog.SetDefault(internal.NewLoggerWithLevel(level))

			if parallelism < 1 {
				err := errors.New("parallelism must be greater than 0")
				fmt.Fprintf(os.Stderr, "error: %s\n\n", err.Error())
				return err
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
	cmd.AddCommand(newListCmd(gobin))
	cmd.AddCommand(newOutdatedCmd(gobin))
	cmd.AddCommand(newRepoCmd(gobin))
	cmd.AddCommand(newUninstallCmd(gobin))
	cmd.AddCommand(newUpgradeCmd(gobin))
	cmd.AddCommand(newVersionCmd(gobin))

	if err := cmd.ExecuteContext(ctx); err != nil {
		return 1
	}

	return 0
}

// newDoctorCmd creates a doctor command to diagnose issues with installed
// binaries.
func newDoctorCmd(gobin *internal.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose issues for installed binaries",
		Long: `Diagnose common issues with installed Go binaries.

Checks for:
  • Binaries not in PATH
  • Duplicate binaries in PATH  
  • Go version mismatches
  • Platform mismatches (OS/architecture)
  • Pseudo-versions and orphaned binaries
  • Binaries built without Go modules
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
func newInfoCmd(gobin *internal.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:           "info [binary]",
		Short:         "Print information about a binary",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return gobin.PrintBinaryInfo(args[0])
		},
	}
}

// newListCmd creates a list command to list installed binaries.
func newListCmd(gobin *internal.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List installed binaries",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			return gobin.ListInstalledBinaries()
		},
	}
}

// newOutdatedCmd creates a outdated command to list outdated binaries.
func newOutdatedCmd(gobin *internal.Gobin) *cobra.Command {
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

// newRepoCmd creates a repo command to show/open the repository URL for a
// binary.
func newRepoCmd(gobin *internal.Gobin) *cobra.Command {
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

			return gobin.ShowBinaryRepository(cmd.Context(), args[0], open)
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
func newUninstallCmd(gobin *internal.Gobin) *cobra.Command {
	return &cobra.Command{
		Use:           "uninstall [binary]",
		Short:         "Uninstall a binary",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return gobin.UninstallBinary(args[0])
		},
	}
}

// newUpgradeCmd creates a upgrade command to upgrade a binary.
func newUpgradeCmd(gobin *internal.Gobin) *cobra.Command {
	var upgradeAll bool
	var majorUpgrade bool
	var rebuild bool

	cmd := &cobra.Command{
		Use:   "upgrade [binaries]",
		Short: "Upgrade specific binaries or all with --all",
		Long: `Upgrade binaries to their latest versions. You can upgrade specific binaries or all outdated ones.

Examples:
  gobin upgrade dlv                    # Upgrade specific binary
  gobin upgrade dlv air gotests        # Upgrade multiple binaries  
  gobin upgrade --all                  # Upgrade all outdated binaries
  gobin upgrade --all --major          # Include major version upgrades
  gobin upgrade dlv --rebuild          # Force rebuild even if up-to-date
  gobin upgrade --all --rebuild        # Rebuild all binaries with current Go version

The --rebuild flag is useful when binaries are up-to-date but compiled with older Go versions.`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

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
					args...,
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
func newVersionCmd(gobin *internal.Gobin) *cobra.Command {
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
