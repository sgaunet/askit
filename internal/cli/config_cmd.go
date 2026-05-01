package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sgaunet/askit/internal/config"
)

// newConfigCommand replaces newConfigStub with the fully wired config
// inspection command: default form prints resolved YAML; --path prints the
// resolved config file path; --explain prints a provenance-aware table.
func newConfigCommand(g *Globals) *cobra.Command {
	var showPath, showExplain bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the resolved configuration",
		Long: "Load the same configuration askit would use for other subcommands " +
			"and print it. Issues no network request. Useful for debugging precedence.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showPath && showExplain {
				return NewUsageErr("--path and --explain are mutually exclusive")
			}
			return runConfig(g, showPath, showExplain, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&showPath, "path", false, "print the resolved config file path (or <builtins>)")
	cmd.Flags().BoolVar(&showExplain, "explain", false, "print each field with its resolved value and source")
	return cmd
}

func runConfig(g *Globals, showPath, showExplain bool, out io.Writer) error {
	// Don't run validation here — the user may have an intentionally
	// partial config to inspect. Call Merge directly instead of Load.
	res, err := loadConfigForInspection(g)
	if err != nil {
		return err
	}

	switch {
	case showPath:
		if res.ResolvedPath == "" {
			_, _ = fmt.Fprintln(out, "<builtins>")
		} else {
			_, _ = fmt.Fprintln(out, res.ResolvedPath)
		}
		return nil
	case showExplain:
		return emitExplain(out, res.Config, res.Provenance)
	default:
		return emitYAML(out, res.Config)
	}
}

const (
	yamlIndent    = 2 // standard YAML indentation for enc.SetIndent
	tabwriterPad  = 2 // minimum cell padding in the explain table
)

func emitYAML(out io.Writer, cfg *config.Config) error {
	enc := yaml.NewEncoder(out)
	enc.SetIndent(yamlIndent)
	defer func() { _ = enc.Close() }()
	//nolint:gosec // G117: APIKey is intentionally shown in config output so the user can verify it is set correctly.
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	return nil
}

func emitExplain(out io.Writer, cfg *config.Config, prov config.Provenance) error {
	lines, err := config.Explain(cfg, prov)
	if err != nil {
		return fmt.Errorf("explain: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, tabwriterPad, ' ', 0)
	_, _ = fmt.Fprintln(tw, "FIELD\tVALUE\tSOURCE")
	for _, l := range lines {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", l.Field, l.Value, l.Source)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush tabwriter: %w", err)
	}
	return nil
}

// loadConfigForInspection is like loadConfig but skips the validation gate
// so config --explain can show a partial configuration the user is
// actively debugging.
func loadConfigForInspection(g *Globals) (*config.Result, error) {
	envOv := g.EnvOverrides()
	flagOv := g.FlagOverrides()

	var files []config.FileLayer

	defaultPath, derr := config.DefaultConfigPath()
	if derr == nil {
		partial, err := config.LoadFile(defaultPath)
		if err == nil {
			files = append(files, config.FileLayer{Partial: partial, Source: config.SourceDefaultFile})
		}
	}
	resolvedPath := ""
	if g.ConfigPath != "" {
		resolvedPath = g.ConfigPath
		src := config.SourceExplicitFile
		if !g.configPathFromFlag {
			src = config.SourceEnv
		}
		partial, err := config.LoadFile(g.ConfigPath)
		if err != nil {
			return nil, categorizeConfigError(err)
		}
		files = append(files, config.FileLayer{Partial: partial, Source: src})
	} else if derr == nil && len(files) > 0 {
		resolvedPath = defaultPath
	}

	cfg, prov, err := config.Merge(files, envOv, flagOv)
	if err != nil {
		return nil, categorizeConfigError(err)
	}
	return &config.Result{
		Config:         cfg,
		Provenance:     prov,
		ResolvedPath:   resolvedPath,
		LoadedFromPath: resolvedPath != "",
	}, nil
}
