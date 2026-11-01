package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cliResult is what one run of the command line did.
type cliResult struct {
	stdout, stderr string
	err            error
	opened         bool   // whether the TUI was opened
	path           string // the data file it was given
	tasks          board  // the tasks it was given
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var result cliResult
	result.err = run(args, &stdout, &stderr, func(path string, tasks board) error {
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
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
	for _, want := range []string{"--version", "--file", "import"} {
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
	if err := saveBoard(path, texts{inProgress: {"elsewhere"}}.board()); err != nil {
		t.Fatal(err)
	}

	result := runCLI(t, "--file", path)

	if result.err != nil {
		t.Fatalf("run: %v", result.err)
	}
	if result.path != path {
		t.Errorf("path = %q, want %q", result.path, path)
	}
	assertBoardsEqual(t, result.tasks, texts{inProgress: {"elsewhere"}})
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
	assertBoardsEqual(t, got, texts{toDo: {"a"}, inProgress: {"b"}, completed: {"c"}})
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
	open, done := got[toDo][0], got[completed][0]
	if open.ID == "" || done.ID == "" || open.ID == done.ID {
		t.Errorf("IDs should be set and differ: %q, %q", open.ID, done.ID)
	}
	if open.Created.IsZero() || done.Created.IsZero() {
		t.Error("imported tasks should have a created time")
	}
	if !open.Completed.IsZero() {
		t.Error("an open task should not be completed")
	}
	if done.Completed.IsZero() {
		t.Error("a done task should have a completed time")
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
	if err := saveBoard(path, texts{toDo: {"existing"}}.board()); err != nil {
		t.Fatal(err)
	}

	if result := runCLI(t, "import", writeFile(t, "new\n")); result.err != nil {
		t.Fatalf("run: %v", result.err)
	}

	got, err := loadBoard(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	assertBoardsEqual(t, got, texts{toDo: {"existing", "new"}})
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
	assertBoardsEqual(t, got, texts{toDo: {"a"}})
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
