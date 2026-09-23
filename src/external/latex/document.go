package latex

import (
	"errors"
	"strings"
)

const (
	beginDoc = `\begin{document}`
	endDoc   = `\end{document}`
)

// Document splits a .tex file into the parts that matter to a language model
// and the parts that do not. A resume preamble is a hundred-odd lines of
// package imports and margin tweaks — sending it would spend a large share of
// the prompt on text the model must not change anyway.
type Document struct {
	Preamble string
	Body     string
}

func Parse(source string) (*Document, error) {
	start := strings.Index(source, beginDoc)
	if start < 0 {
		return nil, errors.New(`no \begin{document} found — is this a LaTeX file?`)
	}
	bodyStart := start + len(beginDoc)

	end := strings.LastIndex(source, endDoc)
	if end < bodyStart {
		return nil, errors.New(`no \end{document} found after \begin{document}`)
	}

	return &Document{
		Preamble: source[:bodyStart],
		Body:     strings.TrimSpace(source[bodyStart:end]),
	}, nil
}

// Render reassembles the document around a new body, so a tailored resume
// keeps the original formatting and compiles unchanged.
func (d *Document) Render(body string) string {
	return d.Preamble + "\n\n" + strings.TrimSpace(body) + "\n\n" + endDoc + "\n"
}
