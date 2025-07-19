package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/brunoribeiro127/gobin/internal/gobin"
)

func main() {
	slog.SetDefault(internal.NewLogger())

	cmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin - CLI tool to manage Go binaries",
	}

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newOutdatedCmd())
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

			return gobin.DiagnoseBinaries()
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

			return gobin.ListOutdatedBinaries(checkMajor)
		},
	}

	cmd.Flags().BoolVarP(
		&checkMajor,
		"major",
		"m",
		false,
		"Checks for major versions",
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
	var majorUpgrade bool

	cmd := &cobra.Command{
		Use:           "upgrade [binary|all]",
		Short:         "Upgrade one or all binaries",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if args[0] == "all" {
				return gobin.UpgradeAllBinaries(majorUpgrade)
			}

			return gobin.UpgradeBinary(args[0], majorUpgrade)
		},
	}

	cmd.Flags().BoolVarP(
		&majorUpgrade,
		"major",
		"m",
		false,
		"Upgrades major version",
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
		"Print short version info",
	)

	return cmd
}
