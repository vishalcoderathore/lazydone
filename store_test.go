package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todos.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertBoardsEqual(t *testing.T, got, want board) {
	t.Helper()
	for i := range numColumns {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("board = %q, want %q", got, want)
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("board = %q, want %q", got, want)
			}
		}
	}
}

func TestLoadMissingFileReturnsEmptyBoard(t *testing.T) {
	got, err := loadBoard(filepath.Join(t.TempDir(), "missing.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertBoardsEqual(t, got, board{})
}

func TestSaveThenLoadKeepsEveryColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.txt")
	want := board{
		toDo:       {"buy milk", "日本語"},
		inProgress: {"call the bank"},
		completed:  {"walk the dog"},
	}

	if err := saveBoard(path, want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, want)
}

func TestTaskThatLooksLikeATagSurvivesSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.txt")
	want := board{toDo: {"[done] is not finished"}}

	if err := saveBoard(path, want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, want)
}

func TestLoadPlainListGoesToToDo(t *testing.T) {
	got, err := loadBoard(writeFile(t, "buy milk\ncall mum\n"))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, board{toDo: {"buy milk", "call mum"}})
}

func TestLoadSkipsBlankLinesAndEmptyTasks(t *testing.T) {
	got, err := loadBoard(writeFile(t, "one\n\n   \n[done] \n[doing] two\n"))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, board{toDo: {"one"}, inProgress: {"two"}})
}

func TestLoadCleansTabsAndWindowsLineEndings(t *testing.T) {
	got, err := loadBoard(writeFile(t, "[todo] a\tb\r\n"))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, board{toDo: {"a b"}})
}

func TestLoadReturnsErrorForUnreadablePath(t *testing.T) {
	// A directory exists but can't be read as a file.
	if _, err := loadBoard(t.TempDir()); err == nil {
		t.Error("expected an error, got nil")
	}
}
