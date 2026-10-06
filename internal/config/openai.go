package config

// OpenAI holds the config for an OpenAI-compatible chat/completions
// endpoint (OpenAI itself or a proxy such as OpenWebUI).
type OpenAI struct {
	Endpoint string
	APIKey   string
}
