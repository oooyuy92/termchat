package ui

import "testing"

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		query  string
		target string
		want   bool
	}{
		{"abc", "abcdef", true},
		{"ace", "abcdef", true},
		{"aec", "abcdef", false},
		{"", "anything", true},
		{"abc", "", false},
		{"dsk", "deepseek", true},
		{"深索", "深度求索", true},
		{"深求", "深度求索", true},
		{"索深", "深度求索", false},
		{"GPT", "gpt-4o", true},
	}
	for _, c := range cases {
		got := fuzzyMatch(c.query, c.target)
		if got != c.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", c.query, c.target, got, c.want)
		}
	}
}
