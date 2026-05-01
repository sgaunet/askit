package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/prompt"
	"github.com/sgaunet/askit/internal/render"
	"github.com/sgaunet/askit/internal/version"
)

type queryFlags struct {
	preset      string
	files       []string
	system      string
	systemSet   bool
	temperature float64
	tempSet     bool
	topP        float64
	topPSet     bool
	maxTokens   int
	maxTokSet   bool
	seed        int64
	seedSet     bool
	output      string
	outputSet   bool
	out         string
	force       bool
	stream      bool
	streamSet   bool
	noStream    bool
	dryRun      bool
	timeout     time.Duration
	idleTimeout time.Duration
	retries     int
	retriesSet  bool
}

// newQueryCommand replaces newQueryStub with a fully wired query command.
func newQueryCommand(g *Globals) *cobra.Command {
	f := &queryFlags{}
	cmd := &cobra.Command{
		Use:   "query [PROMPT]",
		Short: "Run a one-shot chat completion",
		Long:  "Send a single prompt (with optional @path file references) and print the assistant's reply.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd.Context(), g, f, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVarP(&f.preset, "preset", "p", "", "apply a named preset")
	cmd.Flags().StringSliceVarP(&f.files, "file", "f", nil, "attach a file (repeatable)")
	cmd.Flags().StringVarP(&f.system, "system", "s", "", "override system prompt")
	cmd.Flags().Float64VarP(&f.temperature, "temperature", "t", 0, "sampling temperature")
	cmd.Flags().Float64Var(&f.topP, "top-p", 0, "nucleus sampling")
	cmd.Flags().IntVar(&f.maxTokens, "max-tokens", 0, "max completion tokens")
	cmd.Flags().Int64Var(&f.seed, "seed", 0, "deterministic seed (if backend honors it)")
	cmd.Flags().StringVarP(&f.out, "out", "o", "", "write output to FILE (refuses to overwrite without --force)")
	cmd.Flags().BoolVarP(&f.force, "force", "F", false, "overwrite existing output file")
	cmd.Flags().StringVar(&f.output, "output", "", "output format: plain | json | raw")
	cmd.Flags().BoolVar(&f.stream, "stream", false, "enable streaming")
	cmd.Flags().BoolVar(&f.noStream, "no-stream", false, "disable streaming")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "assemble and print the request (redacted) to stderr; skip the call")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 0, "connect + time-to-first-byte deadline (default from config)")
	cmd.Flags().DurationVar(&f.idleTimeout, "stream-idle-timeout", 0, "max silence between streamed chunks (default from config)")
	cmd.Flags().IntVar(&f.retries, "retries", -1, "retry budget for 429/transient-5xx (default from config)")

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		fl := cmd.Flags()
		f.systemSet = fl.Changed("system")
		f.tempSet = fl.Changed("temperature")
		f.topPSet = fl.Changed("top-p")
		f.maxTokSet = fl.Changed("max-tokens")
		f.seedSet = fl.Changed("seed")
		f.outputSet = fl.Changed("output")
		f.streamSet = fl.Changed("stream") || fl.Changed("no-stream")
		f.retriesSet = fl.Changed("retries")
		if fl.Changed("stream") && fl.Changed("no-stream") {
			return NewUsageErr("--stream and --no-stream are mutually exclusive")
		}
		if f.output != "" && !config.OutputFormat(f.output).Valid() {
			return NewUsageErr("--output must be one of plain | json | raw (got %q)", f.output)
		}
		return nil
	}
	return cmd
}

func runQuery(ctx context.Context, g *Globals, f *queryFlags, args []string, stdout, stderr io.Writer) error {
	res, err := loadConfig(g)
	if err != nil {
		return err
	}
	cfg := res.Config

	resolved, err := ResolvePreset(cfg, f.preset, buildPresetFlags(f))
	if err != nil {
		return err
	}

	userPrompt, refs, assembled, logger, err := preparePrompt(g, f, args, cfg, resolved)
	if err != nil {
		return err
	}

	clientReq, hc, retries := buildClientRequest(cfg, f, resolved, assembled)

	// Dry-run: print the redacted request and return.
	if f.dryRun {
		return emitDryRun(stderr, hc, clientReq, cfg.Endpoint, cfg.APIKey)
	}

	sink, atomic, err := openSink(f, stdout)
	if err != nil {
		return err
	}
	if atomic != nil {
		defer func() { _ = atomic.Close() }()
	}

	r, err := selectRenderer(f, resolved, clientReq, sink)
	if err != nil {
		return err
	}

	started := time.Now()
	finalResp, err := executeRequest(ctx, clientReq, hc, r, retries, logger)
	if err != nil {
		return err
	}
	return finalizeAndCommit(r, atomic, finalResp, resolved, cfg, userPrompt, refs, started)
}

