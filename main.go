package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const todoFile = "todos.txt"

// version is replaced at build time for releases:
//
//	go build -ldflags "-X main.version=v0.1.0-alpha.1"
var version = "dev"

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return // the flag package has already printed the usage
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("lazydone", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		_, err := fmt.Fprintln(stdout, "lazydone", version)
		return err
	}

	tasks, err := loadBoard(todoFile)
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(newModel(todoFile, tasks).withAnimations(), tea.WithAltScreen()).Run()
	return err
}
