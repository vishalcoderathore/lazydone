package main

import (
	"errors"
	"os"
	"strings"
	"unicode"
)

// The columns of the board, from left to right.
const (
	toDo = iota
	inProgress
	completed
	numColumns
)

// board holds the tasks of each column.
type board [numColumns][]string

// columnTags mark which column a task belongs to in the file.
var columnTags = [numColumns]string{"[todo]", "[doing]", "[done]"}

// loadBoard reads tasks from a text file that holds one task per line:
//
//	[todo] buy milk
//	[doing] call the bank
//	[done] walk the dog
//
// A line without a tag goes to To Do, so plain lists keep working.
// A file that does not exist yet simply means an empty board.
func loadBoard(path string) (board, error) {
	var tasks board

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tasks, nil
	}
	if err != nil {
		return tasks, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		column, text := parseLine(line)
		if text = cleanText(text); text != "" {
			tasks[column] = append(tasks[column], text)
		}
	}
	return tasks, nil
}

func parseLine(line string) (column int, text string) {
	for column, tag := range columnTags {
		if rest, ok := strings.CutPrefix(line, tag+" "); ok {
			return column, rest
		}
	}
	return toDo, line
}

// saveBoard writes all tasks to a text file, one task per line.
func saveBoard(path string, tasks board) error {
	var content strings.Builder
	for column, list := range tasks {
		for _, task := range list {
			content.WriteString(columnTags[column] + " " + task + "\n")
		}
	}
	return os.WriteFile(path, []byte(content.String()), 0o644)
}

// cleanText trims a task and collapses its whitespace, so that a task always
// fits on one line in the file and on screen.
func cleanText(text string) string {
	return strings.Join(strings.Fields(controlsToSpaces(text)), " ")
}

func controlsToSpaces(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}
