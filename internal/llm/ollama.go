package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/ollama/ollama/api"
)

// ollamaBackend talks to a native Ollama daemon (POST /api/chat) via the
// official ollama/api client.
type ollamaBackend struct {
	model    ModelRef
	endpoint string
}

var (
	ollamaClientMu       sync.Mutex
	ollamaClient         *api.Client
	ollamaClientEndpoint string
)

func getOllamaClient(endpoint string) (*api.Client, error) {
	ollamaClientMu.Lock()
	defer ollamaClientMu.Unlock()
	if ollamaClient != nil && ollamaClientEndpoint == endpoint {
		return ollamaClient, nil
	}
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid ollama endpoint: %w", err)
	}
	ollamaClient = api.NewClient(base, http.DefaultClient)
	ollamaClientEndpoint = endpoint
	return ollamaClient, nil
}

// mapThink converts the model's reasoning_effort setting to Ollama's think
// parameter. Empty means "server default" (nil), "none" disables thinking.
func mapThink(reasoningEffort string) *api.ThinkValue {
	switch reasoningEffort {
	case "":
		return nil
	case "none":
		return &api.ThinkValue{Value: false}
	default:
		return &api.ThinkValue{Value: reasoningEffort}
	}
}

// mapOllamaOptions converts the model's tweaks to Ollama runtime options.
// context_length -> num_ctx and max_tokens -> num_predict; the extended
// penalties are only sent for tweak_level advanced.
func mapOllamaOptions(model ModelRef) map[string]any {
	var t ModelTweaks
	if model.MinimalTweaks {
		t = ModelTweaks{
			ContextLength: model.Basic.ContextLength,
			MaxTokens:     model.Basic.MaxTokens,
			Temperature:   model.Basic.Temperature,
		}
	} else {
		t = model.Full
	}
	opts := make(map[string]any)
	if t.Temperature != 0 {
		opts["temperature"] = t.Temperature
	}
	if t.ContextLength != 0 {
		opts["num_ctx"] = t.ContextLength
	}
	if t.MaxTokens != 0 {
		opts["num_predict"] = t.MaxTokens
	}
	if !model.MinimalTweaks {
		if t.RepeatPenalty != 0 {
			opts["repeat_penalty"] = t.RepeatPenalty
		}
		if t.PresencePenalty != 0 {
			opts["presence_penalty"] = t.PresencePenalty
		}
		if t.FrequencyPenalty != 0 {
			opts["frequency_penalty"] = t.FrequencyPenalty
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

func (b ollamaBackend) StreamChat(ctx context.Context, messages []ChatMessage, emit EmitFunc) error {
	if b.endpoint == "" {
		return fmt.Errorf("ollama endpoint is not configured")
	}
	client, err := getOllamaClient(b.endpoint)
	if err != nil {
		return err
	}

	apiMessages := make([]api.Message, len(messages))
	for i, m := range messages {
		apiMessages[i] = api.Message{Role: m.Role, Content: m.Content}
	}

	stream := true
	req := &api.ChatRequest{
		Model:    b.model.Model,
		Messages: apiMessages,
		Stream:   &stream,
		Think:    mapThink(b.model.ReasoningEffort),
		Options:  mapOllamaOptions(b.model),
	}

	sentDone := false
	err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
		isLast := resp.Done
		if !isLast && resp.Message.Content == "" && resp.Message.Thinking == "" {
			return nil
		}
		emit(CompletionDelta{
			ContentDelta:   resp.Message.Content,
			ReasoningDelta: resp.Message.Thinking,
			Done:           isLast,
		})
		if isLast {
			sentDone = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ollama chat request failed: %w", err)
	}
	if !sentDone {
		emit(CompletionDelta{Done: true})
	}
	return nil
}
