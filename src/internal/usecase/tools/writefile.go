package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"jobagent/src/internal/domain"
)

type WriteFile struct {
	files domain.FileWriter
}

func NewWriteFile(files domain.FileWriter) *WriteFile {
	return &WriteFile{files: files}
}

func (t *WriteFile) Definition() domain.ToolDefinition {
	return domain.ToolDefinition{
		Name: "write_file",
		Description: "Writes text to a file in the output directory, replacing it if " +
			"it already exists. Use this to save results such as a tailored resume " +
			"or a cover letter. Returns the path written.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File name relative to the output directory, e.g. \"cover_letter.md\""
				},
				"content": {
					"type": "string",
					"description": "The full text to write"
				}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *WriteFile) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("arguments were not valid JSON: %w", err)
	}
	if in.Path == "" {
		return "", errors.New("path is required")
	}

	n, where, err := t.files.WriteFile(ctx, in.Path, in.Content)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", n, where), nil
}
