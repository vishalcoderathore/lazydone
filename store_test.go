package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// texts describes a board by the text of its tasks, for tests that don't care
// about a task's other fields.
type texts [numColumns][]string

func (ts texts) board() board {
	var b board
	for column, list := range ts {
		for _, text := range list {
			b[column] = append(b[column], task{Text: text})
		}
	}
	return b
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todos.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertBoardsEqual(t *testing.T, got board, want texts) {
	t.Helper()
	var gotTexts texts
	for column, list := range got {
		for _, item := range list {
			gotTexts[column] = append(gotTexts[column], item.Text)
		}
	}
	for i := range numColumns {
		if !slices.Equal(gotTexts[i], want[i]) {
			t.Fatalf("board = %q, want %q", gotTexts, want)
		}
	}
}

func TestLoadMissingFileReturnsEmptyBoard(t *testing.T) {
	got, err := loadBoard(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertBoardsEqual(t, got, texts{})
}

func TestSaveThenLoadKeepsEveryColumnInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	want := texts{
		toDo:       {"buy milk", "日本語"},
		inProgress: {"call the bank"},
		completed:  {"walk the dog", "[done] is not finished"},
	}

	if err := saveBoard(path, want.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, want)
}

func TestSaveThenLoadKeepsEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	created := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	done := time.Date(2026, 11, 3, 11, 0, 0, 0, time.UTC)
	due := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	want := board{completed: {{
		ID:        "abc123",
		Text:      "ship it",
		Priority:  "high",
		Due:       due,
		Tags:      []string{"+work", "@home"},
		Notes:     "first line\nsecond line",
		Archived:  true,
		Created:   created,
		Updated:   updated,
		Completed: done,
	}}}

	if err := saveBoard(path, want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// This pins the file format, so that a change to it can't silently strand the
// data of existing users.
func TestLoadReadsVersionOneFile(t *testing.T) {
	path := writeFile(t, `{
		"version": 1,
		"tasks": [
			{"id": "a", "text": "first", "status": "doing", "created": "2026-11-01T09:00:00Z"},
			{"id": "b", "text": "second", "status": "done"},
			{"id": "c", "text": "third", "status": "todo", "tags": ["+work"]}
		]
	}`)

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	want := board{
		toDo:       {{ID: "c", Text: "third", Tags: []string{"+work"}}},
		inProgress: {{ID: "a", Text: "first", Created: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}},
		completed:  {{ID: "b", Text: "second"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadReturnsErrorForUnreadablePath(t *testing.T) {
	// A directory exists but can't be read as a file.
	if _, err := loadBoard(t.TempDir()); err == nil {
		t.Error("expected an error, got nil")
	}
}

func TestLegacyPlainListGoesToToDo(t *testing.T) {
	got, err := readLegacyBoard(writeFile(t, "buy milk\ncall mum\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"buy milk", "call mum"}})
}

func TestLegacyTagsChooseTheColumn(t *testing.T) {
	got, err := readLegacyBoard(writeFile(t, "[todo] a\n[doing] b\n[done] c\n[todo] [done] is not finished\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"a", "[done] is not finished"}, inProgress: {"b"}, completed: {"c"}})
}

func TestLegacySkipsBlankLinesAndEmptyTasks(t *testing.T) {
	got, err := readLegacyBoard(writeFile(t, "one\n\n   \n[done] \n[doing] two\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"one"}, inProgress: {"two"}})
}

func TestLegacyCleansTabsAndWindowsLineEndings(t *testing.T) {
	got, err := readLegacyBoard(writeFile(t, "[todo] a\tb\r\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"a b"}})
}

func TestLoadRejectsBadFiles(t *testing.T) {
	cases := map[string]struct{ content, message string }{
		"newer version":  {`{"version": 2, "tasks": []}`, "newer version of lazydone"},
		"not json":       {`[todo] buy milk`, "not a valid lazydone data file"},
		"no version":     {`{"tasks": []}`, "not a valid lazydone data file"},
		"unknown status": {`{"version": 1, "tasks": [{"id": "a", "text": "x", "status": "later"}]}`, `unknown status "later"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, c.content)

			_, err := loadBoard(path)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), c.message) || !strings.Contains(err.Error(), path) {
				t.Errorf("error %q should mention %q and the path", err, c.message)
			}
		})
	}
}

func TestFailedSaveKeepsPreviousData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	if err := saveBoard(path, texts{toDo: {"precious"}}.board()); err != nil {
		t.Fatal(err)
	}

	// With a read-only folder, only a save that overwrites the file in place can
	// succeed. A safe save needs to create a file beside it, and so must fail.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := saveBoard(path, texts{toDo: {"replacement"}}.board()); err == nil {
		t.Fatal("expected the save to fail")
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"precious"}})
}

func TestSaveLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	if err := saveBoard(path, texts{toDo: {"a"}}.board()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "tasks.json" {
		t.Errorf("folder holds %v, want only tasks.json", entries)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s has mode %o, want %o", path, got, want)
	}
}

func TestSaveCreatesPrivateFolders(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "share")
	dir := filepath.Join(parent, "lazydone")
	path := filepath.Join(dir, "tasks.json")

	if err := saveBoard(path, texts{toDo: {"a"}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
}

func TestSaveMakesExistingFilePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "tasks": []}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(path, texts{toDo: {"a"}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	assertMode(t, path, 0o600)
}

func TestSaveLeavesExistingFolderPermissionsAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(filepath.Join(dir, "tasks.json"), texts{toDo: {"a"}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	assertMode(t, dir, 0o755)
}

// saveAndAge saves a board whose only task has the given text, then makes the
// newest backup look old enough for the next save to create another.
func saveAndAge(t *testing.T, path, text string) {
	t.Helper()
	if err := saveBoard(path, texts{toDo: {text}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path+".bak.1", old, old); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func assertBackup(t *testing.T, path string, number int, want string) {
	t.Helper()
	got, err := loadBoard(fmt.Sprintf("%s.bak.%d", path, number))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {want}})
}

func TestSaveBacksUpPreviousData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	saveAndAge(t, path, "first")
	saveAndAge(t, path, "second")

	assertBackup(t, path, 1, "first")
	assertMode(t, path+".bak.1", 0o600)
}

func TestRapidSavesShareOneBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	for _, text := range []string{"first", "second", "third"} {
		if err := saveBoard(path, texts{toDo: {text}}.board()); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}

	// Backing up every save would leave only the last few key presses, so the
	// oldest state stays until the backup has aged.
	assertBackup(t, path, 1, "first")
	if _, err := os.Stat(path + ".bak.2"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected one backup, stat error = %v", err)
	}
}

func TestFailedBackupKeepsOlderBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := saveBoard(path, texts{toDo: {"current"}}.board()); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	for number := 1; number <= backupCount; number++ {
		backup := fmt.Sprintf("%s.bak.%d", path, number)
		if err := os.WriteFile(backup, fmt.Appendf(nil, "backup %d", number), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(backup, old, old); err != nil {
			t.Fatal(err)
		}
	}

	// A folder in the way of the new backup makes writing it fail, and nothing
	// else about the save.
	if err := os.MkdirAll(filepath.Join(path+".bak.0", "blocker"), 0o700); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		if err := saveBoard(path, texts{toDo: {"replacement"}}.board()); err == nil {
			t.Fatal("expected the save to fail")
		}
	}

	for number := 1; number <= backupCount; number++ {
		want := fmt.Sprintf("backup %d", number)
		if got := readFile(t, fmt.Sprintf("%s.bak.%d", path, number)); got != want {
			t.Errorf("backup %d holds %q, want %q", number, got, want)
		}
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"current"}})
}

func TestSaveThroughSymlinkUpdatesTheRealFile(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	link := filepath.Join(dir, "link.json")
	if err := saveBoard(real, texts{toDo: {"old"}}.board()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(link, texts{toDo: {"new"}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link should still be a symlink")
	}
	got, err := loadBoard(real)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"new"}})
}

func TestKeepsFiveMostRecentBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	for _, text := range []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8"} {
		saveAndAge(t, path, text)
	}

	assertBackup(t, path, 1, "v7")
	assertBackup(t, path, 5, "v3")
	if _, err := os.Stat(path + ".bak.6"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a sixth backup should not exist, stat error = %v", err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"v8"}})
}

func assertStillSymlink(t *testing.T, link string) {
	t.Helper()
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link should still be a symlink")
	}
}

func TestSaveThroughDanglingSymlinkCreatesTheTarget(t *testing.T) {
	cases := map[string]func(dir string) (target string, linkTo string){
		"absolute": func(dir string) (string, string) {
			target := filepath.Join(dir, "real.json")
			return target, target
		},
		"relative": func(dir string) (string, string) {
			return filepath.Join(dir, "real.json"), "real.json"
		},
		"missing folder": func(dir string) (string, string) {
			target := filepath.Join(dir, "sync", "lazydone", "real.json")
			return target, target
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			target, linkTo := setup(dir)
			link := filepath.Join(dir, "link.json")
			if err := os.Symlink(linkTo, link); err != nil {
				t.Fatal(err)
			}

			if err := saveBoard(link, texts{toDo: {"new"}}.board()); err != nil {
				t.Fatalf("save failed: %v", err)
			}

			assertStillSymlink(t, link)
			got, err := loadBoard(target)
			if err != nil {
				t.Fatalf("load failed: %v", err)
			}
			assertBoardsEqual(t, got, texts{toDo: {"new"}})
		})
	}
}

func TestSaveThroughChainOfDanglingSymlinks(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.json"), filepath.Join(dir, "second.json")
	target := filepath.Join(dir, "real.json")
	if err := os.Symlink(second, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, second); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(first, texts{toDo: {"new"}}.board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	assertStillSymlink(t, first)
	assertStillSymlink(t, second)
	got, err := loadBoard(target)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"new"}})
}

func TestSaveThroughSymlinkLoopFailsAndKeepsTheLink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(link, link); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(link, texts{toDo: {"new"}}.board()); err == nil {
		t.Error("a link that points at itself should be an error")
	}
	assertStillSymlink(t, link)
}
