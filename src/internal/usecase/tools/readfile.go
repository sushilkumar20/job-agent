package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"jobagent/src/internal/domain"
)

// maxFileRunes caps what reaches the model — a context-window limit, distinct
// from the memory limit the storage adapter applies.
const maxFileRunes = 24000

type ReadFile struct {
	files domain.FileReader
}

func NewReadFile(files domain.FileReader) *ReadFile {
	return &ReadFile{files: files}
}

func (t *ReadFile) Definition() domain.ToolDefinition {
	return domain.ToolDefinition{
		Name: "read_file",
		Description: "Reads a UTF-8 text file from the project data directory and " +
			"returns its contents. Use this whenever you need the contents of a " +
			"file the user refers to, such as a resume or a job description.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path relative to the data directory, e.g. \"resume.md\""
				}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ReadFile) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("arguments were not valid JSON: %w", err)
	}
	if in.Path == "" {
		return "", errors.New("path is required")
	}

	content, err := t.files.ReadFile(ctx, in.Path)
	if err != nil {
		return "", err
	}

	runes := []rune(content)
	if len(runes) > maxFileRunes {
		return string(runes[:maxFileRunes]) + "\n\n[truncated]", nil
	}
	return content, nil
}
