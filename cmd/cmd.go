package cmd

import (
	"github.com/spf13/cobra"

	"github.com/brunoribeiro127/gobin/internal"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "gobin",
		Short: "gobin is a CLI tool for managing Go binaries",
	}

	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newOutdatedCmd())
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List installed Go binaries",
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
		Short:         "List outdated Go binaries",
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

func newUpgradeCmd() *cobra.Command {
	var majorUpgrade bool

	cmd := &cobra.Command{
		Use:           "upgrade [binary|all]",
		Short:         "Upgrade installed Go binaries",
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			if short {
				internal.PrintShortVersion()
				return nil
			}

			return internal.PrintVersion()
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
