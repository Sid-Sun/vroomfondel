# AGENTS.md — vroomfondel contributor notes

## What this is

`vroomfondel` is a small Go CLI that pipes stdin + a positional prompt to an
LLM and streams the response to stdout. No TUI, single-shot per invocation,
no conversation history.

```
echo "some code" | vroomfondel "review this"
vroomfondel "write a haiku about pipes"
cat main.go | vroomfondel -m local "explain this file"
```

Backend logic is ported from `openwebui-tg/` (Telegram bot, separate module).
Only the LLM client portion was reused — none of the bot/store/router code.

## Layout

```
main.go                  # thin entry: cli.Run(os.Args[1:], stdout, stderr, stdin)
example.yaml             # reference config; copy to ~/.vroomfondel.yaml
internal/config/         # viper config loading (config.go, model.go, openai.go, ollama.go)
internal/llm/            # backends (types.go, backend.go, openai.go, ollama.go)
internal/cli/            # flag parsing, stdin+prompt assembly, streaming (cli.go)
openwebui-tg/            # SYMLINK to another repo/module — not part of this build
```

Module: `github.com/sid-sun/vroomfondel`, Go 1.24.1. Deps are intentionally
pinned to match `openwebui-tg`: `viper v1.18.2`, `ollama v0.12.9`.

## Build / verify

Scope Go commands to this module — `./...` follows the `openwebui-tg`
symlink into another module and breaks. Use explicit package lists:

```bash
gofmt -l main.go internal/
go vet ./internal/... .
go build -o /tmp/vroomfondel .
/tmp/vroomfondel --help
```

### Behavior contract (tested)

- Exit codes: `0` ok/help, `2` usage error (no prompt, unknown flag, missing
  flag value), `1` runtime error (config, unknown model, request failure).
- Prompt assembly: positional args joined with spaces; piped stdin appended
  after a blank line (`prompt + "\n\n" + stdin`); stdin-alone works; TTY
  stdin is ignored so bare `vroomfondel "prompt"` works (detected via
  `ModeCharDevice`).
- Output: incremental content deltas to stdout, single trailing newline
  added if missing; all errors/logs to stderr. Reasoning/thinking deltas are
  currently discarded by the CLI (still parsed).
- Flags support both spellings and `=`: `-m/--model`, `-s/--system`,
  `-c/--config`, `-h/--help`.

### Mock-server test pattern

No test suite exists yet. Manual streaming test used so far: a Python
`http.server` stub returning SSE chunks + `data: [DONE]`, with a temp config
pointing `openai.endpoint` at it, e.g. `--config /tmp/vf-test.yaml`. This
verified streaming, stdin+prompt combining (`"my prompt\n\npiped body"`),
`--system`, and `--config=`/`--model=` spellings.

## Config

Load order in `internal/config/config.go::Load`:
1. `--config PATH` if given,
2. `~/.vroomfondel.yaml` (primary),
3. fallback `~/.config/vroomfondel/{vroomfondel,config}.yaml`.

A model named `default` is required; per-model `backend` is `openai`
(OpenAI-compatible `POST {endpoint}chat/completions`, SSE) or `ollama`
(native daemon via `ollama/api`). `tweak_level: advanced` sends full tweaks,
anything else sends the basic subset. `reasoning_effort` passes through
(`reasoning_effort` on OpenAI, `think` on Ollama; `"none"` disables).

## Architecture notes

- Import direction is `config -> llm` (`config.Model` embeds `llm.ModelTweaks`).
  To avoid a cycle, backends take `llm.ModelRef`; convert via
  `config.Model.ToRef()`. Keep it that way.
- `llm.CompletionDelta{ContentDelta, ReasoningDelta, Done}` carries
  incremental (not cumulative) text — this differs from `openwebui-tg`,
  which accumulates cumulative transcripts for Telegram message edits.
- `internal/cli` uses hand-rolled flag parsing (no extra deps). If flags grow
  beyond the current four, consider switching to stdlib `flag` or `pflag`.
- `Ctrl-C` cancels via `signal.NotifyContext`.

## Gotchas / known issues

- **Symlink trap**: `openwebui-tg -> <other checkout>` is a separate module.
  Never run bare `go ./...`, `go fmt ./...`, or `go mod tidy` from root
  without scoping — tidy once emptied this module's `go.mod` (recovered via
  `go get` + `go mod tidy`, then verified with scoped commands).
- **Divergence from upstream**: `openwebui-tg`'s `Model.modelTweakLevel` is
  private so `tweak_level` from YAML never unmarshals (always minimal). Here
  it is public `TweakLevel` with a `"basic"` default in `Normalize()` — do
  not "fix" this back to match upstream.
- **Ollama backend is compile-checked only** (`go vet`); it has not been
  live-tested against a daemon. The OpenAI backend was tested against a mock
  SSE server only, not a real OpenWebUI endpoint.
- SSE scanner buffer is raised to 1 MiB per line in `internal/llm/openai.go`.

## Likely next steps (not started)

- `go test` coverage: `combinePrompt`, `splitFlag`, config `Normalize`,
  OpenAI SSE parsing against `httptest` (replacing the ad-hoc Python mocks).
- Surface reasoning output (`-r/--show-reasoning` to stderr or interleaved).
- Makefile / `go install` docs, non-200 error bodies (currently only first
  line is surfaced), `--stream=false` non-streaming mode.
- TUI was explicitly out of scope for the initial build.
