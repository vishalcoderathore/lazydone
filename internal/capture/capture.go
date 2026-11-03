// Package capture turns quick-capture text, such as
// "fix bug +work @home due:fri", into the fields of a task, and back. The
// command line and the TUI share it, so that they can't read the same text
// differently.
package capture

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/due"
)

const duePrefix = "due:"

// ErrNoText means the input had no text besides tags and a due date.
var ErrNoText = errors.New("the task needs some text besides tags and a due date")

// Fields are what capture text says about a task.
type Fields struct {
	Text string    // the text with tags and the due date taken out
	Tags []string  // +project and @context tags, in the order given, without repeats
	Due  time.Time // midnight at the start of the due date, or zero for none
}

// Parse reads capture text. A word becomes a tag when it starts with + or @
// followed by a letter, so "+1" and "me@example.com" stay as text. A due:
// word gives the due date, in any form that due.Parse accepts, relative to
// now. Text that is left empty is ErrNoText, and a due date that can't be read
// or is given twice is also an error.
func Parse(input string, now time.Time) (Fields, error) {
	var fields Fields
	var words []string
	for _, word := range strings.Fields(board.CleanText(input)) {
		switch {
		case strings.HasPrefix(word, duePrefix):
			if !fields.Due.IsZero() {
				return Fields{}, errors.New("give only one due: date")
			}
			date, err := due.Parse(strings.TrimPrefix(word, duePrefix), now)
			if err != nil {
				return Fields{}, err
			}
			fields.Due = date
		case isTag(word):
			if !slices.Contains(fields.Tags, word) {
				fields.Tags = append(fields.Tags, word)
			}
		default:
			words = append(words, word)
		}
	}
	if len(words) == 0 {
		return Fields{}, ErrNoText
	}
	fields.Text = strings.Join(words, " ")
	return fields, nil
}

func isTag(word string) bool {
	if word[0] != '+' && word[0] != '@' {
		return false
	}
	next, _ := utf8.DecodeRuneInString(word[1:])
	return unicode.IsLetter(next)
}

// Format writes task fields as capture text that Parse reads back the same,
// such as "fix bug +work due:05-11-2026".
func Format(text string, tags []string, date time.Time) string {
	parts := append([]string{text}, tags...)
	if !date.IsZero() {
		parts = append(parts, duePrefix+date.Format(due.Layout))
	}
	return strings.Join(parts, " ")
}
