// Package boardtest has helpers for tests that build and compare boards.
package boardtest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// Texts describes a board by the text of its tasks, for tests that don't care
// about a task's other fields.
type Texts [board.NumColumns][]string

// Board returns a board holding tasks with these texts.
func (ts Texts) Board() board.Board {
	var b board.Board
	for column, list := range ts {
		for _, text := range list {
			b[column] = append(b[column], board.Task{Text: text})
		}
	}
	return b
}

// AssertEqual fails the test unless the board holds tasks with exactly these
// texts, in this order.
func AssertEqual(t testing.TB, got board.Board, want Texts) {
	t.Helper()
	var gotTexts Texts
	for column, list := range got {
		for _, item := range list {
			gotTexts[column] = append(gotTexts[column], item.Text)
		}
	}
	for i := range board.NumColumns {
		if !slices.Equal(gotTexts[i], want[i]) {
			t.Fatalf("board = %q, want %q", gotTexts, want)
		}
	}
}

// ReadFile returns the contents of a file, failing the test if it can't be
// read.
func ReadFile(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// WriteFile writes content to a new file in a temporary folder and returns its
// path.
func WriteFile(t testing.TB, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todos.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
