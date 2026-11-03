package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/boardtest"
	"github.com/vishalcoderathore/lazydone/internal/due"
	"github.com/vishalcoderathore/lazydone/internal/store"
)

type texts = boardtest.Texts

var (
	assertBoardsEqual = boardtest.AssertEqual
	readFile          = boardtest.ReadFile
	writeFile         = boardtest.WriteFile
)

func loadBoard(path string) (board.Board, error) { return store.File{Path: path}.Load() }

func saveBoard(path string, tasks board.Board) error { return store.File{Path: path}.Save(tasks) }

// cliResult is what one run of the command line did.
type cliResult struct {
	stdout, stderr string
	err            error
	opened         bool        // whether the TUI was opened
	path           string      // the data file it was given
	tasks          board.Board // the tasks it was given
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var result cliResult
	result.err = run(args, &stdout, &stderr, func(path string, tasks board.Board) error {
		result.opened, result.path, result.tasks = true, path, tasks
		return nil
	})
	result.stdout, result.stderr = stdout.String(), stderr.String()
	return result
}

// useDataHome points the default data location at a new temporary folder and
// returns the data file that should end up in it.
func useDataHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", home)
	return filepath.Join(home, "lazydone", "tasks.json")
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s should not exist, stat error = %v", path, err)
	}
}

func TestVersionFlagPrintsVersion(t *testing.T) {
	original := version
	version = "v0.1.0-alpha.1"
	t.Cleanup(func() { version = original })

	result := runCLI(t, "--version")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if want := "lazydone v0.1.0-alpha.1\n"; result.stdout != want {
		t.Errorf("stdout = %q, want %q", result.stdout, want)
	}
}

func TestVersionFlagDoesNotOpenTUIOrTouchData(t *testing.T) {
	path := useDataHome(t)

	result := runCLI(t, "--version")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.opened {
		t.Error("the TUI should not open")
	}
	assertNotExist(t, path)
}

func TestUnknownFlagIsAnError(t *testing.T) {
	useDataHome(t)

	result := runCLI(t, "--nope")

	if result.err == nil {
		t.Fatal("run should fail on an unknown flag")
	}
	if result.opened {
		t.Error("the TUI should not open")
	}
	if !strings.Contains(result.stderr, "--version") {
		t.Errorf("stderr should show usage, got %q", result.stderr)
	}
}

func TestUnknownCommandIsAnError(t *testing.T) {
	useDataHome(t)

	result := runCLI(t, "nonsense")

	if result.err == nil {
		t.Fatal("run should fail on an unknown command")
	}
	if result.opened {
		t.Error("the TUI should not open")
	}
}

func TestHelpFlagShowsUsage(t *testing.T) {
	useDataHome(t)

	result := runCLI(t, "--help")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.opened {
		t.Error("the TUI should not open")
	}
	for _, want := range []string{"--version", "--file", "import", "add", "ls", "count", "completion"} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("help should mention %q, got %q", want, result.stdout)
		}
	}
}

func TestOpensDataFileInXDGDataHome(t *testing.T) {
	want := useDataHome(t)

	result := runCLI(t)

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if !result.opened || result.path != want {
		t.Errorf("opened = %v, path = %q, want %q", result.opened, result.path, want)
	}
}

func TestDataFileFallsBackToHome(t *testing.T) {
	for name, xdg := range map[string]string{"unset": "", "relative": "not/absolute"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", xdg)

			result := runCLI(t)

			want := filepath.Join(home, ".local", "share", "lazydone", "tasks.json")
			if result.path != want {
				t.Errorf("path = %q, want %q", result.path, want)
			}
		})
	}
}

func TestDataFileIsTheSameFromAnyFolder(t *testing.T) {
	want := useDataHome(t)
	t.Chdir(t.TempDir())

	if result := runCLI(t); result.path != want {
		t.Errorf("path = %q, want %q", result.path, want)
	}
}

func TestFileFlagOverridesDataFile(t *testing.T) {
	defaultPath := useDataHome(t)
	path := filepath.Join(t.TempDir(), "other.json")
	if err := saveBoard(path, texts{board.InProgress: {"elsewhere"}}.Board()); err != nil {
		t.Fatal(err)
	}

	result := runCLI(t, "--file", path)

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.path != path {
		t.Errorf("path = %q, want %q", result.path, path)
	}
	assertBoardsEqual(t, result.tasks, texts{board.InProgress: {"elsewhere"}})
	assertNotExist(t, defaultPath)
}

