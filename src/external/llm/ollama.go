package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"jobagent/src/internal/domain"
)

type ollamaToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaOptions struct {
	NumCtx int `json:"num_ctx"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options"`
}

// Ollama returns far more than this; json.Unmarshal ignores what we omit.
type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
}

// OllamaLLM talks to a local Ollama server. No API key, no vendor SDK —
// just HTTP and JSON.
// Ollama defaults to a 4096-token context regardless of what the model
// supports, and silently drops the oldest messages past that — which deletes
// the system prompt and the user's request the moment a tool returns anything
// large.
const defaultNumCtx = 16384

type OllamaLLM struct {
	model   string
	baseURL string
	numCtx  int
	http    *http.Client
}

func NewOllamaLLM(model, baseURL string) *OllamaLLM {
	return &OllamaLLM{
		model:   model,
		baseURL: baseURL,
		numCtx:  defaultNumCtx,
		// Local generation of a long document can run many minutes.
		http: &http.Client{Timeout: 20 * time.Minute},
	}
}

func (o *OllamaLLM) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := o.post(ctx, ollamaRequest{
		Model: o.model,
		Messages: []ollamaMessage{
			{Role: string(domain.RoleSystem), Content: systemPrompt},
			{Role: string(domain.RoleUser), Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	if resp.Message.Content == "" {
		return "", fmt.Errorf("no content in ollama response")
	}
	return resp.Message.Content, nil
}

func (o *OllamaLLM) Chat(ctx context.Context, msgs []domain.Message, tools []domain.ToolDefinition) (domain.Message, error) {
	resp, err := o.post(ctx, ollamaRequest{
		Model:    o.model,
		Messages: toOllamaMessages(msgs),
		Tools:    toOllamaTools(tools),
	})
	if err != nil {
		return domain.Message{}, err
	}
	return fromOllamaMessage(resp.Message), nil
}

// post is the one place that speaks HTTP; Complete and Chat differ only in
// what they put in the request and how they read the reply.
func (o *OllamaLLM) post(ctx context.Context, payload ollamaRequest) (*ollamaResponse, error) {
	payload.Stream = false
	payload.Options = ollamaOptions{NumCtx: o.numCtx}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("ollama returned %s: %s", resp.Status, msg)
	}

	var decoded ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &decoded, nil
}

func toOllamaMessages(msgs []domain.Message) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(msgs))
	for _, m := range msgs {
		om := ollamaMessage{Role: string(m.Role), Content: m.Content}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaToolCallFunction{Name: tc.Name, Arguments: tc.Args},
			})
		}
		out = append(out, om)
	}
	return out
}

func toOllamaTools(tools []domain.ToolDefinition) []ollamaTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ollamaTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		})
	}
	return out
}

func fromOllamaMessage(m ollamaMessage) domain.Message {
	out := domain.Message{Role: domain.Role(m.Role), Content: m.Content}
	for i, tc := range m.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, domain.ToolCall{
			// Ollama does not assign call IDs; synthesize one so the domain
			// can correlate results the way other vendors require.
			ID:   fmt.Sprintf("call_%d", i),
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}
	return out
}
