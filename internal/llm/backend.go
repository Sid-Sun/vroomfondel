package llm

import (
	"context"
	"fmt"
)

// ModelRef carries everything a backend needs without importing the config
// package (which itself imports llm). Build it via config.Model.ToRef.
type ModelRef struct {
	Name            string
	Model           string
	Backend         string
	MinimalTweaks   bool
	Basic           BasicModelTweaks
	Full            ModelTweaks
	ReasoningEffort string
}

// Backend streams one chat completion for messages, calling emit once per
// delta chunk with a final Done=true delta.
type Backend interface {
	StreamChat(ctx context.Context, messages []ChatMessage, emit EmitFunc) error
}

// Endpoints groups the daemon addresses/keys the backends need.
type Endpoints struct {
	OpenAIEndpoint string
	OpenAIAPIKey   string
	OllamaEndpoint string
}

// NewBackend dispatches to the backend named by model.Backend ("openai" or
// "ollama"), mirroring openwebui-tg's newBackend.
func NewBackend(model ModelRef, endpoints Endpoints) (Backend, error) {
	switch model.Backend {
	case "", "openai":
		return openAIBackend{model: model, endpoint: endpoints.OpenAIEndpoint, apiKey: endpoints.OpenAIAPIKey}, nil
	case "ollama":
		return ollamaBackend{model: model, endpoint: endpoints.OllamaEndpoint}, nil
	default:
		return nil, fmt.Errorf("unknown backend %q for model %q", model.Backend, model.Name)
	}
}

// StreamChat is the single-shot convenience entrypoint: it dispatches on the
// model and streams until Done.
func StreamChat(ctx context.Context, model ModelRef, endpoints Endpoints, messages []ChatMessage, emit EmitFunc) error {
	backend, err := NewBackend(model, endpoints)
	if err != nil {
		return err
	}
	return backend.StreamChat(ctx, messages, emit)
}
