# Vroomfondel

A tiny command-line tool for getting quick answers from LLMs, with rigidly
defined areas of doubt.

Each invocation is a single shot: pipe stdin and/or a positional prompt to a
model, and the response streams to stdout.

No TUI, no conversation history, no follow-ups. Those are the rigidly defined
areas of doubt.

Works with any OpenAI-compatible endpoint or an Ollama server.

## Why bother?

Zero context switches: ask without leaving the terminal.

Evaluate command output — redirect to a file or pipe directly:

```
root@eros ~# iostat -x 2 5 > iostat.out
root@eros ~# vroomfondel -m cloud "Is the output for iostat -x 2 5 on this system normal?" < iostat.out
The iostat output shows normal system behavior: low CPU utilization, minimal I/O wait, and device utilization under 5% for most disks. No signs of I/O bottlenecks or abnormal load. All metrics are within expected ranges for an idle or lightly used system. Output is normal.
root@eros ~#
root@eros ~# zpool status | vroomfondel -m cloud "Are my ZFS pools healthy?"
Both ZFS pools are healthy (ONLINE with no errors).
root@eros ~# zfs get all immich/immich | vroomfondel -m cloud "Should I tune anything here?"
Yes: set `special_small_blocks=4M` (immich stores many small files) and consider `recordsize=128K` or `256K` for media files.
root@eros ~#
```

Ask a quick knowledge question with no stdin:

```
root@eros ~ [0|1]# vroomfondel -m cloud "What do special_small_blocks and recordsize do in ZFS?"
**recordsize**: Sets the block size for regular files (default 128KB). Larger values improve throughput for large-file workloads; smaller values reduce wasted space for small files.

**special_small_blocks**: In special device pools, determines which small blocks (below this size threshold) get redirected to the special device. Set to 0 to disable, or a size like 512B/4K to offload small blocks to faster storage.

Both settings optimize performance for specific workload patterns.
root@eros ~# # P.S. I don't have special vdevs, the relevant executions did not have this context - time to buy some Intel Optanes, I guess.
```

## Install

```bash
go build -o vroomfondel .
```

Requires Go 1.24.1+.

## Config

Copy `example.yaml` to `~/.vroomfondel.yaml` and adjust. Alternatives, in
order of precedence:

1. `--config PATH` explicitly,
2. `~/.vroomfondel.yaml`,
3. `~/.config/vroomfondel/vroomfondel.yaml` or `~/.config/vroomfondel/config.yaml`.

A model named `default` is required. Each model picks a `backend`: `openai`
(OpenAI-compatible `POST {endpoint}chat/completions`, SSE) or `ollama`
(Ollama server, using the `ollama/api` Go client).

## Usage

```
vroomfondel [flags] [PROMPT...] < stdin
```

Positional args are joined with spaces to form the prompt. Piped stdin is
appended after the prompt, separated by a blank line. With no positional
args, stdin alone is the prompt. One of the two is required. TTY stdin is
ignored, so bare `vroomfondel "prompt"` works.

Answers are extremely brief by default. Override per-invocation with
`-s/--system`, per-model with `system_prompt` in the config file
(`-s ""` sends no system message).

| Flag           | Description                                            |
| -------------- | ------------------------------------------------------ |
| `-m, --model`  | model entry from the config file (default `"default"`) |
| `-s, --system` | system prompt override                                 |
| `-c, --config` | config file path                                       |
| `-h, --help`   | show help and exit                                     |

Both `-m value` and `--model=value` spellings work. Exit codes: `0` ok/help,
`2` usage error (no prompt, unknown flag, missing flag value), `1` runtime
error (config, unknown model, request failure). `Ctrl-C` cancels.
