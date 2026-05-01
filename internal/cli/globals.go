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
	mergeEnvString(&g.ConfigPath, "ASKIT_CONFIG", &g.configPathFromFlag)
	mergeEnvString(&g.Endpoint, "ASKIT_ENDPOINT", &g.endpointFromFlag)
	mergeEnvString(&g.Model, "ASKIT_MODEL", &g.modelFromFlag)
	mergeEnvString(&g.APIKey, "ASKIT_API_KEY", &g.apiKeyFromFlag)
	if !g.NoColor && strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		g.NoColor = true
	}
}

// mergeEnvString copies the named env var into *dest when dest is empty,
// and clears the fromFlag marker to record that the value came from the env.
func mergeEnvString(dest *string, key string, fromFlag *bool) {
	if *dest == "" {
		if v := os.Getenv(key); v != "" {
			*dest = v
			*fromFlag = false
		}
	}
}
