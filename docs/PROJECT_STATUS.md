# job-agent — Project Status

> Snapshot as of 2026-09-24. Based on `AGENT_PLAN.md`, the code in `src/`, and uncommitted work.

## Planned vs. delivered

| Stage (from AGENT_PLAN.md) | Status | Evidence |
|---|---|---|
| **1. Resume tailoring** | ✅ Done (some edits not committed yet) | `Tailor` pipeline: analyse → verdict gate → tailored `.tex` → changes log → cover letter |
| **2. Job match scoring** | 🟡 Partly done | `fetch_url` tool, SSRF-safe fetcher and `POST /api/analyze` score **one** URL. There's no job search, ranking or embeddings. |
| **3. Agent with memory** | 🟡 Loop done, no memory | The ReAct `Agent` loop was built early, in Stage 1. There's no memory store or SQLite. |
| **4. Semi-automated apply** | ⬜ Not started | — |
| Extras you didn't plan | ✅ | 4 LLM adapters (Anthropic, Gemini, Ollama, Sarvam), clean-architecture ports, Gin HTTP server, justfile |

---

## 1. How it works now

### Architecture

```mermaid
flowchart TB
    subgraph Entry["src/main.go (how you start it)"]
        CLI1["go run ./src &lt;prompt&gt;<br/>(agent mode)"]
        CLI2["go run ./src tailor job.txt<br/>(pipeline mode)"]
        CLI3["go run ./src serve<br/>(HTTP mode)"]
    end

    subgraph Delivery["delivery/http"]
        R["router.go<br/>GET /health<br/>POST /api/analyze"]
        H["handler.go<br/>strips LaTeX preamble"]
    end

    subgraph Usecase["internal/usecase"]
        AG["Agent<br/>ReAct loop, max 10 iterations"]
        TL["Tailor<br/>fixed pipeline"]
        subgraph Tools["tools/"]
            T1[current_time]
            T2[fetch_url]
            T3[read_file]
            T4[write_file]
        end
    end

    subgraph Domain["internal/domain (ports)"]
        P1(["LLM<br/>Complete + Chat"])
        P2([PageFetcher])
        P3([FileReader / FileWriter])
        P4([Clock])
        V["VerdictFor(score)<br/>≥70 apply · ≥50 stretch · else skip"]
    end

    subgraph External["external (adapters)"]
        L1["anthropic.go<br/>⚠ Chat not implemented"]
        L2[gemini.go]
        L3[ollama.go]
        L4[sarvam.go]
        W["web/fetcher.go<br/>blocks private IPs"]
        F["fs/store.go<br/>data/ in, data/out/ out"]
        LX["latex/document.go<br/>preamble + body split"]
        CK[clock.go]
    end

    CLI1 --> AG
    CLI2 --> TL
    CLI3 --> R --> H --> TL
    AG --> Tools
    AG --> P1
    TL --> P1
    TL --> P2
    TL --> V
    T2 --> P2
    T3 & T4 --> P3
    T1 --> P4
    P1 -.implemented by.-> L1 & L2 & L3 & L4
    P2 -.-> W
    P3 -.-> F
    P4 -.-> CK
    CLI2 --> LX
    H --> LX
```

### Tailor pipeline (`just run tailor job.txt`)

```mermaid
flowchart LR
    A[data/resume.tex] --> B["latex.Parse<br/>keep preamble,<br/>send body only"]
    J[data/job.txt] --> C
    B --> C["LLM: analyzePrompt<br/>→ JSON score, matched, gaps"]
    C --> D{"VerdictFor(score)"}
    D -- "skip &lt;50" --> S[write analysis.md<br/>stop, save tokens]
    D -- "stretch / apply" --> E["LLM: tailorPrompt<br/>→ JSON body + changes"]
    E --> F["LLM: coverLetterPrompt"]
    F --> O["data/out/<br/>analysis.md<br/>resume_tailored.tex<br/>changes.md<br/>cover_letter.md"]
```

### Agent loop (`just run "your prompt"`)

```mermaid
sequenceDiagram
    participant U as You (CLI)
    participant A as Agent.Run
    participant L as LLM.Chat
    participant T as Tool
    U->>A: system prompt + user prompt
    loop up to 10 iterations
        A->>L: full history + tool definitions
        alt reply has ToolCalls
            L-->>A: tool_call(name, args)
            A->>T: Execute(args)
            T-->>A: result, or "error: …" (never aborts)
            A->>A: append reply + tool result to history
        else plain text
            L-->>A: final answer
            A-->>U: answer
        end
    end
```

---

## 2. What's pending

