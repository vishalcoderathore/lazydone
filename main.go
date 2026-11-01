package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const todoFile = "todos.txt"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	tasks, err := loadBoard(todoFile)
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(newModel(todoFile, tasks), tea.WithAltScreen()).Run()
	return err
}
