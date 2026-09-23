package domain

import (
	"context"
	"time"
)

// FileReader and FileWriter are deliberately separate: the agent reads from
// one directory and writes to another, and splitting the ports means a
// read-only dependency cannot be handed something that can write.
type FileReader interface {
	ReadFile(ctx context.Context, path string) (string, error)
}

type FileWriter interface {
	// WriteFile reports the number of bytes written and where they landed.
	WriteFile(ctx context.Context, path, content string) (int, string, error)
}

// Clock exists so the agent's notion of "now" is injected rather than read
// straight from the process, which keeps it testable.
type Clock interface {
	Now() time.Time
}
