package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/gobin"
)

func main() {
	var verbose bool
	var parallelism int

	cmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin - CLI to manage Go binaries",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			level := slog.LevelWarn
			if verbose {
				level = slog.LevelDebug
			}

			slog.SetDefault(internal.NewLoggerWithLevel(level))
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

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newOutdatedCmd())
	cmd.AddCommand(newRepoCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newUpgradeCmd())
	cmd.AddCommand(newVersionCmd())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "Diagnose issues for installed binaries",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			return gobin.DiagnoseBinaries(parallelism)
		},
	}
}

func newInfoCmd() *cobra.Command {
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

func newListCmd() *cobra.Command {
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

func newOutdatedCmd() *cobra.Command {
	var checkMajor bool

	cmd := &cobra.Command{
		Use:           "outdated",
		Short:         "List outdated binaries",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			parallelism, _ := cmd.Flags().GetInt("parallelism")

			return gobin.ListOutdatedBinaries(checkMajor, parallelism)
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

func newRepoCmd() *cobra.Command {
	var open bool

	cmd := &cobra.Command{
		Use:           "repo [binary]",
		Short:         "Show binary repository",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return gobin.ShowBinaryRepository(args[0], open)
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

func newUninstallCmd() *cobra.Command {
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

func newUpgradeCmd() *cobra.Command {
	var upgradeAll bool
	var majorUpgrade bool
	var rebuild bool

	cmd := &cobra.Command{
		Use:           "upgrade [binaries]",
		Short:         "Upgrade specific binaries or all with --all",
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
				return gobin.UpgradeBinaries(majorUpgrade, rebuild, parallelism)

			case len(args) == 0:
				err := errors.New("no binaries specified (use --all to upgrade all)")
				fmt.Fprintln(os.Stderr, err.Error())
				return err

			default:
				return gobin.UpgradeBinaries(majorUpgrade, rebuild, parallelism, args...)
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

func newVersionCmd() *cobra.Command {
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
