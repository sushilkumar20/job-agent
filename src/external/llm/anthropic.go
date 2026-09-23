package llm

import (
	"context"
	"errors"
	"fmt"
	"jobagent/src/internal/domain"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicLLM struct {
	model     string
	maxTokens int64
	client    anthropic.Client
}

func NewAnthropicLLM(model, apiKey string, maxTokens int64) *AnthropicLLM {
	return &AnthropicLLM{
		model:     model,
		maxTokens: maxTokens,
		client:    anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (a *AnthropicLLM) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {

	resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			out.WriteString(text.Text)
		}
	}

	if out.Len() == 0 {
		return "", fmt.Errorf("no text in response (stop_reason=%s)", resp.StopReason)
	}
	return out.String(), nil
}

// Chat is not implemented yet — Anthropic expresses tool use as typed content
// blocks, which needs its own translation layer.
func (a *AnthropicLLM) Chat(ctx context.Context, msgs []domain.Message, tools []domain.ToolDefinition) (domain.Message, error) {
	return domain.Message{}, errors.New("anthropic: Chat not implemented yet")
}
