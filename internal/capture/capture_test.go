package capture

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vishalcoderathore/lazydone/internal/due"
)

// now is a Tuesday.
var now = time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)

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
		{"pay rent due:fri", "pay rent", nil, "06-11-2026"},
		{"due:tomorrow call", "call", nil, "04-11-2026"},
		{"review due:+2w", "review", nil, "17-11-2026"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := Parse(c.input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Text != c.text {
				t.Errorf("text = %q, want %q", got.Text, c.text)
			}
			if !slices.Equal(got.Tags, c.tags) {
				t.Errorf("tags = %q, want %q", got.Tags, c.tags)
			}
			gotDue := ""
			if !got.Due.IsZero() {
				gotDue = got.Due.Format(due.Layout)
				if h, m, s := got.Due.Clock(); h != 0 || m != 0 || s != 0 {
					t.Errorf("due = %v, want midnight", got.Due)
				}
			}
			if gotDue != c.due {
				t.Errorf("due = %q, want %q", gotDue, c.due)
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
		"pay due:someday":                   `"someday" isn't one I know`,
		"pay due:2026-11-05":                "DD-MM-YYYY",
		"pay due:11/05/2026":                "DD-MM-YYYY",
		"pay due:31-02-2026":                "DD-MM-YYYY",
		"pay due:05-13-2026":                "DD-MM-YYYY",
		"pay due:":                          "DD-MM-YYYY",
		"pay due:05-11-2026 due:06-11-2026": "only one due",
	}
	for input, message := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input, now)
			if err == nil || !strings.Contains(err.Error(), message) {
				t.Errorf("err = %v, want one mentioning %q", err, message)
			}
			if strings.Contains(message, "needs some text") && !errors.Is(err, ErrNoText) {
				t.Errorf("err = %v, want ErrNoText", err)
			}
		})
	}
}

func TestFormatIsReadBackTheSame(t *testing.T) {
	date := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		text string
		tags []string
		due  time.Time
		want string
	}{
		{"fix bug", nil, time.Time{}, "fix bug"},
		{"fix bug", []string{"+work", "@home"}, date, "fix bug +work @home due:05-11-2026"},
	} {
		got := Format(c.text, c.tags, c.due)
		if got != c.want {
			t.Errorf("Format = %q, want %q", got, c.want)
		}
		back, err := Parse(got, now)
		if err != nil || back.Text != c.text || !slices.Equal(back.Tags, c.tags) || !back.Due.Equal(c.due) {
			t.Errorf("Parse(%q) = %+v, %v", got, back, err)
		}
	}
}

// Whatever the input, Parse returns an error or fields with text, never panics,
// and never leaves a tag or due date in the text.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"fix bug +work @home due:05-11-2026", "+", "@", "due:", "a +1 b@c", "\x00+\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := Parse(input, now)
		if err != nil {
			return
		}
		if back, err := Parse(Format(got.Text, got.Tags, got.Due), now); err != nil || back.Text != got.Text {
			t.Fatalf("Format of %+v reads back as %+v, %v", got, back, err)
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
