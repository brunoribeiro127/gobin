package cmd

import (
	"github.com/spf13/cobra"

	"github.com/brunoribeiro127/gobin/internal"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin - CLI tool to manage Go binaries",
	}

	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newInfoCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newOutdatedCmd())
	rootCmd.AddCommand(newUninstallCmd())
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "Diagnose issues for installed binaries",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			return internal.DiagnoseBinaries()
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

			return internal.PrintBinaryInfo(args[0])
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

			return internal.ListInstalledBinaries()
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

			return internal.ListOutdatedBinaries(checkMajor)
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

			return internal.UninstallBinary(args[0])
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
				return internal.UpgradeAllBinaries(majorUpgrade)
			}

			return internal.UpgradeBinary(args[0], majorUpgrade)
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
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.SilenceUsage = true

			if short {
				internal.PrintShortVersion()
			} else {
				internal.PrintVersion()
			}
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
