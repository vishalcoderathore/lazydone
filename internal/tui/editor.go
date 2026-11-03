package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// openEditor gets a task's notes ready to edit in the user's editor. It writes
// them to a temporary file, readable only by the user, and returns the command
// that opens the editor on it, and finish, which reads the file back once the
// editor has closed, removes it unless the notes in it could be lost, and
// reports the result as a notesEdited.
func openEditor(id, notes string, getenv func(string) string) (*exec.Cmd, func(error) tea.Msg, error) {
	file, err := os.CreateTemp("", "lazydone-notes-*.md")
	if err != nil {
		return nil, nil, err
	}
	path := file.Name()
	_, err = file.WriteString(notes)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return nil, nil, err
	}

	editor := "vi"
	for _, name := range []string{"EDITOR", "VISUAL"} { // VISUAL wins over EDITOR
		if value := strings.TrimSpace(getenv(name)); value != "" {
			editor = value
		}
	}
	// Run through the shell, as git does, so an editor such as "code --wait"
	// can carry its own arguments.
	cmd := exec.Command("sh", "-c", editor+` "$@"`, editor, path)

	finish := func(runErr error) tea.Msg {
		// An editor that exits with an error, as vim's :cq does, is giving up,
		// so its file goes. Any other error, such as the terminal not coming
		// back, may come after the notes were saved, so the file is kept.
		var exitErr *exec.ExitError
		if runErr != nil && !errors.As(runErr, &exitErr) {
			return notesEdited{id: id, err: fmt.Errorf("%w (your notes are kept in %s)", runErr, path)}
		}
		defer os.Remove(path)
		if runErr != nil {
			return notesEdited{id: id, err: runErr}
		}
		edited, err := os.ReadFile(path)
		return notesEdited{id: id, notes: string(edited), err: err}
	}
	return cmd, finish, nil
}

// editNotes opens a task's notes in the user's editor, which takes over the
// terminal until it closes. The board then gets a notesEdited.
func editNotes(id, notes string) tea.Cmd {
	cmd, finish, err := openEditor(id, notes, os.Getenv)
	if err != nil {
		return func() tea.Msg { return notesEdited{id: id, err: err} }
	}
	return tea.ExecProcess(cmd, finish)
}
