package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sgaunet/askit/internal/version"
)

// Execute parses argv and dispatches to the matching subcommand, returning
// the ExitCode the process should surface. Centralizing the error→exit
// mapping here keeps cmd/askit/main.go a one-liner.
func Execute(args []string) ExitCode {
	root, globals := newRootCommand()
	root.SetArgs(args)

	err := root.Execute()
	if err != nil {
		// Cobra returns plain errors for flag-parsing failures; classify
		// them as usage-level (exit 2) unless already tagged.
		if CategoryOf(err) == CatGeneric {
			err = WrapCategorized(CatUsage, ExitUsage, err)
		}
		FormatError(os.Stderr, err, globals.Verbose)
		return CodeOf(err)
	}
	return ExitOK
}

// newRootCommand builds the cobra root plus all direct subcommands. The
// returned [Globals] is populated at PersistentPreRun time so subcommands
// can read a consistent view.
func newRootCommand() (*cobra.Command, *Globals) {
	g := &Globals{}

	root := &cobra.Command{
		Use:   "askit",
		Short: "Terminal client for OpenAI-compatible chat completion APIs",
		Long: "askit is a terminal client for OpenAI-compatible chat " +
			"completion APIs with @path file references, multi-config support, " +
			"and named presets.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&g.ConfigPath, "config", "c", "",
		"path to config file (env: ASKIT_CONFIG; default: ~/.config/askit/config.yml)")
	root.PersistentFlags().StringVarP(&g.Endpoint, "endpoint", "e", "",
		"override endpoint base URL (env: ASKIT_ENDPOINT)")
	root.PersistentFlags().StringVarP(&g.Model, "model", "m", "",
		"override model (env: ASKIT_MODEL)")
	root.PersistentFlags().StringVar(&g.APIKey, "api-key", "",
		"override API key (env: ASKIT_API_KEY; discouraged — visible in ps, prefer ASKIT_API_KEY)")
	root.PersistentFlags().BoolVar(&g.NoColor, "no-color", false,
		"disable ANSI styling (env: NO_COLOR)")
	root.PersistentFlags().CountVarP(&g.Verbose, "verbose", "v",
		"increase logging verbosity (stackable: -v request summary, -vv full request/response)")

	showVersion := false
	root.Flags().BoolVar(&showVersion, "version", false, "print version and build info, then exit")

	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if showVersion {
			fmt.Fprintln(cmd.OutOrStdout(), version.Info())
			return nil
		}
		return cmd.Help()
	}

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Always read from the root command's persistent flags so
		// subcommands that inherit them still report correctly. See
		// cobra docs: persistent flags are parsed on the parent, but
		// PersistentPreRunE receives the INVOKED subcommand as `cmd`.
		rootFlags := cmd.Root().PersistentFlags()
		g.configPathFromFlag = rootFlags.Changed("config")
		g.endpointFromFlag = rootFlags.Changed("endpoint")
		g.modelFromFlag = rootFlags.Changed("model")
		g.apiKeyFromFlag = rootFlags.Changed("api-key")
		g.mergeEnvFallbacks()
		return nil
	}

	root.AddCommand(newQueryCommand(g))
	root.AddCommand(newChatCommand(g))
	root.AddCommand(newConfigCommand(g))
	root.AddCommand(newModelsCommand(g))

	return root, g
}

