package tui

import (
	"strings"
	"unicode"
)

// Scores for fuzzyMatch, in the style of fzf: every matched letter counts, a
// letter right after the one before counts more, and so does one that starts
// a word. Each letter skipped between matches costs a little.
const (
	scoreMatch       = 16
	bonusConsecutive = 8
	bonusWordStart   = 10
	penaltyGapStart  = 3
	penaltyGapExtend = 1
)

// fuzzyMatch reports whether the letters of query appear in order in text,
// ignoring case and spaces in the query. If so, it also returns where they
// matched, as rune positions in text, and a score: higher is a better match.
//
// It finds the earliest place the whole query can end, then walks back from
// there for the latest place it can start, so the match is as tight as it can
// be without searching every possibility.
func fuzzyMatch(text, query string) (positions []int, score int, ok bool) {
	var pattern []rune
	for _, r := range strings.ToLower(query) {
		if r != ' ' {
			pattern = append(pattern, r)
		}
	}
	if len(pattern) == 0 {
		return nil, 0, false
	}
	runes := []rune(text)
	lower := make([]rune, len(runes))
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}

	// Forward: the earliest position where the last letter can match.
	end, p := -1, 0
	for i, r := range lower {
		if r == pattern[p] {
			p++
			if p == len(pattern) {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return nil, 0, false
	}

	// Backward from there: match the letters latest first, which keeps them
	// close together.
	positions = make([]int, len(pattern))
	p = len(pattern) - 1
	for i := end; p >= 0; i-- {
		if lower[i] == pattern[p] {
			positions[p] = i
			p--
		}
	}

	for n, at := range positions {
		score += scoreMatch
		if at == 0 || !isWordRune(runes[at-1]) {
			score += bonusWordStart
		}
		if n > 0 {
			if gap := at - positions[n-1] - 1; gap == 0 {
				score += bonusConsecutive
			} else {
				score -= penaltyGapStart + penaltyGapExtend*(gap-1)
			}
		}
	}
	return positions, score, true
}

// isWordRune reports whether r is part of a word, so a letter after anything
// else, such as a space or the + of a tag, starts one.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
