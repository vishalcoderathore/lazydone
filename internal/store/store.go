// Package store loads and saves the board, in a versioned JSON file that is
// written safely and backed up.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/vishalcoderathore/lazydone/internal/board"
)

// fileVersion is the version of the data file format that this build writes.
const fileVersion = 1

const (
	backupCount    = 5         // backups kept of the data file
	backupInterval = time.Hour // minimum time between backups
)

type dataFile struct {
	Version int          `json:"version"`
	Tasks   []storedTask `json:"tasks"`
}

type storedTask struct {
	board.Task
	Status string `json:"status"`
}

// Path returns the data file to use: the override if one was given,
// otherwise tasks.json in the standard data folder. The XDG spec says that a
// relative XDG_DATA_HOME must be ignored.
func Path(override string) (string, error) {
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

// File is a data file that holds a board. Several processes can share one
// file safely, as long as every change goes through Update.
type File struct {
	Path string
}

// Update changes the board in the file as one step that no other lazydone
// process can interleave with. It takes the file lock, reads the latest board,
// applies the change, writes the result and releases the lock, so that a change
// never overwrites one made elsewhere. It returns the board as written.
func (f File) Update(change func(board.Board) board.Board) (board.Board, error) {
	// Two links to one file must share a lock, so lock the real file.
	path, err := resolveSymlinks(f.Path)
	if err != nil {
		return board.Board{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return board.Board{}, err
	}
	held, err := lock(path)
	if err != nil {
		return board.Board{}, err
	}
	defer func() { _ = held.Close() }() // closing releases the lock; it has nothing to report

	tasks, err := File{path}.Load()
	if err != nil {
		return board.Board{}, err
	}
	tasks = change(tasks)
	return tasks, save(path, tasks)
}

// Save replaces the whole board in the file.
func (f File) Save(tasks board.Board) error {
	_, err := f.Update(func(board.Board) board.Board { return tasks })
	return err
}

// ModTime returns when the file last changed, or the zero time if it doesn't
// exist yet. Watching it shows when another process has changed the board.
func (f File) ModTime() (time.Time, error) {
	info, err := os.Stat(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// lock waits for, then takes, an exclusive lock on path.lock. The lock is
// released when the returned file is closed or the process ends, so a crash
// can't leave it held. The lock file is left in place, because removing it
// would let two processes lock two different files of the same name.
func lock(path string) (*os.File, error) {
	file, err := os.OpenFile(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close() // the lock error is the one to report
		return nil, err
	}
	return file, nil
}

// Load reads the board from the file. A file that does not exist yet simply
// means an empty board.
func (f File) Load() (board.Board, error) {
	var tasks board.Board
	path := f.Path

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
		column := slices.Index(board.Statuses[:], stored.Status)
		if column < 0 {
			return board.Board{}, fmt.Errorf("%s has a task with unknown status %q", path, stored.Status)
		}
		if stored.ID == "" {
			stored.ID = stableID(stored.Status, len(tasks[column]), stored.Text)
		}
		tasks[column] = append(tasks[column], stored.Task)
	}
	return tasks, nil
}

// stableID gives a task from a hand-edited file, which has no ID, one that is
// the same on every load. Changes find tasks by ID, so every process that reads
// the file must agree on it until the next save writes it down.
func stableID(status string, index int, text string) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s\x00%d\x00%s", status, index, text))
	return "file-" + hex.EncodeToString(sum[:8])
}

// save writes all tasks to the real file behind any symlinks, safely, backing
// up what it replaces. The caller holds the lock.
func save(path string, tasks board.Board) error {
	file := dataFile{Version: fileVersion, Tasks: []storedTask{}}
	for column, list := range tasks {
		for _, item := range list {
			file.Tasks = append(file.Tasks, storedTask{item, board.Statuses[column]})
		}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
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
var legacyTags = [board.NumColumns]string{"[todo]", "[doing]", "[done]"}

// ReadLegacy reads the old text format, which holds one task per line:
//
//	[todo] buy milk
//	[doing] call the bank
//	[done] walk the dog
//
// A line without a tag goes to To Do, so plain lists keep working.
func ReadLegacy(path string) (board.Board, error) {
	var tasks board.Board

	data, err := os.ReadFile(path)
	if err != nil {
		return tasks, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		column, text := parseLegacyLine(line)
		if text = board.CleanText(text); text != "" {
			tasks[column] = append(tasks[column], board.Task{Text: text})
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
	return board.ToDo, line
}
