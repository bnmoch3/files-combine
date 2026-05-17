# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## What This Is

`files-combine` is a CLI tool that aggregates multiple files into a single
document (Markdown or XML) for feeding to LLMs. Inspired by simonw's
files-to-prompt. Module path: `github.com/bnmoch3/files-combine`.

## Commands

```bash
# Build
go build -o files-combine ./cmd/files-combine/

# Install
go install github.com/bnmoch3/files-combine/cmd/files-combine@latest

# Test
go test ./...

# Vet / format
go vet ./...
go fmt ./...
```

No Makefile. No test files currently exist.

## Architecture

Three source files:

- **`gather.go`** — file discovery and content reading. Uses a 3-stage
  concurrent pipeline: `walkFiles()` (walks tree, respects .gitignore via
  go-git) → `processFile()` workers (one per CPU) → `merge()` (fan-in). Produces
  `[]FileResult`.
- **`combine.go`** — output formatting. `Combine()` dispatches to
  `combineAsMarkdown()` or `combineAsXML()`. Markdown format handles nested
  backtick fences by choosing a longer fence string. Maps file extensions to
  language identifiers for syntax highlighting.
- **`cmd/files-combine/main.go`** — Cobra CLI. `normalizeOutputFileAndFormat()`
  resolves output format from file extension or `--format` flag, with conflict
  detection.

## Key Design Details

- Output format is inferred from the `-o` filename extension (`.md` → markdown,
  `.xml` → xml) and reconciled against an explicit `--format` flag — conflicts
  are an error.
- `.gitignore` is parsed with `go-git/go-git/v5` and applied by default; disable
  with `--ignore-gitignore`.
- Cancellation propagates via a `done` channel passed through the pipeline
  stages.
- Current version: `1.0.2` (set in `cmd/files-combine/main.go`).
