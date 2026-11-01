package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// The columns of the board, from left to right.
const (
	toDo = iota
	inProgress
	completed
	numColumns
)

// task is one item on the board. Its status is the column that holds it, and a
// task's place in that column is its manual order.
type task struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Priority  string    `json:"priority,omitempty"`
	Due       time.Time `json:"due,omitzero"`
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Archived  bool      `json:"archived,omitempty"`
	Created   time.Time `json:"created,omitzero"`
	Updated   time.Time `json:"updated,omitzero"`
	Completed time.Time `json:"completed,omitzero"`
}

// newTask returns a task created at the given time, with a new ID.
func newTask(text string, now time.Time) task {
	return task{ID: rand.Text(), Text: text, Created: now, Updated: now}
}

// board holds the tasks of each column.
type board [numColumns][]task

// fileVersion is the version of the data file format that this build writes.
const fileVersion = 1

const (
	backupCount    = 5         // backups kept of the data file
	backupInterval = time.Hour // minimum time between backups
)

// statusNames are how the columns are written in the data file.
var statusNames = [numColumns]string{"todo", "doing", "done"}

type dataFile struct {
	Version int          `json:"version"`
	Tasks   []storedTask `json:"tasks"`
}

type storedTask struct {
	task
	Status string `json:"status"`
}

// dataFilePath returns the data file to use: the override if one was given,
// otherwise tasks.json in the standard data folder. The XDG spec says that a
// relative XDG_DATA_HOME must be ignored.
func dataFilePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	dir := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "lazydone", "tasks.json"), nil
}

// loadBoard reads tasks from a data file. A file that does not exist yet simply
// means an empty board.
func loadBoard(path string) (board, error) {
	var tasks board

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tasks, nil
	}
	if err != nil {
		return tasks, err
	}

	var file dataFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version < 1 {
		return tasks, fmt.Errorf("%s is not a valid lazydone data file", path)
	}
	if file.Version > fileVersion {
		return tasks, fmt.Errorf("%s was written by a newer version of lazydone, so update lazydone to open it", path)
	}
	for _, stored := range file.Tasks {
		column := slices.Index(statusNames[:], stored.Status)
		if column < 0 {
			return board{}, fmt.Errorf("%s has a task with unknown status %q", path, stored.Status)
		}
		tasks[column] = append(tasks[column], stored.task)
	}
	return tasks, nil
}

// saveBoard writes all tasks to a data file. A symlink is followed, because
// renaming over it would replace the link and leave its target out of date.
func saveBoard(path string, tasks board) error {
	path, err := resolveSymlinks(path)
	if err != nil {
		return err
	}
	file := dataFile{Version: fileVersion, Tasks: []storedTask{}}
	for column, list := range tasks {
		for _, item := range list {
			file.Tasks = append(file.Tasks, storedTask{item, statusNames[column]})
		}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := backUp(path); err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// resolveSymlinks returns the file that a path really refers to, so that saving
// updates that file instead of replacing the link. The file may not exist yet, as
// with a link into a dotfiles folder on a new machine, and a loop is an error.
func resolveSymlinks(path string) (string, error) {
	for {
		target, err := filepath.EvalSymlinks(path)
		if err == nil {
			return target, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		link, err := os.Readlink(path)
		if err != nil {
			return path, nil // nothing exists at this path, so it's the file to create
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		path = link
	}
}

// backUp copies the current data file to path.bak.1, moving older backups along
// and dropping the oldest. It does nothing if the newest backup is recent, so
// that backups cover hours of use rather than the last few key presses.
func backUp(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	backup := func(number int) string { return fmt.Sprintf("%s.bak.%d", path, number) }
	if info, err := os.Stat(backup(1)); err == nil && time.Since(info.ModTime()) < backupInterval {
		return nil
	}
	// Write the new backup before moving the others, so that a failed write
	// doesn't push old backups out.
	if err := writeFileAtomic(backup(0), data); err != nil {
		return err
	}
	for number := backupCount - 1; number >= 0; number-- {
		err := os.Rename(backup(number), backup(number+1))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// writeFileAtomic replaces a file in one step, by writing a private temporary
// file beside it and renaming that over the top. A crash or a failure at any
// point leaves the previous file intact.
func writeFileAtomic(path string, data []byte) (err error) {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(temp.Name())
		}
	}()

	_, err = temp.Write(data)
	if err == nil {
		err = temp.Sync() // the data must reach the disk before the rename
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// legacyTags mark which column a task belongs to in the old text format.
var legacyTags = [numColumns]string{"[todo]", "[doing]", "[done]"}

// readLegacyBoard reads the old text format, which holds one task per line:
//
//	[todo] buy milk
//	[doing] call the bank
//	[done] walk the dog
//
// A line without a tag goes to To Do, so plain lists keep working.
func readLegacyBoard(path string) (board, error) {
	var tasks board

	data, err := os.ReadFile(path)
	if err != nil {
		return tasks, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		column, text := parseLegacyLine(line)
		if text = cleanText(text); text != "" {
			tasks[column] = append(tasks[column], task{Text: text})
		}
	}
	return tasks, nil
}

func parseLegacyLine(line string) (column int, text string) {
	for column, tag := range legacyTags {
		if rest, ok := strings.CutPrefix(line, tag+" "); ok {
			return column, rest
		}
	}
	return toDo, line
}

// cleanText trims a task and collapses its whitespace, so that a task always
// fits on one line on screen.
func cleanText(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
}
