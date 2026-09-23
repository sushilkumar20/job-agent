package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jobagent/src/internal/domain"

	"google.golang.org/genai"
)

type GeminiLLM struct {
	model     string
	maxTokens int64
	client    *genai.Client
}

// Unlike the Anthropic constructor, this one can fail — genai.NewClient
// resolves credentials eagerly.
func NewGeminiLLM(ctx context.Context, model, apiKey string, maxTokens int64) (*GeminiLLM, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	return &GeminiLLM{
		model:     model,
		maxTokens: maxTokens,
		client:    client,
	}, nil
}

func (g *GeminiLLM) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := g.generateWithRetry(
		ctx,
		[]*genai.Content{genai.NewContentFromText(userPrompt, genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
			MaxOutputTokens:   int32(g.maxTokens),
		},
	)
	if err != nil {
		return "", err
	}

	out := resp.Text()
	if out == "" {
		return "", fmt.Errorf("no text in gemini response")
	}
	return out, nil
}

func (g *GeminiLLM) Chat(ctx context.Context, msgs []domain.Message, tools []domain.ToolDefinition) (domain.Message, error) {
	contents, system, err := toGeminiContents(msgs)
	if err != nil {
		return domain.Message{}, err
	}

	cfg := &genai.GenerateContentConfig{MaxOutputTokens: int32(g.maxTokens)}
	if system != "" {
		cfg.SystemInstruction = genai.NewContentFromText(system, genai.RoleUser)
	}
	if decls := toGeminiTools(tools); len(decls) > 0 {
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: decls}}
	}

	resp, err := g.generateWithRetry(ctx, contents, cfg)
	if err != nil {
		return domain.Message{}, err
	}
	return fromGeminiResponse(resp)
}

const maxAttempts = 4

// generateWithRetry backs off on overload and quota errors. Without this a
// single transient 503 mid-loop aborts the run and throws away every step the
// agent had already completed.
func (g *GeminiLLM) generateWithRetry(ctx context.Context, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	var lastErr error

	for attempt := range maxAttempts {
		if attempt > 0 {
			delay := time.Duration(1<<attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
		if err == nil {
			return resp, nil
		}
		if !retryable(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("gave up after %d attempts: %w", maxAttempts, lastErr)
}

func retryable(err error) bool {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	// 429 quota, 500/503 overload — all transient on the free tier.
	return apiErr.Code == 429 || apiErr.Code == 500 || apiErr.Code == 503
}

// toGeminiContents also returns the system prompt separately: Gemini takes it
// as its own config field rather than as a message in the history.
func toGeminiContents(msgs []domain.Message) ([]*genai.Content, string, error) {
	var system string
	out := make([]*genai.Content, 0, len(msgs))

	for _, m := range msgs {
		switch m.Role {
		case domain.RoleSystem:
			system = m.Content

		case domain.RoleUser:
			out = append(out, genai.NewContentFromText(m.Content, genai.RoleUser))

		case domain.RoleAssistant:
			var parts []*genai.Part
			if m.Content != "" {
				parts = append(parts, &genai.Part{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if len(tc.Args) > 0 {
					if err := json.Unmarshal(tc.Args, &args); err != nil {
						return nil, "", fmt.Errorf("tool call %s: %w", tc.Name, err)
					}
				}
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID: tc.ID, Name: tc.Name, Args: args,
					},
					// Gemini rejects the request if a signed call comes back
					// without its signature.
					ThoughtSignature: tc.VendorState,
				})
			}
			out = append(out, &genai.Content{Role: genai.RoleModel, Parts: parts})

		case domain.RoleTool:
			// Gemini carries tool results as a user turn holding a
			// functionResponse part, keyed by the tool's name.
			out = append(out, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
					ID:       m.ToolCallID,
					Name:     m.ToolName,
					Response: map[string]any{"result": m.Content},
				}}},
			})
		}
	}
	return out, system, nil
}

func toGeminiTools(tools []domain.ToolDefinition) []*genai.FunctionDeclaration {
	out := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		var schema any
		if len(t.Schema) > 0 {
			if err := json.Unmarshal(t.Schema, &schema); err != nil {
				continue
			}
		}
		out = append(out, &genai.FunctionDeclaration{
			Name:                 t.Name,
			Description:          t.Description,
			ParametersJsonSchema: schema,
		})
	}
	return out
}

func fromGeminiResponse(resp *genai.GenerateContentResponse) (domain.Message, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return domain.Message{}, errors.New("gemini returned no candidates")
	}

	out := domain.Message{Role: domain.RoleAssistant}
	var text strings.Builder

	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" && !p.Thought {
			text.WriteString(p.Text)
		}
		if fc := p.FunctionCall; fc != nil {
			args := []byte("{}")
			if len(fc.Args) > 0 {
				encoded, err := json.Marshal(fc.Args)
				if err != nil {
					return domain.Message{}, fmt.Errorf("encode args for %s: %w", fc.Name, err)
				}
				args = encoded
			}
			id := fc.ID
			if id == "" {
				id = fc.Name // Gemini does not always assign call ids
			}
			out.ToolCalls = append(out.ToolCalls, domain.ToolCall{
				ID: id, Name: fc.Name, Args: args,
				VendorState: p.ThoughtSignature,
			})
		}
	}

	out.Content = text.String()
	return out, nil
}
