package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"jobagent/src/external/latex"
	"jobagent/src/internal/domain"
)

// Scorer is what the delivery layer needs from the usecase layer — declared
// here so the handler depends on a capability rather than a concrete type.
type Scorer interface {
	AnalyzeURL(ctx context.Context, resumeBody, jobURL string) (*domain.MatchAnalysis, error)
}

type Handler struct {
	scorer Scorer
}

func NewHandler(scorer Scorer) *Handler {
	return &Handler{scorer: scorer}
}

type analyzeRequest struct {
	Resume string `json:"resume" binding:"required"`
	JobURL string `json:"jobUrl" binding:"required"`
}

type analyzeResponse struct {
	Score   int            `json:"score"`
	Verdict string         `json:"verdict"`
	Summary string         `json:"summary"`
	Matched []domain.Match `json:"matched"`
	Gaps    []string       `json:"gaps"`
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) analyze(c *gin.Context) {
	var req analyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysis, err := h.scorer.AnalyzeURL(c.Request.Context(), resumeBody(req.Resume), req.JobURL)
	if err != nil {
		// The model or the posting failed, not the caller's request.
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analyzeResponse{
		Score:   analysis.Score,
		Verdict: string(analysis.Verdict),
		Summary: analysis.Summary,
		Matched: analysis.Matched,
		Gaps:    analysis.Gaps,
	})
}

// resumeBody strips a LaTeX preamble when one is present. A resume .tex is
// over a hundred lines of package imports before any content, and sending
// those would spend a large share of the prompt on text the model must ignore.
func resumeBody(raw string) string {
	if !strings.Contains(raw, `\begin{document}`) {
		return raw
	}
	doc, err := latex.Parse(raw)
	if err != nil {
		return raw
	}
	return doc.Body
}
