package domain

import "testing"

func TestVerdictFor(t *testing.T) {
	cases := []struct {
		score int
		want  Verdict
	}{
		{0, VerdictSkip},
		{49, VerdictSkip},
		{50, VerdictStretch}, // boundary
		{60, VerdictStretch}, // previously came back "apply" from the model
		{69, VerdictStretch},
		{70, VerdictApply}, // boundary
		{75, VerdictApply}, // previously came back "stretch" from the model
		{100, VerdictApply},
	}

	for _, c := range cases {
		if got := VerdictFor(c.score); got != c.want {
			t.Errorf("VerdictFor(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}
