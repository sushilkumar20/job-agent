package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"jobagent/src/internal/domain"
)

// maxTextRunes caps what reaches the model. This is a context-window concern,
// not a transport one, which is why it lives here and not in the fetcher.
const maxTextRunes = 24000

type FetchURL struct {
	fetcher domain.PageFetcher
}

func NewFetchURL(fetcher domain.PageFetcher) *FetchURL {
	return &FetchURL{fetcher: fetcher}
}

func (t *FetchURL) Definition() domain.ToolDefinition {
	return domain.ToolDefinition{
		Name: "fetch_url",
		Description: "Fetches a public web page and returns its readable text with " +
			"markup removed. Use this to read a job posting when the user gives you " +
			"a link. Only works for public http(s) URLs.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "Full URL including scheme, e.g. \"https://example.com/jobs/123\""
				}
			},
			"required": ["url"]
		}`),
	}
}

func (t *FetchURL) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("arguments were not valid JSON: %w", err)
	}
	if in.URL == "" {
		return "", errors.New("url is required")
	}

	text, err := t.fetcher.Fetch(ctx, in.URL)
	if err != nil {
		return "", err
	}

	runes := []rune(text)
	if len(runes) > maxTextRunes {
		return string(runes[:maxTextRunes]) + "\n\n[truncated]", nil
	}
	return text, nil
}
