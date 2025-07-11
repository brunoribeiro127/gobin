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
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}

func newListCmd() *cobra.Command {
	var checkMajor bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed Go binaries",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			binInfos, err := internal.GetAllBinInfos(checkMajor)
			if err != nil {
				return err
			}

			return internal.PrintTabularBinInfos(binInfos)
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
		Use:   "upgrade",
		Short: "Upgrade installed Go binaries",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			binInfos, err := internal.GetAllBinInfos(majorUpgrade)
			if err != nil {
				return err
			}

			for _, info := range binInfos {
				if info.NeedsUpgrade {
					if err = internal.InstallGoBin(info); err != nil {
						return err
					}
				}
			}

			return nil
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
		Use:   "version",
		Short: "Shows the package version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
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