func TestBadDataFileStopsTheAppAndStaysIntact(t *testing.T) {
	const content = `{"version": 99, "tasks": []}`
	path := writeFile(t, content)

	result := runCLI(t, "--file", path)

	if result.err == nil || !strings.Contains(result.err.Error(), "newer version") {
		t.Errorf("err = %v, want a newer version error", result.err)
	}
	if result.opened {
		t.Error("the TUI should not open")
	}
	if got := readFile(t, path); got != content {
		t.Errorf("file changed to %q", got)
	}
}

func TestImportMovesLegacyTasksToDataFile(t *testing.T) {
	path := useDataHome(t)
	source := writeFile(t, "[todo] a\n[doing] b\n[done] c\n")

	result := runCLI(t, "import", source)

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.opened {
		t.Error("import should not open the TUI")
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"a"}, board.InProgress: {"b"}, board.Completed: {"c"}})
	if !strings.Contains(result.stdout, "Imported 3 tasks") || !strings.Contains(result.stdout, path) {
		t.Errorf("stdout = %q, should say what happened and where", result.stdout)
	}
}

func TestImportGivesTasksIDsAndTimestamps(t *testing.T) {
	path := useDataHome(t)

	if result := runCLI(t, "import", writeFile(t, "[todo] a\n[done] b\n")); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	open, done := got[board.ToDo][0], got[board.Completed][0]
	if open.ID == "" || done.ID == "" || open.ID == done.ID {
		t.Errorf("IDs should be set and differ: %q, %q", open.ID, done.ID)
	}
	if open.Created.IsZero() || done.Created.IsZero() {
		t.Error("imported tasks should have a created time")
	}
	if !open.Completed.IsZero() {
		t.Error("an open task should not be board.Completed")
	}
	if done.Completed.IsZero() {
		t.Error("a done task should have a board.Completed time")
	}
}

func TestImportLeavesOriginalUntouched(t *testing.T) {
	useDataHome(t)
	const content = "[todo] a\nplain\n"
	source := writeFile(t, content)

	if result := runCLI(t, "import", source); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if got := readFile(t, source); got != content {
		t.Errorf("original changed to %q", got)
	}
}

func TestImportAddsToExistingTasks(t *testing.T) {
	path := useDataHome(t)
	if err := saveBoard(path, texts{board.ToDo: {"existing"}}.Board()); err != nil {
		t.Fatal(err)
	}

	if result := runCLI(t, "import", writeFile(t, "new\n")); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"existing", "new"}})
}

func TestImportHonorsFileFlag(t *testing.T) {
	defaultPath := useDataHome(t)
	path := filepath.Join(t.TempDir(), "other.json")

	if result := runCLI(t, "import", writeFile(t, "a\n"), "--file", path); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"a"}})
	assertNotExist(t, defaultPath)
}

func TestImportOfMissingFileFailsWithoutCreatingData(t *testing.T) {
	path := useDataHome(t)

	result := runCLI(t, "import", filepath.Join(t.TempDir(), "missing.txt"))

	if result.err == nil {
		t.Fatal("import of a missing file should fail")
	}
	assertNotExist(t, path)
}

func TestImportDoesNotOverwriteBadDataFile(t *testing.T) {
	const content = "not json"
	path := writeFile(t, content)

	result := runCLI(t, "import", writeFile(t, "a\n"), "--file", path)

	if result.err == nil {
		t.Fatal("import into a bad data file should fail")
	}
	if got := readFile(t, path); got != content {
		t.Errorf("file changed to %q", got)
	}
}

func TestImportNeedsAPath(t *testing.T) {
	useDataHome(t)

	if result := runCLI(t, "import"); result.err == nil {
		t.Error("import without a path should fail")
	}
}

// seed saves a board to the default data file.
func seed(t *testing.T, tasks board.Board) string {
	t.Helper()
	path := useDataHome(t)
	if err := saveBoard(path, tasks); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddSavesTaskWithTagsAndDueDate(t *testing.T) {
	path := useDataHome(t)

	result := runCLI(t, "add", "fix", "bug", "+work", "@home", "due:05-11-2026")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.opened {
		t.Error("add should not open the TUI")
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"fix bug"}})
	added := got[board.ToDo][0]
	if !slices.Equal(added.Tags, []string{"+work", "@home"}) {
		t.Errorf("tags = %q", added.Tags)
	}
	if added.Due.Format("02-01-2006") != "05-11-2026" {
		t.Errorf("due = %v", added.Due)
	}
	if added.ID == "" || added.Created.IsZero() {
		t.Error("the task should have an ID and a created time")
	}
	if want := "Added: [todo] fix bug +work @home due:05-11-2026\n"; result.stdout != want {
		t.Errorf("stdout = %q, want %q", result.stdout, want)
	}
}

