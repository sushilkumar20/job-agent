package usecase

import (
	"context"
	"fmt"

	"jobagent/src/internal/domain"
)

const defaultMaxIterations = 10

// Agent runs the think → act → observe loop. It owns the orchestration; the
// LLM only decides, and the tools only execute.
type Agent struct {
	llm   domain.LLM
	tools map[string]domain.Tool
	defs  []domain.ToolDefinition

	// MaxIterations stops a model that keeps calling tools without ever
	// producing an answer.
	MaxIterations int

	// OnStep, if set, is called with each assistant reply so callers can
	// watch the loop without the agent depending on a logger.
	OnStep func(iteration int, reply domain.Message)
}

func NewAgent(llm domain.LLM, tools []domain.Tool) *Agent {
	registry := make(map[string]domain.Tool, len(tools))
	defs := make([]domain.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		def := t.Definition()
		registry[def.Name] = t
		defs = append(defs, def)
	}

	return &Agent{
		llm:           llm,
		tools:         registry,
		defs:          defs,
		MaxIterations: defaultMaxIterations,
	}
}

func (a *Agent) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	msgs := []domain.Message{
		{Role: domain.RoleSystem, Content: systemPrompt},
		{Role: domain.RoleUser, Content: userPrompt},
	}

	for i := 1; i <= a.MaxIterations; i++ {
		reply, err := a.llm.Chat(ctx, msgs, a.defs)
		if err != nil {
			return "", fmt.Errorf("iteration %d: %w", i, err)
		}

		if a.OnStep != nil {
			a.OnStep(i, reply)
		}

		if len(reply.ToolCalls) == 0 {
			return reply.Content, nil
		}

		// The request and every result must both be in the history, or the
		// model sees answers to questions it never asked.
		msgs = append(msgs, reply)
		for _, call := range reply.ToolCalls {
			msgs = append(msgs, a.execute(ctx, call))
		}
	}

	return "", fmt.Errorf("agent did not finish within %d iterations", a.MaxIterations)
}

// execute never returns an error: a failed tool is something the model should
// see and react to, not something that aborts the run.
func (a *Agent) execute(ctx context.Context, call domain.ToolCall) domain.Message {
	result := func() string {
		tool, ok := a.tools[call.Name]
		if !ok {
			return fmt.Sprintf("error: no tool named %q is available", call.Name)
		}
		out, err := tool.Execute(ctx, call.Args)
		if err != nil {
			return "error: " + err.Error()
		}
		return out
	}()

	return domain.Message{
		Role:       domain.RoleTool,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Content:    result,
	}
}
