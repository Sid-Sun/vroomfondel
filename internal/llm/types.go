package llm

// ChatMessage is a single role/content entry in a chat completion request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionDelta is one streamed chunk: the incremental text (not the
// cumulative transcript). Done marks the terminal chunk.
type CompletionDelta struct {
	ContentDelta   string
	ReasoningDelta string
	Done           bool
}

// EmitFunc receives streamed deltas in order.
type EmitFunc func(CompletionDelta)

// ModelOptions mirrors the shared request options.
type ModelOptions struct {
	Model           string `json:"model"`
	Stream          bool   `json:"stream"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// BasicModelTweaks is the subset sent for tweak_level != "advanced".
type BasicModelTweaks struct {
	ContextLength int     `json:"context_length"`
	MaxTokens     int     `json:"max_tokens"`
	Temperature   float64 `json:"temperature"`
}

// ModelTweaks is the full tweak set sent for tweak_level == "advanced".
type ModelTweaks struct {
	ContextLength    int     `json:"context_length" mapstructure:"context_length"`
	MaxTokens        int     `json:"max_tokens" mapstructure:"max_tokens"`
	Temperature      float64 `json:"temperature" mapstructure:"temperature"`
	FrequencyPenalty float64 `json:"frequency_penalty" mapstructure:"frequency_penalty"`
	PresencePenalty  float64 `json:"presence_penalty" mapstructure:"presence_penalty"`
	RepeatPenalty    float64 `json:"repeat_penalty" mapstructure:"repeat_penalty"`
}

type ChatCompletionPayloadMinimal struct {
	BasicModelTweaks
	ModelOptions
	Messages []ChatMessage `json:"messages"`
}

type ChatCompletionPayload struct {
	ModelOptions
	ModelTweaks
	Messages []ChatMessage `json:"messages"`
}

// ChatCompletionResponse is one SSE data payload from an OpenAI-compatible
// stream.
type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices"`
}

type ChatCompletionChoice struct {
	Delta struct {
		Content          string `json:"content"`
		ReasoningContent string `json:"reasoning_content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}