func finalizeAndCommit(
	r render.Renderer,
	atomic *AtomicWriter,
	finalResp *client.Response,
	resolved ResolvedPreset,
	cfg *config.Config,
	userPrompt string,
	refs []prompt.FileRef,
	started time.Time,
) error {
	if err := r.Finalize(render.Meta{
		AskitVersion: version.Version,
		Model:        resolved.Model,
		Endpoint:     cfg.Endpoint,
		PresetName:   resolved.Name,
		System:       resolved.System,
		UserPrompt:   userPrompt,
		Inputs:       refs,
		StartedAt:    started,
		CompletedAt:  time.Now(),
		Duration:     time.Since(started),
		FinishReason: finalResp.FinishReason,
		Usage:        finalResp.Usage,
		RawBody:      finalResp.Raw,
		Text:         finalResp.Text,
	}); err != nil {
		return fmt.Errorf("render finalize: %w", err)
	}
	if atomic != nil {
		if err := atomic.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// preparePrompt reads the user prompt, assembles file references, and returns
// the assembled prompt alongside the resolved refs and logger.
func preparePrompt(
	g *Globals,
	f *queryFlags,
	args []string,
	cfg *config.Config,
	resolved ResolvedPreset,
) (string, []prompt.FileRef, *prompt.Prompt, *slog.Logger, error) {
	userPrompt, err := ReadPrompt(args, os.Stdin, StdinIsTTY())
	if err != nil {
		return "", nil, nil, nil, err
	}
	logger := NewLogger(g.Verbose)
	assembled, refs, err := prompt.Assemble(userPrompt, prompt.AssembleOptions{
		Policy:       cfg.FileReferences,
		SystemPrompt: resolved.System,
		ExtraFiles:   f.files,
		Logger:       logger,
	})
	if err != nil {
		return "", nil, nil, nil, categorizeAssembleError(err)
	}
	return userPrompt, refs, assembled, logger, nil
}

// buildClientRequest assembles the client.Request from resolved preset + flags.
func buildClientRequest(
	cfg *config.Config,
	f *queryFlags,
	resolved ResolvedPreset,
	assembled *prompt.Prompt,
) (*client.Request, *client.Client, int) {
	clientReq := &client.Request{
		Model:       resolved.Model,
		Prompt:      assembled,
		Temperature: resolved.Temperature,
		TopP:        resolved.TopP,
		MaxTokens:   resolved.MaxTokens,
		Seed:        resolved.Seed,
		Stream:      resolved.Stream,
	}
	connect := cfg.Defaults.Timeout.AsDuration()
	if f.timeout > 0 {
		connect = f.timeout
	}
	idle := cfg.Defaults.StreamIdleTimeout.AsDuration()
	if f.idleTimeout > 0 {
		idle = f.idleTimeout
	}
	retries := cfg.Defaults.Retries
	if f.retriesSet && f.retries >= 0 {
		retries = f.retries
	}
	return clientReq, client.New(cfg.Endpoint, cfg.APIKey, connect, idle), retries
}

// openSink opens the output sink (file or stdout). When a file is requested it
// returns an AtomicWriter; the caller is responsible for Commit/Close.
func openSink(f *queryFlags, stdout io.Writer) (io.Writer, *AtomicWriter, error) {
	if f.out == "" {
		return stdout, nil, nil
	}
	atomic, err := OpenOutput(f.out, f.force)
	if err != nil {
		return nil, nil, err
	}
	return atomic, atomic, nil
}

// selectRenderer chooses and configures the renderer based on the output
// format flag / preset. Also disables streaming for buffered formats.
//
//nolint:ireturn // render.Renderer is a required interface return; callers need Stream and Finalize.
func selectRenderer(
	f *queryFlags,
	resolved ResolvedPreset,
	clientReq *client.Request,
	sink io.Writer,
) (render.Renderer, error) {
	outputFormat := resolved.Output
	if f.outputSet {
		outputFormat = config.OutputFormat(f.output)
	}
	switch outputFormat {
	case config.OutputPlain:
		return &render.PlainRenderer{Out: sink}, nil
	case config.OutputJSON:
		clientReq.Stream = false
		return &render.JSONRenderer{Out: sink}, nil
	case config.OutputRaw:
		clientReq.Stream = false
		return &render.RawRenderer{Out: sink}, nil
	default:
		return nil, NewUsageErr("unknown output format %q", outputFormat)
	}
}

// executeRequest runs the retry loop and drains the response into r.
func executeRequest(
	ctx context.Context,
	clientReq *client.Request,
	hc *client.Client,
	r render.Renderer,
	retries int,
	logger *slog.Logger,
) (*client.Response, error) {
	retryOpts := client.DefaultRetryOptions()
	retryOpts.MaxAttempts = retries + 1
	retryOpts.Logger = logger

	if clientReq.Stream {
		return executeStreaming(ctx, retryOpts, hc, clientReq, r)
	}
	return executeBuffered(ctx, retryOpts, hc, clientReq, r)
}

func executeStreaming(
	ctx context.Context,
	opts client.RetryOptions,
	hc *client.Client,
	clientReq *client.Request,
	r render.Renderer,
) (*client.Response, error) {
	var finalResp *client.Response
	_, err := client.DoWithRetry(ctx, opts, func(ctx context.Context) (struct{}, error) {
		chunks, errs := hc.Stream(ctx, clientReq)
		resp, serr := drainAndRender(chunks, errs, r)
		if serr != nil {
			return struct{}{}, serr
		}
		finalResp = resp
		return struct{}{}, nil
	})
	if err != nil {
		return nil, categorizeTransportError(err)
	}
	return finalResp, nil
}

func executeBuffered(
	ctx context.Context,
	opts client.RetryOptions,
	hc *client.Client,
	clientReq *client.Request,
	r render.Renderer,
) (*client.Response, error) {
	finalResp, err := client.DoWithRetry(ctx, opts, func(ctx context.Context) (*client.Response, error) {
		return hc.Complete(ctx, clientReq)
	})
	if err != nil {
		return nil, categorizeTransportError(err)
	}
	if werr := r.Stream(finalResp.Text); werr != nil {
		return nil, fmt.Errorf("render stream: %w", werr)
	}
	return finalResp, nil
}

func drainAndRender(chunks <-chan client.StreamChunk, errs <-chan error, r render.Renderer) (*client.Response, error) {
	var sb []byte
	var finish string
	var usage client.Usage
	for chunk := range chunks {
		if chunk.Delta != "" {
			if werr := r.Stream(chunk.Delta); werr != nil {
				return nil, fmt.Errorf("render stream: %w", werr)
			}
			sb = append(sb, chunk.Delta...)
		}
		if chunk.FinishReason != "" {
			finish = chunk.FinishReason
		}
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
	}
	if err, ok := <-errs; ok && err != nil {
		return nil, err
	}
	return &client.Response{
		Text:         string(sb),
		FinishReason: finish,
		Usage:        usage,
	}, nil
}

func buildPresetFlags(f *queryFlags) PresetFlags {
	pf := PresetFlags{}
	if f.systemSet {
		s := f.system
		pf.System = &s
	}
	if f.tempSet {
		t := f.temperature
		pf.Temperature = &t
	}
	if f.topPSet {
		t := f.topP
		pf.TopP = &t
	}
	if f.maxTokSet {
		m := f.maxTokens
		pf.MaxTokens = &m
	}
	if f.seedSet {
		s := int(f.seed)
		pf.Seed = &s
	}
	if f.streamSet {
		b := f.stream && !f.noStream
		pf.Stream = &b
	}
	if f.outputSet {
		o := config.OutputFormat(f.output)
		pf.Output = &o
	}
	return pf
}

// loadConfig resolves the full Config given the globals. Kept here so both
// query and (future) chat/config/models paths share one implementation.
func loadConfig(g *Globals) (*config.Result, error) {
	envOv := g.EnvOverrides()
	flagOv := g.FlagOverrides()

	explicit := g.ConfigPath
	opts := config.LoadOptions{
		ExplicitPath:   explicit,
		ExplicitSource: config.SourceFlag,
		EnvOverrides:   envOv,
		FlagOverrides:  flagOv,
	}
	// If ASKIT_CONFIG supplied the path rather than -c, attribute it to env.
	if explicit != "" && !g.configPathFromFlag {
		opts.ExplicitSource = config.SourceEnv
	}
	res, err := config.Load(opts)
	if err != nil {
		return nil, categorizeConfigError(err)
	}
	return res, nil
}

func categorizeConfigError(err error) error {
	if err == nil {
		return nil
	}
	var vErr *config.ValidationError
	if errors.As(err, &vErr) {
		return NewConfigErr("%s", err.Error())
	}
	if errors.Is(err, config.ErrConfigMissing) {
		return NewConfigErr("%s", err.Error())
	}
	return NewConfigErr("%s", err.Error())
}

func categorizeAssembleError(err error) error {
	if err == nil {
		return nil
	}
	// Prompt-package errors are always file-class in US1 scope.
	return NewFileErr("%s", err.Error())
}

func categorizeTransportError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return &CategorizedError{Cat: CatAPI, Code: ExitAPI, Err: apiErr}
	}
	var timeoutErr *client.TimeoutError
	if errors.As(err, &timeoutErr) {
		return NewTimeoutErr("%s", err.Error())
	}
	var netErr *client.NetworkError
	if errors.As(err, &netErr) {
		return NewNetworkErr("%s", err.Error())
	}
	return NewNetworkErr("%s", err.Error())
}

// emitDryRun assembles the full HTTP request askit would have sent, redacts
// the API key, and prints a JSON representation to stderr. Never hits the
// network. Returns nil (exit 0) regardless of upstream readiness.
func emitDryRun(w io.Writer, _ *client.Client, r *client.Request, endpoint, apiKey string) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if apiKey != "" {
		headers["Authorization"] = "***" // always redacted per FR-092
	}
	type dryRunPayload struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Request *client.Request   `json:"request"`
	}
	//nolint:musttag // dryRunPayload has json tags on all fields; Request.Prompt is internal transport only.
	body, err := json.MarshalIndent(dryRunPayload{
		Method:  "POST",
		URL:     endpoint + "/chat/completions",
		Headers: headers,
		Request: r,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("dry-run marshal: %w", err)
	}
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n"))
	return nil
}
