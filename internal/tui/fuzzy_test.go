package tui

import (
	"slices"
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		text, query string
		want        []int // nil for no match
	}{
		{"buy milk", "mlk", []int{4, 6, 7}},
		{"buy milk", "MLK", []int{4, 6, 7}},
		{"buy milk", "b m", []int{0, 4}},
		{"milkshake", "milk", []int{0, 1, 2, 3}},
		{"call bank +money", "money", []int{11, 12, 13, 14, 15}},
		{"abcab", "ab", []int{0, 1}}, // the earliest tight match
		{"buy milk", "klm", nil},
		{"buy milk", "", nil},
		{"buy milk", "   ", nil},
		{"café", "fé", []int{2, 3}},
	}
	for _, c := range cases {
		got, _, ok := fuzzyMatch(c.text, c.query)
		if ok != (c.want != nil) || !slices.Equal(got, c.want) {
			t.Errorf("fuzzyMatch(%q, %q) = %v, %v; want %v", c.text, c.query, got, ok, c.want)
		}
	}
}

func TestFuzzyMatchScoresTighterMatchesHigher(t *testing.T) {
	better := [][2]string{
		// {better match, worse match} for the same query
		{"milk", "mxixlxk"},         // together beats spread out
		{"buy milk", "summarily k"}, // at a word start beats in the middle
		{"call bank +money", "my old notes on everything"},
	}
	queries := []string{"milk", "mk", "money"}
	for i, pair := range better {
		_, high, ok1 := fuzzyMatch(pair[0], queries[i])
		_, low, ok2 := fuzzyMatch(pair[1], queries[i])
		if !ok1 || !ok2 || high <= low {
			t.Errorf("%q: %q scored %d, %q scored %d; want the first higher", queries[i], pair[0], high, pair[1], low)
		}
	}
}

func FuzzFuzzyMatch(f *testing.F) {
	for _, seed := range [][2]string{{"buy milk", "mlk"}, {"café", "É"}, {"", "a"}, {"aaa", "aa"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, text, query string) {
		positions, _, ok := fuzzyMatch(text, query)
		if !ok {
			return
		}
		for i, at := range positions {
			if at < 0 || at >= len([]rune(text)) || (i > 0 && at <= positions[i-1]) {
				t.Fatalf("positions %v out of order or range for %q", positions, text)
			}
		}
	})
}
