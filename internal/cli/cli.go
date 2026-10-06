package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/sid-sun/vroomfondel/internal/config"
	"github.com/sid-sun/vroomfondel/internal/llm"
)

const usage = `vroomfondel - pipe stdin + prompt to an LLM, stream the response

Usage:
  vroomfondel [flags] [PROMPT...] < stdin
  echo "some code" | vroomfondel "review this"
  vroomfondel "write a haiku about pipes"
  cat main.go | vroomfondel -m local "explain this file"

Prompt:
  Positional args are joined with spaces to form the prompt. If stdin is
  piped, it is appended after the prompt (separated by a blank line). If no
  positional args are given, stdin alone is the prompt. One of the two is
  required.

Flags:
  -m, --model NAME    model entry from the config file (default "default")
  -s, --system TEXT   system prompt (default: model's system_prompt,
                      else "You are a friendly assistant")
  -c, --config PATH   config file (default ~/.vroomfondel.yaml,
                      fallback ~/.config/vroomfondel/{vroomfondel,config}.yaml)
  -h, --help          show this help and exit

Config:
  See example.yaml. Multiple models with per-model backend ("openai" for an
  OpenAI-compatible chat/completions endpoint, "ollama" for a native Ollama
  daemon) are supported.
`

// Run executes the CLI. args is os.Args[1:]. It returns the process exit code.
// Flag parsing is hand-rolled to support both -m and --model spellings
// without extra dependencies; unknown flags and missing values exit 2.
func Run(args []string, stdout, stderr io.Writer, stdin *os.File) int {
	var modelName = "default"
	var systemPrompt string
	var systemFlagSet bool
	var configFile string
	var showHelp bool
	var positional []string

	// Minimal hand-rolled parsing: supports -m/--model value, --model=value,
	// same for -s/--system and -c/--config, plus -h/--help. Unknown flags are
	// an error, mirroring flag.ContinueOnError behavior.
	i := 0
	for i < len(args) {
		a := args[i]
		name, value, hasValue := splitFlag(a)
		if name == "" {
			positional = append(positional, a)
			i++
			continue
		}
		if !hasValue {
			if name == "h" || name == "help" {
				showHelp = true
				i++
				continue
			}
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "flag -%s requires a value\n\n%s", name, usage)
				return 2
			}
			value = args[i+1]
			i += 2
		} else {
			i++
		}
		switch name {
		case "m", "model":
			modelName = value
		case "s", "system":
			systemPrompt = value
			systemFlagSet = true
		case "c", "config":
			configFile = value
		case "h", "help":
			showHelp = true
		default:
			fmt.Fprintf(stderr, "unknown flag -%s\n\n%s", name, usage)
			return 2
		}
	}

	if showHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}

	prompt := strings.Join(positional, " ")
	stdinText, err := readPipedStdin(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "failed to read stdin: %v\n", err)
		return 1
	}
	combined := combinePrompt(prompt, stdinText)
	if strings.TrimSpace(combined) == "" {
		fmt.Fprintf(stderr, "no prompt: provide positional args, piped stdin, or both\n\n%s", usage)
		return 2
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load config: %v\n", err)
		return 1
	}
	model, ok := cfg.Models[modelName]
	if !ok {
		fmt.Fprintf(stderr, "unknown model %q (available: %s)\n", modelName, strings.Join(cfg.ModelNames, ", "))
		return 1
	}

	// Precedence: -s/--system flag > per-model system_prompt in config >
	// built-in default. The flag wins even when empty so it can clear a
	// config value; an empty final prompt omits the system message.
	if !systemFlagSet {
		systemPrompt = model.SystemPrompt
	}
	if systemPrompt == "" && !systemFlagSet {
		systemPrompt = "You are a friendly assistant"
	}

	messages := []llm.ChatMessage{
		{Role: "user", Content: combined},
	}
	if systemPrompt != "" {
		messages = []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: combined},
		}
	}
	endpoints := llm.Endpoints{
		OpenAIEndpoint: cfg.OpenAIAPI.Endpoint,
		OpenAIAPIKey:   cfg.OpenAIAPI.APIKey,
		OllamaEndpoint: cfg.OllamaAPI.Endpoint,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	wroteAny := false
	trailingNewline := false
	streamErr := llm.StreamChat(ctx, model.ToRef(), endpoints, messages, func(d llm.CompletionDelta) {
		if d.ContentDelta != "" {
			wroteAny = true
			trailingNewline = strings.HasSuffix(d.ContentDelta, "\n")
			fmt.Fprint(stdout, d.ContentDelta)
		}
	})
	if streamErr != nil {
		if wroteAny {
			fmt.Fprintln(stderr)
		}
		fmt.Fprintf(stderr, "request failed: %v\n", streamErr)
		return 1
	}
	if wroteAny && !trailingNewline {
		fmt.Fprintln(stdout)
	}
	return 0
}

// splitFlag parses "-name value", "-name=value", "--name value",
// "--name=value". It returns ("", "", false) for non-flag args.
func splitFlag(a string) (name, value string, hasValue bool) {
	if len(a) < 2 || a[0] != '-' {
		return "", "", false
	}
	s := strings.TrimLeft(a, "-")
	if s == "" || s == "-" {
		return "", "", false
	}
	if idx := strings.Index(s, "="); idx >= 0 {
		return s[:idx], s[idx+1:], true
	}
	return s, "", false
}

// readPipedStdin returns stdin contents if stdin is piped/redirected, or ""
// if it is a terminal (no pipe). This lets `vroomfondel "prompt"` work with
// no stdin while `echo data | vroomfondel "prompt"` appends data.
func readPipedStdin(stdin *os.File) (string, error) {
	if stdin == nil {
		return "", nil
	}
	st, err := stdin.Stat()
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// combinePrompt joins the positional prompt and piped stdin. Stdin is appended
// after the prompt so `cat code | vroomfondel "review"` reads naturally.
func combinePrompt(prompt, stdinText string) string {
	prompt = strings.TrimSpace(prompt)
	stdinText = strings.TrimSpace(stdinText)
	switch {
	case prompt != "" && stdinText != "":
		return prompt + "\n\n" + stdinText
	case prompt != "":
		return prompt
	default:
		return stdinText
	}
}
