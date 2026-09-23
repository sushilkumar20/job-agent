package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	httpdelivery "jobagent/src/delivery/http"
	"jobagent/src/external/clock"
	"jobagent/src/external/fs"
	"jobagent/src/external/latex"
	"jobagent/src/external/llm"
	"jobagent/src/external/web"
	"jobagent/src/internal/config"
	"jobagent/src/internal/domain"
	"jobagent/src/internal/usecase"
	"jobagent/src/internal/usecase/tools"
)

const systemPrompt = `You are a job application assistant.

You have tools for reading and writing files in the project data directory and
for fetching public web pages. Use them rather than guessing or inventing
content.

When working with a resume: never invent experience, employers, dates, or
metrics that are not in the source. Reframing what is there is fine;
fabrication is not. Name real gaps honestly instead of flattering the
candidate.`

const defaultPrompt = "Read notes.md and write a two-sentence professional summary to summary.md, " +
	"then tell me what you saved."

// prompt takes the request from the command line so a new question does not
// need a recompile.
func prompt() string {
	if args := os.Args[1:]; len(args) > 0 {
		return strings.Join(args, " ")
	}
	return defaultPrompt
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("provider=%s model=%s maxTokens=%d\n", cfg.Provider, cfg.Model, cfg.MaxTokens)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	l, err := newLLM(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	dataDir, err := fs.NewReader("data")
	if err != nil {
		log.Fatal(err)
	}
	defer dataDir.Close()

	outDir, err := fs.NewWriter("data/out")
	if err != nil {
		log.Fatal(err)
	}
	defer outDir.Close()

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "serve" {
		runServer(l)
		return
	}

	if len(args) > 1 && args[0] == "tailor" {
		if err := runTailor(ctx, l, dataDir, outDir, args[1]); err != nil {
			log.Fatalf("%s: %v", cfg.Provider, err)
		}
		return
	}

	agent := usecase.NewAgent(l, []domain.Tool{
		tools.NewCurrentTime(clock.NewSystem()),
		tools.NewFetchURL(web.NewFetcher()),
		tools.NewReadFile(dataDir),
		tools.NewWriteFile(outDir),
	})

	agent.OnStep = func(i int, reply domain.Message) {
		for _, c := range reply.ToolCalls {
			fmt.Printf("  [%d] wants %s(%s)\n", i, c.Name, c.Args)
		}
		if len(reply.ToolCalls) == 0 {
			fmt.Printf("  [%d] answered\n", i)
		}
	}

	answer, err := agent.Run(ctx, systemPrompt, prompt())
	if err != nil {
		log.Fatalf("%s: %v", cfg.Provider, err)
	}

	fmt.Println("\nanswer:", answer)
}

// runServer exposes the agent over HTTP. The request context governs each
// analysis, so there is no process-wide deadline here.
func runServer(l domain.LLM) {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}

	tailor := usecase.NewTailor(l, web.NewFetcher())
	srv := httpdelivery.NewServer(
		httpdelivery.Config{Addr: addr, AllowedOrigin: origin},
		httpdelivery.NewHandler(tailor),
	)

	log.Printf("listening on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// runTailor drives the fixed tailoring pipeline: read the LaTeX resume and the
// posting, analyse the fit, and only then generate documents.
func runTailor(ctx context.Context, l domain.LLM, in *fs.Reader, out *fs.Writer, jobPath string) error {
	source, err := in.ReadFile(ctx, "resume.tex")
	if err != nil {
		return fmt.Errorf("read resume.tex: %w", err)
	}
	doc, err := latex.Parse(source)
	if err != nil {
		return err
	}

	job, err := in.ReadFile(ctx, jobPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", jobPath, err)
	}

	tailor := usecase.NewTailor(l, web.NewFetcher())
	tailor.OnStep = func(msg string) { fmt.Println(" ", msg) }

	app, err := tailor.Run(ctx, doc.Body, job)
	if err != nil {
		return err
	}

	a := app.Analysis
	fmt.Printf("\nscore %d/100  verdict %s\n%s\n\n", a.Score, a.Verdict, a.Summary)
	for _, m := range a.Matched {
		fmt.Printf("  + %s\n      %s\n", m.Requirement, m.Evidence)
	}
	for _, g := range a.Gaps {
		fmt.Printf("  - %s\n", g)
	}

	if _, _, err := out.WriteFile(ctx, "analysis.md", renderAnalysis(a)); err != nil {
		return err
	}
	if app.Tailored == nil {
		fmt.Println("\nverdict was skip — no documents generated")
		return nil
	}

	if _, path, err := out.WriteFile(ctx, "resume_tailored.tex", doc.Render(app.Tailored.Body)); err != nil {
		return err
	} else {
		fmt.Println("\nwrote", path)
	}
	if _, path, err := out.WriteFile(ctx, "changes.md", renderChanges(app.Tailored.Changes)); err != nil {
		return err
	} else {
		fmt.Println("wrote", path)
	}
	if _, path, err := out.WriteFile(ctx, "cover_letter.md", app.CoverLetter); err != nil {
		return err
	} else {
		fmt.Println("wrote", path)
	}
	return nil
}

func renderAnalysis(a domain.MatchAnalysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Match analysis\n\n**Score:** %d/100  \n**Verdict:** %s\n\n%s\n\n## Matched\n\n", a.Score, a.Verdict, a.Summary)
	for _, m := range a.Matched {
		fmt.Fprintf(&b, "- **%s**\n  - %s\n", m.Requirement, m.Evidence)
	}
	b.WriteString("\n## Gaps\n\n")
	for _, g := range a.Gaps {
		fmt.Fprintf(&b, "- %s\n", g)
	}
	return b.String()
}

func renderChanges(changes []domain.Change) string {
	var b strings.Builder
	b.WriteString("# What changed\n\n")
	for _, c := range changes {
		fmt.Fprintf(&b, "**Why:** %s\n\n- before: %s\n- after: %s\n\n---\n\n", c.Reason, c.Before, c.After)
	}
	return b.String()
}

// newLLM is the only place that maps configuration to a concrete adapter.
// Its return type is the port, so nothing downstream knows which one it got.
func newLLM(ctx context.Context, cfg *config.Config) (domain.LLM, error) {
	switch cfg.Provider {
	case config.ProviderAnthropic:
		return llm.NewAnthropicLLM(cfg.Model, cfg.APIKey, cfg.MaxTokens), nil
	case config.ProviderGemini:
		return llm.NewGeminiLLM(ctx, cfg.Model, cfg.APIKey, cfg.MaxTokens)
	case config.ProviderOllama:
		return llm.NewOllamaLLM(cfg.Model, cfg.BaseURL), nil
	case config.ProviderSarvam:
		return llm.NewSarvamLLM(cfg.Model, cfg.APIKey, cfg.BaseURL, cfg.ReasoningEffort, cfg.MaxTokens), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}
