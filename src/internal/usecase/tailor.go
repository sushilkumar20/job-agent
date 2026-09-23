package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jobagent/src/internal/domain"
)

// Tailor is a fixed pipeline, not an agent: the steps and their order are
// known up front, so there is nothing for a model to decide about control
// flow.
type Tailor struct {
	llm domain.LLM
	// fetcher is only needed by AnalyzeURL; the file-based entry points work
	// without it.
	fetcher domain.PageFetcher

	// OnStep, if set, reports progress so callers can show what is happening
	// without this package depending on a logger.
	OnStep func(step string)
}

func NewTailor(llm domain.LLM, fetcher domain.PageFetcher) *Tailor {
	return &Tailor{llm: llm, fetcher: fetcher}
}

// AnalyzeURL fetches a posting and scores it. This is the entry point the HTTP
// layer uses: a caller supplies a resume and a link, and gets a score back.
func (t *Tailor) AnalyzeURL(ctx context.Context, resumeBody, jobURL string) (*domain.MatchAnalysis, error) {
	if t.fetcher == nil {
		return nil, errors.New("no page fetcher configured")
	}

	t.step("fetching posting")
	job, err := t.fetcher.Fetch(ctx, jobURL)
	if err != nil {
		return nil, fmt.Errorf("fetch posting: %w", err)
	}

	return t.Analyze(ctx, resumeBody, job)
}

// Analyze scores the fit. It runs alone so a poor match can stop the pipeline
// before spending tokens on a resume that should not be sent.
func (t *Tailor) Analyze(ctx context.Context, resumeBody, job string) (*domain.MatchAnalysis, error) {
	t.step("analysing fit")

	raw, err := t.llm.Complete(ctx, analyzePrompt, buildInput(resumeBody, job))
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	var analysis domain.MatchAnalysis
	if err := decodeJSON(raw, &analysis); err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	return &analysis, nil
}

func (t *Tailor) TailorResume(ctx context.Context, resumeBody, job string) (*domain.TailoredResume, error) {
	t.step("tailoring resume")

	raw, err := t.llm.Complete(ctx, tailorPrompt, buildInput(resumeBody, job))
	if err != nil {
		return nil, fmt.Errorf("tailor: %w", err)
	}

	var tailored domain.TailoredResume
	if err := decodeJSON(raw, &tailored); err != nil {
		return nil, fmt.Errorf("tailor: %w", err)
	}
	if strings.TrimSpace(tailored.Body) == "" {
		return nil, fmt.Errorf("tailor: model returned an empty resume body")
	}
	return &tailored, nil
}

func (t *Tailor) CoverLetter(ctx context.Context, resumeBody, job string) (string, error) {
	t.step("writing cover letter")

	letter, err := t.llm.Complete(ctx, coverLetterPrompt, buildInput(resumeBody, job))
	if err != nil {
		return "", fmt.Errorf("cover letter: %w", err)
	}
	return strings.TrimSpace(letter), nil
}

// Run executes the full pipeline, stopping early when the analysis says the
// candidate should not apply.
func (t *Tailor) Run(ctx context.Context, resumeBody, job string) (*domain.Application, error) {
	analysis, err := t.Analyze(ctx, resumeBody, job)
	if err != nil {
		return nil, err
	}

	app := &domain.Application{Analysis: *analysis}
	if analysis.Verdict == domain.VerdictSkip {
		t.step("verdict is skip — stopping before generating documents")
		return app, nil
	}

	tailored, err := t.TailorResume(ctx, resumeBody, job)
	if err != nil {
		return nil, err
	}
	app.Tailored = tailored

	letter, err := t.CoverLetter(ctx, resumeBody, job)
	if err != nil {
		return nil, err
	}
	app.CoverLetter = letter

	return app, nil
}

func (t *Tailor) step(msg string) {
	if t.OnStep != nil {
		t.OnStep(msg)
	}
}

func buildInput(resumeBody, job string) string {
	return fmt.Sprintf("<resume>\n%s\n</resume>\n\n<job_posting>\n%s\n</job_posting>", resumeBody, job)
}

// decodeJSON tolerates the markdown fence models wrap JSON in despite being
// told not to.
func decodeJSON(raw string, target any) error {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```") {
		if i := strings.Index(cleaned, "\n"); i != -1 {
			cleaned = cleaned[i+1:]
		}
		cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(cleaned)), target); err != nil {
		return fmt.Errorf("model did not return valid JSON: %w\n--- response ---\n%s", err, raw)
	}
	return nil
}
