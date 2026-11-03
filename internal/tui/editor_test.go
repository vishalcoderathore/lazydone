package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEditor writes a shell script that acts as an editor: it keeps a copy of
// the file it was given in seen, then runs body on it as "$1".
func fakeEditor(t *testing.T, body string) (editor, seen string) {
	t.Helper()
	dir := t.TempDir()
	editor, seen = filepath.Join(dir, "editor"), filepath.Join(dir, "seen")
	script := "#!/bin/sh\ncp \"$1\" '" + seen + "'\n" + body + "\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return editor, seen
}

// env is a stand-in for os.Getenv.
type env map[string]string

func (e env) get(name string) string { return e[name] }

// runEditor opens the editor as E does, runs it to the end as Bubble Tea
// would, and returns what it sent back.
func runEditor(t *testing.T, notes string, getenv func(string) string) notesEdited {
	t.Helper()
	cmd, finish, err := openEditor("a", notes, getenv)
	if err != nil {
		t.Fatal(err)
	}
	return finish(cmd.Run()).(notesEdited)
}

func TestEditorGetsTheNotesAndReturnsWhatWasWritten(t *testing.T) {
	editor, seen := fakeEditor(t, `printf 'new notes\n' > "$1"`)

	got := runEditor(t, "old notes", env{"VISUAL": editor}.get)

	if got.id != "a" || got.notes != "new notes\n" || got.err != nil {
		t.Errorf("got %+v, want the new notes for task a", got)
	}
	if before, _ := os.ReadFile(seen); string(before) != "old notes" {
		t.Errorf("the editor opened %q, want the old notes", before)
	}
}

func TestTheNotesFileIsRemovedAfterwards(t *testing.T) {
	for name, body := range map[string]string{"saved": `echo hi > "$1"`, "failed": "exit 1"} {
		t.Run(name, func(t *testing.T) {
			editor, _ := fakeEditor(t, body)
			cmd, finish, err := openEditor("a", "notes", env{"VISUAL": editor}.get)
			if err != nil {
				t.Fatal(err)
			}
			path := cmd.Args[len(cmd.Args)-1]
			finish(cmd.Run())

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("%s is still there (err %v), want it removed", path, err)
			}
		})
	}
}

func TestTheNotesFileIsKeptWhenTheTerminalDoesNotComeBack(t *testing.T) {
	editor, _ := fakeEditor(t, `echo hi > "$1"`)
	cmd, finish, err := openEditor("a", "notes", env{"VISUAL": editor}.get)
	if err != nil {
		t.Fatal(err)
	}
	path := cmd.Args[len(cmd.Args)-1]
	defer os.Remove(path)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// The editor saved, then Bubble Tea could not restore the terminal.
	got := finish(errors.New("restore failed")).(notesEdited)

	if got.err == nil || !strings.Contains(got.err.Error(), path) {
		t.Errorf("got %+v, want an error naming %s", got, path)
	}
	if kept, err := os.ReadFile(path); err != nil || string(kept) != "hi\n" {
		t.Errorf("notes file holds %q (err %v), want the saved notes kept", kept, err)
	}
}

func TestEditorIsVisualThenEditorThenVi(t *testing.T) {
	visual, _ := fakeEditor(t, `printf visual > "$1"`)
	editor, _ := fakeEditor(t, `printf editor > "$1"`)
	vi, _ := fakeEditor(t, `printf vi > "$1"`)
	// A vi of our own comes first on the PATH, so no real editor opens.
	t.Setenv("PATH", filepath.Dir(vi)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.Rename(vi, filepath.Join(filepath.Dir(vi), "vi")); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		env  env
		want string
	}{
		"both set":     {env{"VISUAL": visual, "EDITOR": editor}, "visual"},
		"only EDITOR":  {env{"EDITOR": editor}, "editor"},
		"blank VISUAL": {env{"VISUAL": "  ", "EDITOR": editor}, "editor"},
		"neither":      {env{}, "vi"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := runEditor(t, "", c.env.get); got.notes != c.want || got.err != nil {
				t.Errorf("got %+v, want notes written by %s", got, c.want)
			}
		})
	}
}

func TestEditorMayHaveArguments(t *testing.T) {
	// Like "code --wait": the file comes after the editor's own arguments.
	editor, _ := fakeEditor(t, `for file; do :; done; printf '%s' "$1" > "$file"`)

	if got := runEditor(t, "", env{"EDITOR": editor + " --wait"}.get); got.notes != "--wait" {
		t.Errorf("got %+v, want the editor to get --wait and then the file", got)
	}
}
