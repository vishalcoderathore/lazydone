package capture

import (
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		input string
		text  string
		tags  []string
		due   string // in DateLayout, or empty for none
	}{
		{"buy milk", "buy milk", nil, ""},
		{"fix bug +work", "fix bug", []string{"+work"}, ""},
		{"+work fix @home bug", "fix bug", []string{"+work", "@home"}, ""},
		{"call mum due:05-11-2026", "call mum", nil, "05-11-2026"},
		{"due:5-1-2027 pay rent", "pay rent", nil, "05-01-2027"},
		{"ship +work @office due:31-12-2026 now", "ship now", []string{"+work", "@office"}, "31-12-2026"},
		{"repeat +a +a +b", "repeat", []string{"+a", "+b"}, ""},
		{"ok +日本", "ok", []string{"+日本"}, ""},
		{"add +1 to score", "add +1 to score", nil, ""},
		{"mail me@example.com", "mail me@example.com", nil, ""},
		{"lone + and @ signs", "lone + and @ signs", nil, ""},
		{"+-dash", "+-dash", nil, ""},
		{"  spaced \t out\nlines  ", "spaced out lines", nil, ""},
		{"Due:05-11-2026 is text", "Due:05-11-2026 is text", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := Parse(c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Text != c.text {
				t.Errorf("text = %q, want %q", got.Text, c.text)
			}
			if !slices.Equal(got.Tags, c.tags) {
				t.Errorf("tags = %q, want %q", got.Tags, c.tags)
			}
			due := ""
			if !got.Due.IsZero() {
				due = got.Due.Format(DateLayout)
				if h, m, s := got.Due.Clock(); h != 0 || m != 0 || s != 0 {
					t.Errorf("due = %v, want midnight", got.Due)
				}
			}
			if due != c.due {
				t.Errorf("due = %q, want %q", due, c.due)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"":                                  "needs some text",
		"   ":                               "needs some text",
		"+work @home":                       "needs some text",
		"due:05-11-2026":                    "needs some text",
		"pay due:fri":                       `"fri" must be a date in DD-MM-YYYY form`,
		"pay due:2026-11-05":                "DD-MM-YYYY",
		"pay due:11/05/2026":                "DD-MM-YYYY",
		"pay due:31-02-2026":                "DD-MM-YYYY",
		"pay due:05-13-2026":                "DD-MM-YYYY",
		"pay due:":                          "DD-MM-YYYY",
		"pay due:05-11-2026 due:06-11-2026": "only one due",
	}
	for input, message := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			if err == nil || !strings.Contains(err.Error(), message) {
				t.Errorf("err = %v, want one mentioning %q", err, message)
			}
		})
	}
}

// Whatever the input, Parse returns an error or fields with text, never panics,
// and never leaves a tag or due date in the text.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"fix bug +work @home due:05-11-2026", "+", "@", "due:", "a +1 b@c", "\x00+\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := Parse(input)
		if err != nil {
			return
		}
		if got.Text == "" {
			t.Fatal("no error, but empty text")
		}
		for _, word := range strings.Fields(got.Text) {
			if isTag(word) || strings.HasPrefix(word, duePrefix) {
				t.Fatalf("text %q still holds %q", got.Text, word)
			}
		}
	})
}
