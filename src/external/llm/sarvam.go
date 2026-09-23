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

// Sarvam speaks the OpenAI chat-completions shape, which differs from Ollama's
// in one easy-to-miss way: tool arguments arrive as a JSON *string* rather than
// an object.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openAITool struct {
	Type     string            `json:"type"`
	Function openAIFunctionDef `json:"function"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Tools    []openAITool    `json:"tools,omitempty"`
	// ReasoningEffort matters more than it looks: at the default, sarvam-105b
	// can spend an entire 16k output budget thinking and return nothing.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTokens       int64  `json:"max_tokens,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type SarvamLLM struct {
	model     string
	apiKey    string
	baseURL   string
	effort    string
	maxTokens int64
	http      *http.Client
}

func NewSarvamLLM(model, apiKey, baseURL, effort string, maxTokens int64) *SarvamLLM {
	return &SarvamLLM{
		model:     model,
		apiKey:    apiKey,
		baseURL:   baseURL,
		effort:    effort,
		maxTokens: maxTokens,
		http:      &http.Client{Timeout: 10 * time.Minute},
	}
}

func (s *SarvamLLM) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	msg, err := s.post(ctx, openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{Role: string(domain.RoleSystem), Content: systemPrompt},
			{Role: string(domain.RoleUser), Content: userPrompt},
		},
		MaxTokens: s.maxTokens,
	})
	if err != nil {
		return "", err
	}
	if msg.Content == "" {
		return "", fmt.Errorf("no content in sarvam response")
	}
	return msg.Content, nil
}

func (s *SarvamLLM) Chat(ctx context.Context, msgs []domain.Message, tools []domain.ToolDefinition) (domain.Message, error) {
	msg, err := s.post(ctx, openAIRequest{
		Model:     s.model,
		Messages:  toOpenAIMessages(msgs),
		Tools:     toOpenAITools(tools),
		MaxTokens: s.maxTokens,
	})
	if err != nil {
		return domain.Message{}, err
	}
	return fromOpenAIMessage(msg)
}

func (s *SarvamLLM) post(ctx context.Context, payload openAIRequest) (openAIMessage, error) {
	var empty openAIMessage
	payload.ReasoningEffort = s.effort

	body, err := json.Marshal(payload)
	if err != nil {
		return empty, fmt.Errorf("encode request: %w", err)
	}

	// v2 exists but is beta-gated per account; v1 serves sarvam-105b and
	// supports tools.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return empty, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-subscription-key", s.apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return empty, fmt.Errorf("call sarvam at %s: %w", s.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return empty, fmt.Errorf("sarvam returned %s: %s", resp.Status, detail)
	}

	var decoded openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return empty, fmt.Errorf("decode response: %w", err)
	}
	if decoded.Error != nil {
		return empty, fmt.Errorf("sarvam: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return empty, fmt.Errorf("sarvam returned no choices")
	}

	choice := decoded.Choices[0]
	if choice.FinishReason == "length" && choice.Message.Content == "" && len(choice.Message.ToolCalls) == 0 {
		return empty, fmt.Errorf("sarvam hit max_tokens (%d) before producing output — "+
			"lower SARVAM_REASONING_EFFORT or raise LLM_MAX_TOKENS", s.maxTokens)
	}
	return choice.Message, nil
}

func toOpenAIMessages(msgs []domain.Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(msgs))
	for _, m := range msgs {
		om := openAIMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			args := string(tc.Args)
			if args == "" {
				args = "{}"
			}
			om.ToolCalls = append(om.ToolCalls, openAIToolCall{
				ID:       tc.ID,
				Type:     "function",
				Function: openAIFunctionCall{Name: tc.Name, Arguments: args},
			})
		}
		out = append(out, om)
	}
	return out
}

func toOpenAITools(tools []domain.ToolDefinition) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		out = append(out, openAITool{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		})
	}
	return out
}

func fromOpenAIMessage(m openAIMessage) (domain.Message, error) {
	out := domain.Message{Role: domain.RoleAssistant, Content: m.Content}
	for _, tc := range m.ToolCalls {
		// Arguments is a JSON string here, not an object — validate it so a
		// malformed payload fails now rather than inside a tool.
		args := json.RawMessage(tc.Function.Arguments)
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		if !json.Valid(args) {
			return domain.Message{}, fmt.Errorf("tool call %s: arguments were not valid JSON: %s", tc.Function.Name, tc.Function.Arguments)
		}
		out.ToolCalls = append(out.ToolCalls, domain.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}
	return out, nil
}
