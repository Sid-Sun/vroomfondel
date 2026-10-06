package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// openAIBackend talks to an OpenAI-compatible chat/completions endpoint
// (OpenAI itself or a proxy such as OpenWebUI).
type openAIBackend struct {
	model    ModelRef
	endpoint string
	apiKey   string
}

func (b openAIBackend) StreamChat(ctx context.Context, messages []ChatMessage, emit EmitFunc) error {
	var payload any
	if b.model.MinimalTweaks {
		payload = ChatCompletionPayloadMinimal{
			ModelOptions:     ModelOptions{Model: b.model.Model, Stream: true, ReasoningEffort: b.model.ReasoningEffort},
			Messages:         messages,
			BasicModelTweaks: b.model.Basic,
		}
	} else {
		payload = ChatCompletionPayload{
			ModelOptions: ModelOptions{Model: b.model.Model, Stream: true, ReasoningEffort: b.model.ReasoningEffort},
			Messages:     messages,
			ModelTweaks:  b.model.Full,
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := strings.TrimRight(b.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	if b.apiKey != "" {
		req.Header.Add("Authorization", "Bearer "+b.apiKey)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		head, _ := bufio.NewReader(res.Body).ReadString('\n')
		return fmt.Errorf("backend returned %s: %s", res.Status, strings.TrimSpace(head))
	}

	scanner := bufio.NewScanner(res.Body)
	// SSE lines are small, but a single max-token chunk could be large;
	// allow up to 1MiB per line.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawDone := false
	for scanner.Scan() {
		data := scanner.Text()

		dataFields := strings.SplitN(data, "data: ", 2)
		if len(dataFields) == 1 {
			// Skip lines without a data payload (blank lines, SSE comments).
			continue
		}

		// The stream is terminated by a literal "data: [DONE]" line, which is
		// not JSON and carries no choices.
		if strings.TrimSpace(dataFields[1]) == "[DONE]" {
			sawDone = true
			break
		}

		var resp ChatCompletionResponse
		if err := json.Unmarshal([]byte(dataFields[1]), &resp); err != nil {
			continue
		}
		if len(resp.Choices) == 0 {
			continue
		}

		choice := resp.Choices[0]
		// finish_reason is only set on the terminal chunk, but it isn't always
		// "stop" (e.g. "length" when max_tokens is hit) - any non-empty value
		// means this is the last chunk.
		isLast := choice.FinishReason != ""
		if !isLast && choice.Delta.Content == "" && choice.Delta.ReasoningContent == "" {
			continue
		}
		emit(CompletionDelta{
			ContentDelta:   choice.Delta.Content,
			ReasoningDelta: choice.Delta.ReasoningContent,
			Done:           isLast,
		})
		if isLast {
			sawDone = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	if !sawDone {
		// Stream ended without a terminal chunk - emit Done so the consumer
		// isn't left waiting for more.
		emit(CompletionDelta{Done: true})
	}
	return nil
}
