package cli

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sgaunet/askit/internal/client"
)

// newModelsCommand replaces newModelsStub with a real command that lists
// models advertised by the configured endpoint.
func newModelsCommand(g *Globals) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List models advertised by the endpoint",
		Long: "Issues GET /v1/models against the configured endpoint. " +
			"Default form prints one model ID per line (stable-sorted). " +
			"With --json, prints the upstream response body verbatim.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModels(cmd.Context(), g, asJSON, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the upstream /v1/models response body verbatim")
	return cmd
}

func runModels(ctx context.Context, g *Globals, asJSON bool, out io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := loadConfig(g)
	if err != nil {
		return err
	}
	cfg := res.Config
	hc := client.New(cfg.Endpoint, cfg.APIKey, cfg.Defaults.Timeout.AsDuration(), cfg.Defaults.StreamIdleTimeout.AsDuration())

	models, err := hc.ListModels(ctx)
	if err != nil {
		return categorizeTransportError(err)
	}

	if asJSON {
		_, _ = out.Write(models.Raw)
		// Ensure trailing newline for clean terminal / pipe behavior.
		if len(models.Raw) > 0 && models.Raw[len(models.Raw)-1] != '\n' {
			_, _ = out.Write([]byte("\n"))
		}
		return nil
	}

	ids := make([]string, 0, len(models.Data))
	for _, m := range models.Data {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		_, _ = fmt.Fprintln(out, id)
	}
	return nil
}
