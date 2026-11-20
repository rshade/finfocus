package cli

import (
	"context"
	"fmt"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
)

const registryCommandPair = 2

func newPluginRegistryCmd() *cobra.Command {
	service := registry.NewCustomRegistryService(nil)
	cmd := &cobra.Command{Use: "registry", Short: "Manage custom plugin registries"}
	cmd.AddCommand(
		newRegistryAddCmd(service),
		newRegistryListCmd(service),
		newRegistryRemoveCmd(service),
		newRegistryInstallCmd(service),
	)
	return cmd
}

func newRegistryAddCmd(service *registry.CustomRegistryService) *cobra.Command {
	var source config.PluginRegistrySource
	cmd := &cobra.Command{
		Use:   "add <name> <https-url>",
		Short: "Add a custom registry to global configuration",
		Args:  cobra.ExactArgs(registryCommandPair),
		RunE: func(cmd *cobra.Command, args []string) error {
			source.Name, source.URL = args[0], args[1]
			ctx := cmd.Context()
			if err := config.ValidatePluginRegistrySource(source); err != nil {
				return toValidationError(ctx, err)
			}
			return ax.Perform(ctx, func(context.Context) error {
				cmd.Printf("Would add plugin registry %s\n", source.Name)
				return nil
			}, func(ctx context.Context) error {
				if err := service.Add(ctx, source); err != nil {
					return fmt.Errorf("adding plugin registry: %w", err)
				}
				cmd.Printf("Added plugin registry %s\n", source.Name)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&source.Trust, "trust", "community", "Trust level: community or experimental")
	cmd.Flags().StringVar(&source.SHA256, "sha256", "", "Expected SHA-256 of the registry index")
	return cmd
}

func newRegistryListCmd(service *registry.CustomRegistryService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured custom registries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sources, err := service.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				cmd.Println("No custom plugin registries configured.")
				return nil
			}
			for _, source := range sources {
				cmd.Printf("%s\t%s\t%s\n", source.Name, source.Trust, source.URL)
			}
			return nil
		},
	}
}

func newRegistryRemoveCmd(service *registry.CustomRegistryService) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom registry and its trust confirmation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ax.Perform(cmd.Context(), func(context.Context) error {
				cmd.Printf("Would remove plugin registry %s\n", args[0])
				return nil
			}, func(ctx context.Context) error {
				if err := service.Remove(ctx, args[0]); err != nil {
					return err
				}
				cmd.Printf("Removed plugin registry %s\n", args[0])
				return nil
			})
		},
	}
}

func newRegistryInstallCmd(service *registry.CustomRegistryService) *cobra.Command {
	var confirm, version string
	cmd := &cobra.Command{
		Use:   "install <registry> <plugin>",
		Short: "Install a plugin from a custom registry with checksum verification",
		Args:  cobra.ExactArgs(registryCommandPair),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if secureErr := registry.CheckSecurePlugins(false); secureErr != nil {
				return toValidationError(ctx, secureErr)
			}
			plugin, err := service.Resolve(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			return ax.Perform(ctx, func(context.Context) error {
				cmd.Printf("Would install %s from %s (%s) with checksum verification\n",
					plugin.Entry.Name, plugin.Source.Name, plugin.Entry.Repository)
				return nil
			}, func(ctx context.Context) error {
				result, installErr := service.Install(
					ctx, registry.NewInstaller(config.New().PluginDir), plugin, version, confirm,
					registry.InstallOptions{}, func(line string) { cmd.Println(line) },
				)
				if installErr != nil {
					return fmt.Errorf("installing from custom registry: %w", installErr)
				}
				displayInstallResult(cmd, result)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Release version (default: latest)")
	cmd.Flags().StringVar(&confirm, "confirm-registry", "", "Type the registry name to authorize its first install")
	return cmd
}
