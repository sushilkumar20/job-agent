# 🤖 Job Application Agent — Project Context & Plan

> **Purpose of this file**: This captures the full context of our planning discussion.
> If you're an AI assistant reading this in a new session — this is your onboarding doc.
> Read this fully before proceeding with any work.

---

## 👤 About the Developer (Sushil Kumar)

- **Background**: Backend developer, experienced with Go, Kafka, distributed systems
- **Current projects**: `kafka_on_shore` (Go/clean architecture), `agent-orchestrator`, `triply`, `focusguard`, `blog_publication`
- **Goal**: Learn about AI agents, LLMs, and prompt engineering by building a real project
- **Experience with AI/Agents**: Beginner — wants to understand deeply, not just use frameworks
- **Teaching approach requested**: Explain concepts briefly but with depth. Goal is independence — Sushil wants to build agents on his own in the future without relying on anyone.
- **Resume files available**: `/Users/sushilkumar/dev/resume.tex` and `/Users/sushilkumar/dev/sushil_kumar_resume.tex`

---

## 🎯 Project: Job Application Agent

An AI agent that helps find, evaluate, tailor resumes for, and semi-automatically apply to jobs.

### Why This Project?
- Learn how agents work (LLM calls, tools, memory, loops)
- Learn prompt engineering and structured output
- Build something practically useful
- Get into the AI/LLM world coming from a backend engineering background

---

## 🏗️ Architecture — 4 Stages (Build Incrementally)

Each stage teaches a core agent concept:

### Stage 1: Resume Tailoring Agent 🟢 (START HERE)
- **Input**: Resume + Job description
- **Output**: Tailored resume + cover letter + match analysis
- **Concepts learned**: LLM API calls, prompt engineering, structured output, system vs user prompts
- **Estimated time**: 1 weekend (2-3 days)

### Stage 2: Job Match Scoring Agent 🔧
- **Input**: Resume + search query (e.g., "backend engineer, remote")
- **Output**: Ranked list of jobs with match scores
- **Concepts learned**: Tools/function calling, web scraping, embeddings & similarity
- **Estimated time**: ~1 week

### Stage 3: Smart Agent with Memory 🧠
- **Input**: Natural language instruction ("Find me backend jobs, prefer remote")
- **Output**: Autonomous multi-step workflow with memory of preferences
- **Concepts learned**: ReAct agent loop, state management, short-term & long-term memory
- **Estimated time**: ~1 week

### Stage 4: Semi-Automated Apply 👤
- **Input**: Full pipeline — search → score → tailor → draft → approve → apply
- **Output**: Human-in-the-loop application workflow
- **Concepts learned**: Human-in-the-loop, safety/guardrails, end-to-end orchestration
- **Estimated time**: 3-4 days

---

## 🧠 Key Concepts Explained (Reference)

### What is an Agent?
A regular LLM call is one-shot: Question → Answer → Done.
An Agent has a **loop**: Think → Act → Observe → Think again.
It decides what to do next on its own, has tools it can use, and optionally remembers past interactions.

### The 4 Building Blocks
1. **LLM (Brain)** — The AI model that reasons and generates text
2. **Tools** — Functions the agent can call (search, scrape, score, email)
3. **Memory** — Past context (short-term = conversation, long-term = saved preferences)
4. **Orchestration Loop** — The think→act→observe cycle (ReAct pattern)

### The ReAct Loop (Core of Every Agent)
```go
func RunAgent(userRequest string) string {
    messages := []Message{systemPrompt, {Role: "user", Content: userRequest}}
    for {
        response := llm.Generate(messages)
        if response.HasToolCall() {
            result := executeTool(response.ToolCall)
            messages = append(messages, result) // OBSERVE
        } else {
            return response.Text              // DONE
        }
    }
}
```

### Why Build From Scratch (Not LangChain)?
Frameworks hide the internals. Building from scratch first teaches how agents actually work.
After understanding the patterns, frameworks become easy to use and debug.

---

## 🛠️ Tech Stack (Decided)

| Component | Choice | Reason |
|-----------|--------|--------|
| Language | Go 1.22+ | Sushil's primary language, strong typing, single binary |
| LLM | Anthropic Claude API via `github.com/anthropics/anthropic-sdk-go` | Official Go SDK, powerful models (Sonnet/Haiku), excellent structured output & tool use |
| HTTP/Scraping | `net/http` + `goquery` | Standard lib HTTP + jQuery-like HTML parsing |
| Output Parsing | Go structs + `encoding/json` | Native JSON unmarshalling with struct tags |
| Storage | JSON files → SQLite via `modernc.org/sqlite` (Stage 3) | Start simple, upgrade later (pure Go SQLite) |
| CLI Interface | `cobra` + `lipgloss`/`bubbletea` | Industry-standard CLI framework + beautiful TUI |
| Module System | Go modules | Native, no extra tooling needed |

---

## 📁 Project Structure (Planned)

```
job-agent/
├── go.mod
├── go.sum
├── README.md
├── .env                        # API keys (never commit)
├── AGENT_PLAN.md               # THIS FILE — project context
├── data/
│   ├── my_resume.md            # Resume in markdown
│   └── jobs/                   # Saved job listings
├── cmd/
│   └── jobagent/
│       └── main.go             # CLI entry point
├── internal/
│   ├── llm/
│   │   └── client.go           # Claude (Anthropic) API client wrapper
│   ├── prompts/
│   │   └── templates.go        # Prompt templates
│   ├── models/
│   │   └── models.go           # Data structs (resume, job, analysis)
│   ├── tools/                  # Agent tools (Stage 2+)
│   │   ├── jobsearch.go
│   │   ├── resumeparser.go
│   │   └── emaildrafter.go
│   ├── memory/                 # Agent memory (Stage 3+)
│   │   └── store.go
│   └── agent/
│       └── agent.go            # Agent orchestration loop (Stage 3+)
└── tests/
```

---

## ❓ Open Questions

1. ~~**API key**~~ — ✅ Switched from Gemini to **Anthropic Claude API**. Sushil will set `ANTHROPIC_API_KEY` in `.env`
2. ~~**Job target**~~ — ✅ Backend Engineer / SDE-2 roles
3. ~~**Timeline**~~ — To be decided as we go

---

## 📝 Discussion Log

### Session 1 — Sept 20, 2026
- Discussed project ideas for learning about AI agents
- Decided on "Job Application Agent" as the project
- Created 4-stage implementation plan (resume tailor → job match → memory → semi-auto apply)
- Initially decided on Python, then **switched to Go** — Sushil's primary language
- Rationale: Learn agent concepts without language learning overhead. Go Gemini SDK is solid.
- Created project folder at `/Users/sushilkumar/dev/job-agent/`
- **Status**: Planning complete. Ready to start Stage 1 once open questions are answered.

### Session 2 — Sept 22, 2026
- Switched LLM provider from **Google Gemini** to **Anthropic Claude API**
- SDK: `github.com/anthropics/anthropic-sdk-go` (official Go SDK)
- Models: `claude-sonnet-4-6` (primary), `claude-haiku-4-5` (cheaper/faster option)
- Env var: `ANTHROPIC_API_KEY`
- Rationale: Sushil already has Claude API access. Claude has excellent structured output and tool use support — ideal for agent building.

---

## 🚀 Next Steps

1. Set up the Go project (`go mod init`, install `anthropic-sdk-go`)
2. Get Anthropic API key configured in `.env` (`ANTHROPIC_API_KEY=sk-ant-...`)
3. Start Stage 1: Resume Tailoring Agent


