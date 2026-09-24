package latex

import "testing"

const sample = `\documentclass{article}
\usepackage{fontawesome5}
\begin{document}
\section{Experience}
\resumeItem{Built things}
\end{document}
`

func TestParseSplitsPreambleFromBody(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}

	if want := `\section{Experience}` + "\n" + `\resumeItem{Built things}`; doc.Body != want {
		t.Errorf("body = %q, want %q", doc.Body, want)
	}
	// The preamble must keep \begin{document} so Render can reattach it.
	if got := doc.Preamble; got[len(got)-len(`\begin{document}`):] != `\begin{document}` {
		t.Errorf("preamble should end with the begin marker, got %q", got)
	}
}

func TestRenderRoundTrips(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Parse(doc.Render(doc.Body))
	if err != nil {
		t.Fatalf("rendered output is no longer parseable: %v", err)
	}
	if out.Body != doc.Body {
		t.Errorf("body changed across a round trip:\n got %q\nwant %q", out.Body, doc.Body)
	}
}

func TestParseRejectsNonLatex(t *testing.T) {
	for name, input := range map[string]string{
		"no markers":  "# Just markdown\n\nSome text.",
		"no end":      `\begin{document}` + "\ncontent",
		"end first":   `\end{document}` + "\ncontent\n" + `\begin{document}`,
		"empty input": "",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
