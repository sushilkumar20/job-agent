package domain

import (
	"context"
	"encoding/json"
)

// ToolDefinition is what the model sees. Description is prompt text the model
// reads to decide whether to call the tool, not documentation for humans.
type ToolDefinition struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// Tool is something the agent can run on the model's behalf. Definition is
// sent to the model; Execute is invoked only if the model asks for it and the
// agent agrees.
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
