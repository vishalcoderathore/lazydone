package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/boardtest"
)

type texts = boardtest.Texts

var (
	assertBoardsEqual = boardtest.AssertEqual
	readFile          = boardtest.ReadFile
	writeFile         = boardtest.WriteFile
)

func loadBoard(path string) (board.Board, error) { return File{Path: path}.Load() }

func saveBoard(path string, tasks board.Board) error { return File{Path: path}.Save(tasks) }

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
		board.ToDo:       {"buy milk", "日本語"},
		board.InProgress: {"call the bank"},
		board.Completed:  {"walk the dog", "[done] is not finished"},
	}

	if err := saveBoard(path, want.Board()); err != nil {
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
	want := board.Board{board.Completed: {{
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
	want := board.Board{
		board.ToDo:       {{ID: "c", Text: "third", Tags: []string{"+work"}}},
		board.InProgress: {{ID: "a", Text: "first", Created: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}},
		board.Completed:  {{ID: "b", Text: "second"}},
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
	got, err := ReadLegacy(writeFile(t, "buy milk\ncall mum\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"buy milk", "call mum"}})
}

func TestLegacyTagsChooseTheColumn(t *testing.T) {
	got, err := ReadLegacy(writeFile(t, "[todo] a\n[doing] b\n[done] c\n[todo] [done] is not finished\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"a", "[done] is not finished"}, board.InProgress: {"b"}, board.Completed: {"c"}})
}

func TestLegacySkipsBlankLinesAndEmptyTasks(t *testing.T) {
	got, err := ReadLegacy(writeFile(t, "one\n\n   \n[done] \n[doing] two\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"one"}, board.InProgress: {"two"}})
}

func TestLegacyCleansTabsAndWindowsLineEndings(t *testing.T) {
	got, err := ReadLegacy(writeFile(t, "[todo] a\tb\r\n"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"a b"}})
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
	if err := saveBoard(path, texts{board.ToDo: {"precious"}}.Board()); err != nil {
		t.Fatal(err)
	}

	// With a read-only folder, only a save that overwrites the file in place can
	// succeed. A safe save needs to create a file beside it, and so must fail.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := saveBoard(path, texts{board.ToDo: {"replacement"}}.Board()); err == nil {
		t.Fatal("expected the save to fail")
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"precious"}})
}

func TestSaveLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	if err := saveBoard(path, texts{board.ToDo: {"a"}}.Board()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{"tasks.json", "tasks.json.lock"}; !slices.Equal(names, want) {
		t.Errorf("folder holds %q, want %q", names, want)
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

	if err := saveBoard(path, texts{board.ToDo: {"a"}}.Board()); err != nil {
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

	if err := saveBoard(path, texts{board.ToDo: {"a"}}.Board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	assertMode(t, path, 0o600)
}

func TestSaveLeavesExistingFolderPermissionsAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(filepath.Join(dir, "tasks.json"), texts{board.ToDo: {"a"}}.Board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	assertMode(t, dir, 0o755)
}

// saveAndAge saves a board whose only task has the given text, then makes the
// newest backup look old enough for the next save to create another.
func saveAndAge(t *testing.T, path, text string) {
	t.Helper()
	if err := saveBoard(path, texts{board.ToDo: {text}}.Board()); err != nil {
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
	assertBoardsEqual(t, got, texts{board.ToDo: {want}})
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
		if err := saveBoard(path, texts{board.ToDo: {text}}.Board()); err != nil {
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
	if err := saveBoard(path, texts{board.ToDo: {"current"}}.Board()); err != nil {
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
		if err := saveBoard(path, texts{board.ToDo: {"replacement"}}.Board()); err == nil {
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
	assertBoardsEqual(t, got, texts{board.ToDo: {"current"}})
}

func TestSaveThroughSymlinkUpdatesTheRealFile(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	link := filepath.Join(dir, "link.json")
	if err := saveBoard(real, texts{board.ToDo: {"old"}}.Board()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(link, texts{board.ToDo: {"new"}}.Board()); err != nil {
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
	assertBoardsEqual(t, got, texts{board.ToDo: {"new"}})
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
	assertBoardsEqual(t, got, texts{board.ToDo: {"v8"}})
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

			if err := saveBoard(link, texts{board.ToDo: {"new"}}.Board()); err != nil {
				t.Fatalf("save failed: %v", err)
			}

			assertStillSymlink(t, link)
			got, err := loadBoard(target)
			if err != nil {
				t.Fatalf("load failed: %v", err)
			}
			assertBoardsEqual(t, got, texts{board.ToDo: {"new"}})
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

	if err := saveBoard(first, texts{board.ToDo: {"new"}}.Board()); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	assertStillSymlink(t, first)
	assertStillSymlink(t, second)
	got, err := loadBoard(target)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"new"}})
}

func TestSaveThroughSymlinkLoopFailsAndKeepsTheLink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(link, link); err != nil {
		t.Fatal(err)
	}

	if err := saveBoard(link, texts{board.ToDo: {"new"}}.Board()); err == nil {
		t.Error("a link that points at itself should be an error")
	}
	assertStillSymlink(t, link)
}

func addTask(text string) func(board.Board) board.Board {
	return func(b board.Board) board.Board { return b.Add(board.NewTask(text, time.Now())) }
}

func TestUpdateAppliesChangeToLatestData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := saveBoard(path, texts{board.ToDo: {"saved elsewhere"}}.Board()); err != nil {
		t.Fatal(err)
	}

	got, err := File{Path: path}.Update(addTask("new"))
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"saved elsewhere", "new"}})
	saved, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	assertBoardsEqual(t, saved, texts{board.ToDo: {"saved elsewhere", "new"}})
}

func TestConcurrentUpdatesLoseNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	const writers, each = 8, 15

	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := range writers {
		wg.Go(func() {
			for i := range each {
				if _, err := (File{Path: path}).Update(addTask(fmt.Sprintf("%d-%d", w, i))); err != nil {
					errs <- err
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got[board.ToDo]); n != writers*each {
		t.Errorf("%d tasks saved, want %d", n, writers*each)
	}
}

func TestUpdateDoesNotWriteOverBadFile(t *testing.T) {
	const content = "not json"
	path := writeFile(t, content)

	if _, err := (File{Path: path}).Update(addTask("new")); err == nil {
		t.Fatal("expected an error")
	}
	if got := readFile(t, path); got != content {
		t.Errorf("file changed to %q", got)
	}
}

func TestUpdateThroughSymlinkLocksTheRealFile(t *testing.T) {
	dir := t.TempDir()
	real, link := filepath.Join(dir, "real.json"), filepath.Join(dir, "link.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if _, err := (File{Path: link}).Update(addTask("new")); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if _, err := os.Stat(real + ".lock"); err != nil {
		t.Errorf("the real file's lock should be used: %v", err)
	}
	assertNotExist(t, link+".lock")
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s should not exist, stat error = %v", path, err)
	}
}

func TestTasksWithoutIDsGetTheSameIDsOnEveryLoad(t *testing.T) {
	path := writeFile(t, `{"version": 1, "tasks": [
		{"text": "same", "status": "todo"},
		{"text": "same", "status": "todo"},
		{"text": "same", "status": "done"}
	]}`)

	first, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("loads disagree: %+v and %+v", first, second)
	}
	ids := []string{first[board.ToDo][0].ID, first[board.ToDo][1].ID, first[board.Completed][0].ID}
	if ids[0] == "" || ids[0] == ids[1] || ids[0] == ids[2] || ids[1] == ids[2] {
		t.Errorf("IDs should be set and differ: %q", ids)
	}

	// The next save writes the IDs down, so they stay as the board changes.
	updated, err := File{Path: path}.Update(func(b board.Board) board.Board {
		b, _ = b.Delete(ids[0])
		return b
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := updated[board.ToDo][0].ID; got != ids[1] {
		t.Errorf("remaining task has ID %q, want %q", got, ids[1])
	}
	if !strings.Contains(readFile(t, path), ids[1]) {
		t.Error("the save should write the IDs")
	}
}

func TestModTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if got, err := (File{Path: path}).ModTime(); err != nil || !got.IsZero() {
		t.Errorf("missing file: got %v, %v, want zero time", got, err)
	}

	if err := saveBoard(path, texts{board.ToDo: {"a"}}.Board()); err != nil {
		t.Fatal(err)
	}
	first, err := File{Path: path}.ModTime()
	if err != nil || first.IsZero() {
		t.Fatalf("got %v, %v, want a time", first, err)
	}
	if err := os.Chtimes(path, first.Add(-time.Minute), first.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := saveBoard(path, texts{board.ToDo: {"b"}}.Board()); err != nil {
		t.Fatal(err)
	}
	if second, _ := (File{Path: path}).ModTime(); second.Equal(first.Add(-time.Minute)) {
		t.Error("a save should change the time")
	}
}

// Whatever a data file holds, loading returns a board or an error, never panics.
func FuzzLoad(f *testing.F) {
	f.Add([]byte(`{"version": 1, "tasks": [{"id": "a", "text": "x", "status": "doing"}]}`))
	f.Add([]byte(`{"version": 1, "tasks": [{"text": "x", "status": "todo", "due": "bad"}]}`))
	f.Add([]byte(`{"version": 2}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "tasks.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = loadBoard(path)
	})
}
