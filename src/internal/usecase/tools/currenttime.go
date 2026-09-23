package tools

import (
	"context"
	"encoding/json"
	"time"

	"jobagent/src/internal/domain"
)

type CurrentTime struct {
	clock domain.Clock
}

func NewCurrentTime(clock domain.Clock) *CurrentTime {
	return &CurrentTime{clock: clock}
}

func (t *CurrentTime) Definition() domain.ToolDefinition {
	return domain.ToolDefinition{
		Name: "get_current_time",
		// This text is prompt input, not documentation. Saying *when* to call
		// the tool matters more than saying what it returns.
		Description: "Returns the current local date and time in RFC3339 format. " +
			"Call this whenever the user asks what the time or date is — you have " +
			"no other way to know it.",
		Schema: json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (t *CurrentTime) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return t.clock.Now().Format(time.RFC3339), nil
}