func TestAddTakesTaskAsOneQuotedArgument(t *testing.T) {
	path := useDataHome(t)

	if result := runCLI(t, "add", "call mum +family"); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"call mum"}})
}

func TestAddKeepsExistingTasks(t *testing.T) {
	path := seed(t, texts{board.ToDo: {"old"}, board.Completed: {"done"}}.Board())

	if result := runCLI(t, "add", "new"); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	assertBoardsEqual(t, got, texts{board.ToDo: {"old", "new"}, board.Completed: {"done"}})
}

func TestAddRejectsBadInputWithoutWriting(t *testing.T) {
	for name, args := range map[string][]string{
		"no arguments":   {"add"},
		"only tags":      {"add", "+work", "@home"},
		"unknown due":    {"add", "pay", "due:someday"},
		"ISO due date":   {"add", "pay", "due:2026-11-05"},
		"impossible day": {"add", "pay", "due:31-02-2026"},
	} {
		t.Run(name, func(t *testing.T) {
			path := useDataHome(t)

			if result := runCLI(t, args...); result.err == nil {
				t.Error("add should fail")
			}
			assertNotExist(t, path)
		})
	}
}

// listBoard has a task in each column, an archived task and a task with every
// field that ls shows.
func listBoard() board.Board {
	due := time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local)
	return board.Board{
		board.ToDo: {
			{ID: "a", Text: "fix bug", Tags: []string{"+work"}, Due: due, Priority: "high"},
			{ID: "b", Text: "hidden", Archived: true},
		},
		board.InProgress: {{ID: "c", Text: "call mum"}},
		board.Completed:  {{ID: "d", Text: "walk dog"}},
	}
}

func TestLsListsTasksInBoardOrder(t *testing.T) {
	seed(t, listBoard())

	result := runCLI(t, "ls")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	want := "[todo] fix bug +work due:05-11-2026 priority:high\n[doing] call mum\n[done] walk dog\n"
	if result.stdout != want {
		t.Errorf("stdout = %q, want %q", result.stdout, want)
	}
}

func TestStatusFlagsChooseColumns(t *testing.T) {
	seed(t, listBoard())
	cases := map[string]struct {
		flags     []string
		ls, count string
	}{
		"todo":     {[]string{"--todo"}, "[todo] fix bug +work due:05-11-2026 priority:high\n", "1\n"},
		"doing":    {[]string{"--doing"}, "[doing] call mum\n", "1\n"},
		"done":     {[]string{"--done"}, "[done] walk dog\n", "1\n"},
		"two":      {[]string{"--doing", "--done"}, "[doing] call mum\n[done] walk dog\n", "2\n"},
		"no flags": {nil, "[todo] fix bug +work due:05-11-2026 priority:high\n[doing] call mum\n[done] walk dog\n", "3\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := runCLI(t, append([]string{"ls"}, c.flags...)...); got.err != nil || got.stdout != c.ls {
				t.Errorf("ls stdout = %q, err = %v, want %q", got.stdout, got.err, c.ls)
			}
			if got := runCLI(t, append([]string{"count"}, c.flags...)...); got.err != nil || got.stdout != c.count {
				t.Errorf("count stdout = %q, err = %v, want %q", got.stdout, got.err, c.count)
			}
		})
	}
}

func TestLsJSON(t *testing.T) {
	seed(t, listBoard())

	result := runCLI(t, "ls", "--json", "--todo", "--doing")

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	var got []struct {
		ID, Text, Status, Priority string
		Tags                       []string
		Due                        time.Time
	}
	if err := json.Unmarshal([]byte(result.stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, result.stdout)
	}
	if len(got) != 2 || got[0].ID != "a" || got[0].Status != "todo" || got[1].ID != "c" || got[1].Status != "doing" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Priority != "high" || !slices.Equal(got[0].Tags, []string{"+work"}) || got[0].Due.Day() != 5 {
		t.Errorf("fields missing from %+v", got[0])
	}
}

