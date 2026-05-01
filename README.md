# askit

[![linter](https://github.com/sgaunet/askit/actions/workflows/linter.yml/badge.svg)](https://github.com/sgaunet/askit/actions/workflows/linter.yml)
[![snapshot](https://github.com/sgaunet/askit/actions/workflows/snapshot.yml/badge.svg)](https://github.com/sgaunet/askit/actions/workflows/snapshot.yml)
[![release](https://github.com/sgaunet/askit/actions/workflows/release.yml/badge.svg)](https://github.com/sgaunet/askit/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sgaunet/askit)](https://goreportcard.com/report/github.com/sgaunet/askit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A terminal client for OpenAI-compatible chat-completion APIs (LM Studio,
vLLM, llama.cpp server, Ollama's OpenAI endpoint, OpenAI itself), built
around three ideas:

- **Inline `@path` file references** in prompts with an interactive file
  picker triggered by typing `@`.
- **Multiple independent config files** — a default at
  `~/.config/askit/config.yml` and any number of others selected with `-c`.
- **Named presets** — reusable bundles of system prompt, sampling
  parameters, and output format for repeated workflows (OCR runs being the
  motivating example).

Optimised for the *"extract text from a scan, save it to a file, move on"*
loop, but stays general enough to be a daily driver for any
OpenAI-compatible endpoint.

> **Status**: v0.1.x in development.

## Install

```sh
# Homebrew (Linux/macOS)
brew install sgaunet/tools/askit

# go install (requires Go 1.25+)
go install github.com/sgaunet/askit/cmd/askit@latest
```

Release binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
and windows/amd64 are published on every tagged release.

## Quick start

Create `~/.config/askit/config.yml` pointing at your endpoint, then:

```sh
# One-shot OCR from an image to stdout
askit query -p ocr-plain "@./scan.png"

# Save to a file (refuses to overwrite without --force)
askit query -p ocr-plain -o scan.txt "@./scan.png"

# Interactive session with a file picker
askit chat -p ocr-md

# Inspect how askit resolved configuration
askit config --explain

# List the models the endpoint advertises
askit models
```

## Features

- One-shot (`query`) and interactive (`chat`) subcommands.
- Inline `@path` file references: `@./scan.png`, `` @`my scan.png` ``,
  `@blob.dat:text`, `\@` escape.
- Three output formats: plain (streams), structured JSON envelope, raw
  upstream response.
- Bounded retries with full-jitter exponential backoff on 429 / transient
  5xx; honors `Retry-After`.
- Split timeouts: connect+TTFB and stream-idle — a slow-but-progressing
  OCR stream is never killed by a wall-clock deadline.
- Atomic file writes (`-o`): refuses to overwrite without `--force`, and
  writes go through a sibling temp file + rename so a crashed run never
  leaves a half-written output in place of an older valid one.
- API key redacted to `***` in every diagnostic output path
  (`--dry-run`, `-v`, `-vv`).
- Stable exit-code taxonomy (0 success; 2 usage; 3 config; 4 file;
  5 network; 6 API; 7 timeout) for shell-script branching.
- Hand-rolled typed config with a provenance-aware
  `askit config --explain` for debugging precedence.

## Platform support

- **CLI** (`query`, `config`, `models`): Linux (amd64/arm64), macOS
  (amd64/arm64), Windows (amd64). All automated-tested in CI.
- **TUI** (`chat`): Linux and macOS are tier-1 with pty-based automated
  tests. Windows TUI is **best-effort, manually smoke-tested only** —
  the `creack/pty` dependency has no Windows support, so interactive
  behavior on Windows may regress between releases.

## Development

```sh
task --list          # show all available tasks
task lint            # golangci-lint
task test            # unit tests
task test:race       # race detector
task test:cover      # coverage profile
task build           # build into ./dist/
task snapshot        # goreleaser snapshot
```

Install [pre-commit](https://pre-commit.com/) hooks:

```sh
pre-commit install
```

## License

[MIT](LICENSE) — © 2026 Sylvain.
