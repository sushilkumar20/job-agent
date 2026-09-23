package domain

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is the model asking for a function to be run. It never runs
// anything itself — executing this is the caller's decision.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage

	// VendorState is opaque data the provider attached to this call and
	// requires back verbatim on the next request — Gemini's thought
	// signature, for example. The agent never inspects it; it only carries
	// it along so the adapter can replay it.
	VendorState []byte
}

type Message struct {
	Role      Role
	Content   string
	ToolCalls []ToolCall
	// ToolCallID and ToolName are set on a RoleTool message to say which
	// ToolCall it answers. Vendors correlate differently — some by id, some
	// by name — so the domain carries both.
	ToolCallID string
	ToolName   string
}

// LLM is the contract the domain requires of any language model.
type LLM interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// Chat is a single round trip: the full history and the tool menu go in,
	// one reply comes out. The reply either has Content (done) or ToolCalls
	// (the model wants something run). It does not loop.
	Chat(ctx context.Context, msgs []Message, tools []ToolDefinition) (Message, error)
}
