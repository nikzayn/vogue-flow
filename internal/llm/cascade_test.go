package llm

import "testing"

func TestClassifyTier(t *testing.T) {
	c := &Cascade{}
	cases := []struct {
		query, intent string
		want          int
	}{
		{"Build me an outfit for a wedding", "styling", 3},
		{"Is the satin robe in stock?", "discovery", 1},
		{"What size should I get?", "sizing", 2},
		{"show me black bras", "discovery", 2},
	}
	for _, tc := range cases {
		if got := c.ClassifyTier(tc.query, tc.intent); got != tc.want {
			t.Errorf("ClassifyTier(%q) = %d, want %d", tc.query, got, tc.want)
		}
	}
}
