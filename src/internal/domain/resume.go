package domain

// Verdict is the analysis step's recommendation, used to decide whether
// generating a tailored resume is worth the tokens at all.
type Verdict string

const (
	VerdictApply   Verdict = "apply"
	VerdictStretch Verdict = "stretch"
	VerdictSkip    Verdict = "skip"
)

// Verdict thresholds. These are the gate that decides whether the pipeline
// spends tokens generating documents.
const (
	ApplyThreshold   = 70
	StretchThreshold = 50
)

// VerdictFor derives the recommendation from the score. Asking the model for
// both produced contradictions — a 60 came back "apply" while a 75 came back
// "stretch" — because it treated them as independent judgements. Deriving it
// here makes the gate consistent by construction.
func VerdictFor(score int) Verdict {
	switch {
	case score >= ApplyThreshold:
		return VerdictApply
	case score >= StretchThreshold:
		return VerdictStretch
	default:
		return VerdictSkip
	}
}

// Match pairs a requirement from the posting with the evidence in the resume
// that supports it. Requiring evidence is what stops the model asserting a
// score it cannot justify.
type Match struct {
	Requirement string `json:"requirement"`
	Evidence    string `json:"evidence"`
}

type MatchAnalysis struct {
	Score int `json:"score"`
	// Verdict is derived from Score by the usecase, not returned by the model.
	Verdict Verdict `json:"-"`
	Matched []Match `json:"matched"`
	// Gaps are requirements with nothing in the resume behind them. These are
	// the point of the analysis — an honest gap is more useful than a score.
	Gaps    []string `json:"gaps"`
	Summary string   `json:"summary"`
}

// Change records one edit so the whole rewrite can be audited quickly. Without
// it, verifying that nothing was fabricated means re-reading the full resume.
type Change struct {
	Before string `json:"before"`
	After  string `json:"after"`
	Reason string `json:"reason"`
}

type TailoredResume struct {
	Body    string   `json:"body"`
	Changes []Change `json:"changes"`
}

type Application struct {
	Analysis    MatchAnalysis
	Tailored    *TailoredResume
	CoverLetter string
}