```mermaid
flowchart TB
    classDef urgent fill:#fde2e2,stroke:#c0392b,color:#000
    classDef todo fill:#fff4d6,stroke:#b7950b,color:#000
    classDef later fill:#e8eaf6,stroke:#5c6bc0,color:#000

    subgraph Now["Loose ends in current code"]
        N1["Anthropic Chat() is a stub<br/>default provider can't run agent mode"]:::urgent
        N2["Uncommitted: resume.go, tailor.go,<br/>tailor_prompts.go, resume_test.go"]:::urgent
        N3["HTTP only exposes /analyze<br/>no /tailor or /cover-letter"]:::todo
        N4["Tests: only VerdictFor<br/>missing: Agent loop, decodeJSON,<br/>latex.Parse, handler"]:::todo
        N5["No .tex → PDF compile step"]:::todo
        N6["Anti-fabrication is prompt-only<br/>no check that 'after' text is backed by the source"]:::todo
        N7["README empty; AGENT_PLAN.md is stale<br/>(says cmd/ + cobra, log ends at Session 2)"]:::todo
    end

    subgraph S2["Stage 2 remaining"]
        A1[Job search tool / job board source]:::later
        A2[Score many postings in one run]:::later
        A3[Rank + dedupe results]:::later
        A4[Embeddings for pre-filtering]:::later
    end

    subgraph S3["Stage 3 remaining"]
        B1["Memory store: SQLite (modernc)"]:::later
        B2["Long-term preferences:<br/>remote, stack, salary"]:::later
        B3["Track seen / applied jobs"]:::later
    end

    subgraph S4["Stage 4 (all)"]
        C1[Approval step: human in the loop]:::later
        C2[Guardrails: rate limits, allowlist]:::later
        C3[Apply / email draft action]:::later
    end

    N1 --> S3
    A2 --> A3 --> B3 --> C1
```

**Most important gap:** `LLM_PROVIDER` defaults to `anthropic`, but [anthropic.go:57](../src/external/llm/anthropic.go#L57) returns "Chat not implemented". With the default provider, only `tailor` and `serve` work. Agent mode only runs on Gemini, Ollama or Sarvam.

---

## 3. Future plan

### Roadmap

```mermaid
gantt
    title job-agent roadmap (from AGENT_PLAN.md estimates)
    dateFormat YYYY-MM-DD
    axisFormat %d %b

    section Stabilise
    Commit current work + tests        :s1, 2026-09-25, 1d
    Anthropic Chat with tool_use blocks :s2, after s1, 2d
    Update README + AGENT_PLAN          :s3, after s1, 1d

    section Stage 2 · Match scoring
    Job source tool (search/listing)    :t1, after s2, 2d
    Batch scoring + ranking             :t2, after t1, 2d
    Embeddings pre-filter (optional)    :t3, after t2, 2d
    HTTP /api/jobs + /api/tailor        :t4, after t2, 1d

    section Stage 3 · Memory
    SQLite store (jobs, preferences)    :m1, after t3, 2d
    Preference memory in system prompt  :m2, after m1, 2d
    Natural-language multi-step runs    :m3, after m2, 2d

    section Stage 4 · Apply
    Approval queue (CLI/HTTP)           :h1, after m3, 2d
    Guardrails + audit log              :h2, after h1, 1d
    End-to-end pipeline                 :h3, after h2, 1d
```

### Target architecture after Stage 4

```mermaid
flowchart LR
    U["You: 'find remote backend<br/>SDE-2 jobs'"] --> AG[Agent ReAct loop]
    MEM[("SQLite memory<br/>preferences · seen jobs<br/>applications")] <--> AG
    AG --> TS[search_jobs tool]
    AG --> SC["score_job tool<br/>(existing Tailor.Analyze)"]
    AG --> TA["tailor tool<br/>(existing Tailor.Run)"]
    TS --> RK[rank + filter]
    RK --> SC
    SC -->|apply / stretch| TA
    TA --> Q{"Human approval<br/>✅ / ✏️ / ❌"}
    Q -- approve --> AP["apply / draft email<br/>+ audit log"]
    Q -- edit --> TA
    AP --> MEM
```

**Design note for Stage 3:** `Tailor` is a fixed pipeline, and `Agent` is the loop that makes decisions. The easy path is to wrap `Tailor.Analyze` and `Tailor.Run` as **tools**. The agent then chooses *which* jobs to process, and the pipeline keeps doing each job the same way every time. The verdict gate from Stage 1 carries over unchanged.

## Next step

1. Commit the current changes (verdict derivation + `resume_test.go`).
2. Implement Anthropic `Chat()`: translate `domain.Message`/`ToolCall` to Anthropic's typed `tool_use`/`tool_result` content blocks, so the default provider can run agent mode.
