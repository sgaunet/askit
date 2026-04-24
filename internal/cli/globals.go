package cli

import (
	"os"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// Globals holds the values of the global (root-level) flags plus the env
// vars that parallel them. Populated by [harvestGlobals] at
// PersistentPreRun time so that downstream subcommands all see the same
// resolved view.
type Globals struct {
	ConfigPath string // from -c or ASKIT_CONFIG
	Endpoint   string // from -e or ASKIT_ENDPOINT
	Model      string // from -m or ASKIT_MODEL
	APIKey     string // from --api-key or ASKIT_API_KEY
	NoColor    bool   // from --no-color or NO_COLOR
	Verbose    int    // from -v (stackable)

	// Source tracking — whether a value came from the flag or the env var.
	configPathFromFlag bool
	endpointFromFlag   bool
	modelFromFlag      bool
	apiKeyFromFlag     bool
}

// EnvOverrides returns the Overrides view of env-var values that should be
// layered after the explicit config file but before flag overrides.
func (g *Globals) EnvOverrides() config.Overrides {
	ov := config.Overrides{Source: config.SourceEnv}
	if g.Endpoint != "" && !g.endpointFromFlag {
		e := g.Endpoint
		ov.Endpoint = &e
	}
	if g.Model != "" && !g.modelFromFlag {
		m := g.Model
		ov.Model = &m
	}
	if g.APIKey != "" && !g.apiKeyFromFlag {
		k := g.APIKey
		ov.APIKey = &k
	}
	return ov
}

// FlagOverrides returns the Overrides view of flag-supplied values that
// should be layered last.
func (g *Globals) FlagOverrides() config.Overrides {
	ov := config.Overrides{Source: config.SourceFlag}
	if g.endpointFromFlag {
		e := g.Endpoint
		ov.Endpoint = &e
	}
	if g.modelFromFlag {
		m := g.Model
		ov.Model = &m
	}
	if g.apiKeyFromFlag {
		k := g.APIKey
		ov.APIKey = &k
	}
	return ov
}

// mergeEnvFallbacks fills any empty flag-supplied slot from the matching env var.
// Called before [EnvOverrides] / [FlagOverrides] are consulted.
func (g *Globals) mergeEnvFallbacks() {
	if g.ConfigPath == "" {
		if v := os.Getenv("ASKIT_CONFIG"); v != "" {
			g.ConfigPath = v
			g.configPathFromFlag = false
		}
	}
	if g.Endpoint == "" {
		if v := os.Getenv("ASKIT_ENDPOINT"); v != "" {
			g.Endpoint = v
			g.endpointFromFlag = false
		}
	}
	if g.Model == "" {
		if v := os.Getenv("ASKIT_MODEL"); v != "" {
			g.Model = v
			g.modelFromFlag = false
		}
	}
	if g.APIKey == "" {
		if v := os.Getenv("ASKIT_API_KEY"); v != "" {
			g.APIKey = v
			g.apiKeyFromFlag = false
		}
	}
	if !g.NoColor {
		if v := strings.TrimSpace(os.Getenv("NO_COLOR")); v != "" {
			g.NoColor = true
		}
	}
}
