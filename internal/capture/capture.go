// Package capture turns quick-capture text, such as
// "fix bug +work @home due:05-11-2026", into the fields of a task. The command
// line and the TUI share it, so that they can't read the same text differently.
package capture

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// DateLayout is how due dates are written: DD-MM-YYYY. Parsing also accepts a
// single-digit day or month, such as 5-1-2027.
const (
	DateLayout  = "02-01-2006"
	parseLayout = "2-1-2006"
)

const duePrefix = "due:"

// Fields are what capture text says about a task.
type Fields struct {
	Text string    // the text with tags and the due date taken out
	Tags []string  // +project and @context tags, in the order given, without repeats
	Due  time.Time // midnight local time on the due date, or zero for none
}

// Parse reads capture text. A word becomes a tag when it starts with + or @
// followed by a letter, so "+1" and "me@example.com" stay as text. A due:
// word gives the due date. Text that is left empty is an error, as is a due
// date that isn't a real DD-MM-YYYY date or is given twice.
func Parse(input string) (Fields, error) {
	var fields Fields
	var words []string
	for _, word := range strings.Fields(board.CleanText(input)) {
		switch {
		case strings.HasPrefix(word, duePrefix):
			if !fields.Due.IsZero() {
				return Fields{}, errors.New("give only one due: date")
			}
			due, err := parseDate(strings.TrimPrefix(word, duePrefix))
			if err != nil {
				return Fields{}, err
			}
			fields.Due = due
		case isTag(word):
			if !slices.Contains(fields.Tags, word) {
				fields.Tags = append(fields.Tags, word)
			}
		default:
			words = append(words, word)
		}
	}
	if len(words) == 0 {
		return Fields{}, errors.New("the task needs some text besides tags and a due date")
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

func parseDate(value string) (time.Time, error) {
	due, err := time.ParseInLocation(parseLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("due date %q must be a date in DD-MM-YYYY form, such as 05-11-2026", value)
	}
	return due, nil
}
