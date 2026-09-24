package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"jobagent/src/internal/domain"
)

// fakeLLM replays a scripted list of replies. This is the payoff for putting
// the model behind an interface: the loop is testable with no network, no key,
// and no cost.
type fakeLLM struct {
	replies []domain.Message
	calls   int
}

func (f *fakeLLM) Complete(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeLLM) Chat(_ context.Context, _ []domain.Message, _ []domain.ToolDefinition) (domain.Message, error) {
	reply := f.replies[min(f.calls, len(f.replies)-1)]
	f.calls++
	return reply, nil
}

type fakeTool struct {
	name   string
	result string
	err    error
	runs   int
}

func (t *fakeTool) Definition() domain.ToolDefinition {
	return domain.ToolDefinition{Name: t.name, Schema: json.RawMessage(`{"type":"object"}`)}
}

func (t *fakeTool) Execute(context.Context, json.RawMessage) (string, error) {
	t.runs++
	return t.result, t.err
}

func toolCall(name string) domain.Message {
	return domain.Message{
		Role:      domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{{ID: "c1", Name: name, Args: json.RawMessage(`{}`)}},
	}
}

func TestAgentReturnsWhenNoToolCalls(t *testing.T) {
	llm := &fakeLLM{replies: []domain.Message{
		{Role: domain.RoleAssistant, Content: "done"},
	}}

	got, err := NewAgent(llm, nil).Run(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got != "done" {
		t.Errorf("got %q, want %q", got, "done")
	}
	if llm.calls != 1 {
		t.Errorf("called the model %d times, want 1", llm.calls)
	}
}

func TestAgentRunsToolThenReturns(t *testing.T) {
	tool := &fakeTool{name: "clock", result: "12:00"}
	llm := &fakeLLM{replies: []domain.Message{
		toolCall("clock"),
		{Role: domain.RoleAssistant, Content: "it is 12:00"},
	}}

	got, err := NewAgent(llm, []domain.Tool{tool}).Run(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got != "it is 12:00" {
		t.Errorf("got %q", got)
	}
	if tool.runs != 1 {
		t.Errorf("tool ran %d times, want 1", tool.runs)
	}
}

// A model that keeps asking for tools must not loop forever — every iteration
// resends the whole history, so an unbounded loop is expensive as well as stuck.
func TestAgentStopsAtMaxIterations(t *testing.T) {
	tool := &fakeTool{name: "clock", result: "12:00"}
	llm := &fakeLLM{replies: []domain.Message{toolCall("clock")}} // never finishes

	agent := NewAgent(llm, []domain.Tool{tool})
	agent.MaxIterations = 3

	_, err := agent.Run(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected an error when the model never stops calling tools")
	}
	if !strings.Contains(err.Error(), "did not finish") {
		t.Errorf("unhelpful error: %v", err)
	}
	if llm.calls != 3 {
		t.Errorf("model called %d times, want 3", llm.calls)
	}
}

// A failing tool should report back to the model rather than abort the run —
// the model may recover by trying something else.
func TestAgentFeedsToolErrorsBackToModel(t *testing.T) {
	tool := &fakeTool{name: "clock", err: errStub}
	llm := &fakeLLM{replies: []domain.Message{
		toolCall("clock"),
		{Role: domain.RoleAssistant, Content: "the clock is broken"},
	}}

	got, err := NewAgent(llm, []domain.Tool{tool}).Run(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("a tool failure must not abort the run: %v", err)
	}
	if got != "the clock is broken" {
		t.Errorf("got %q", got)
	}
}

func TestAgentHandlesUnknownTool(t *testing.T) {
	llm := &fakeLLM{replies: []domain.Message{
		toolCall("nonexistent"),
		{Role: domain.RoleAssistant, Content: "no such tool"},
	}}

	if _, err := NewAgent(llm, nil).Run(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("an unknown tool must not abort the run: %v", err)
	}
}

var errStub = stubError("tool exploded")

type stubError string

func (e stubError) Error() string { return string(e) }