func TestLsAndCountOnAnEmptyBoard(t *testing.T) {
	path := useDataHome(t)

	for args, want := range map[string]string{"ls": "", "ls --json": "[]\n", "count": "0\n"} {
		result := runCLI(t, strings.Fields(args)...)
		if result.err != nil || result.stdout != want {
			t.Errorf("%s: stdout = %q, err = %v, want %q", args, result.stdout, result.err, want)
		}
	}
	assertNotExist(t, path)
}

func TestLsReportsABadDataFile(t *testing.T) {
	path := writeFile(t, "not json")

	if result := runCLI(t, "ls", "--file", path); result.err == nil {
		t.Error("ls should fail on a bad data file")
	}
}

func TestCompletionScripts(t *testing.T) {
	useDataHome(t)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			result := runCLI(t, "completion", shell)
			if result.err != nil || !strings.Contains(result.stdout, "lazydone") {
				t.Errorf("err = %v, stdout starts %.80q", result.err, result.stdout)
			}
		})
	}
}

// Several processes adding at once must not lose any task. The test runs
// itself again as the child processes.
func TestConcurrentAddsFromManyProcessesLoseNothing(t *testing.T) {
	const childEnv, processes, each = "LAZYDONE_ADD_CHILD", 6, 10
	if path := os.Getenv(childEnv); path != "" {
		for i := range each {
			if result := runCLI(t, "add", fmt.Sprintf("task %d-%d", os.Getpid(), i), "--file", path); result.err != nil {
				t.Fatal(result.err)
			}
		}
		return
	}

	path := filepath.Join(t.TempDir(), "tasks.json")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	children := make([]*exec.Cmd, processes)
	outputs := make([]*bytes.Buffer, processes)
	for i := range children {
		outputs[i] = &bytes.Buffer{}
		children[i] = exec.Command(binary, "-test.run=^TestConcurrentAddsFromManyProcessesLoseNothing$")
		children[i].Env = append(os.Environ(), childEnv+"="+path)
		children[i].Stdout, children[i].Stderr = outputs[i], outputs[i]
		if err := children[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i, child := range children {
		if err := child.Wait(); err != nil {
			t.Fatalf("child %d: %v\n%s", i, err, outputs[i])
		}
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got[board.ToDo]); n != processes*each {
		t.Errorf("%d tasks saved, want %d", n, processes*each)
	}
}

func TestAddReadsNaturalDueDates(t *testing.T) {
	path := useDataHome(t)

	if result := runCLI(t, "add", "pay", "rent", "due:fri"); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	got, err := loadBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := due.Parse("fri", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if added := got[board.ToDo][0]; !added.Due.Equal(want) {
		t.Errorf("due = %v, want %v", added.Due, want)
	}
}

// todayBoard has tasks on either side of today's boundaries.
func todayBoard() board.Board {
	today := due.Day(time.Now())
	return board.Board{
		board.ToDo: {
			{ID: "late", Text: "late", Due: today.AddDate(0, 0, -2)},
			{ID: "soon", Text: "soon", Due: today.AddDate(0, 0, 1)},
			{ID: "none", Text: "none"},
		},
		board.InProgress: {{ID: "now", Text: "now", Due: today}},
		board.Completed: {
			{ID: "fresh", Text: "fresh", Completed: time.Now()},
			{ID: "stale", Text: "stale", Completed: today.Add(-time.Hour), Due: today},
			{ID: "put away", Text: "put away", Completed: time.Now(), Archived: true},
		},
	}
}

func TestTodayFlagMatchesTheTodayView(t *testing.T) {
	seed(t, todayBoard())
	cases := map[string]struct{ ls, count string }{
		"ls --today":                {"late\nnow\nfresh\n", "3\n"},
		"ls --today --todo --doing": {"late\nnow\n", "2\n"},
		"ls --today --done":         {"fresh\n", "1\n"},
	}
	for args, want := range cases {
		t.Run(args, func(t *testing.T) {
			result := runCLI(t, strings.Fields(args)...)
			if result.err != nil {
				t.Fatalf("run: %v", result.err)
			}
			var texts []string
			for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
				fields := strings.Fields(line)
				texts = append(texts, fields[1])
			}
			if got := strings.Join(texts, "\n") + "\n"; got != want.ls {
				t.Errorf("ls gave %q, want %q", got, want.ls)
			}
			countArgs := append([]string{"count"}, strings.Fields(args)[1:]...)
			if got := runCLI(t, countArgs...); got.stdout != want.count {
				t.Errorf("count gave %q, want %q", got.stdout, want.count)
			}
		})
	}
}
