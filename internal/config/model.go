package config

import "github.com/sid-sun/vroomfondel/internal/llm"

var defaultModel = Model{
	Name:       "default",
	Model:      "llama3:instruct",
	Backend:    "openai",
	TweakLevel: "basic",
	Tweaks: llm.ModelTweaks{
		ContextLength:    8192,
		MaxTokens:        1024,
		Temperature:      0.8,
		RepeatPenalty:    1.2,
		PresencePenalty:  1.5,
		FrequencyPenalty: 1.0,
	},
}

type Model struct {
	Name  string `mapstructure:"name"`
	Model string `mapstructure:"model"`
	// Backend selects which completion backend serves this model: "openai"
	// (OpenAI-compatible chat/completions endpoint) or "ollama" (native
	// Ollama daemon via POST /api/chat). Empty defaults to "openai".
	Backend string `mapstructure:"backend"`
	// TweakLevel controls which tweaks are sent: any value but "advanced"
	// sends only the basic subset (context_length, max_tokens, temperature).
	TweakLevel string          `mapstructure:"tweak_level"`
	Tweaks     llm.ModelTweaks `mapstructure:"tweaks"`
	// ReasoningEffort is passed through verbatim as "reasoning_effort" to the
	// backend for models that support toggling/tuning extended thinking (e.g.
	// "none" to disable thinking, or "low"/"medium"/"high"). Leave empty to
	// omit the field and let the backend/model use its own default.
	ReasoningEffort string `mapstructure:"reasoning_effort"`
}

func (m Model) UseMinimalTweaks() bool {
	return m.TweakLevel != "advanced"
}

// Normalize applies defaults and validates the model entry. An empty Backend
// means "openai" for backward compatibility with configs written before the
// backend option existed.
func (m Model) Normalize() (Model, error) {
	if m.Backend == "" {
		m.Backend = "openai"
	}
	if m.Backend != "openai" && m.Backend != "ollama" {
		return m, errInvalidBackend(m.Name)
	}
	if m.TweakLevel == "" {
		m.TweakLevel = "basic"
	}
	return m, nil
}

func (m Model) GetAdvancedTweaks() llm.ModelTweaks {
	return m.Tweaks
}

func (m Model) GetBasicTweaks() llm.BasicModelTweaks {
	return llm.BasicModelTweaks{
		ContextLength: m.Tweaks.ContextLength,
		MaxTokens:     m.Tweaks.MaxTokens,
		Temperature:   m.Tweaks.Temperature,
	}
}

// ToRef converts the config model into the backend-agnostic llm.ModelRef,
// breaking the config -> llm import cycle in the other direction.
func (m Model) ToRef() llm.ModelRef {
	return llm.ModelRef{
		Name:            m.Name,
		Model:           m.Model,
		Backend:         m.Backend,
		MinimalTweaks:   m.UseMinimalTweaks(),
		Basic:           m.GetBasicTweaks(),
		Full:            m.GetAdvancedTweaks(),
		ReasoningEffort: m.ReasoningEffort,
	}
}
