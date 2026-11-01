package main

import (
	"fmt"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// version is replaced at build time for releases:
//
//	go build -ldflags "-X main.version=v0.1.0-alpha.1"
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, openTUI); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// opener opens the interactive board on a data file.
type opener func(path string, tasks board) error

func openTUI(path string, tasks board) error {
	_, err := tea.NewProgram(newModel(path, tasks).withAnimations(), tea.WithAltScreen()).Run()
	return err
}

func run(args []string, stdout, stderr io.Writer, open opener) error {
	var file string

	root := &cobra.Command{
		Use:               "lazydone",
		Short:             "A keyboard-driven kanban todo app for the terminal",
		Version:           version,
		Args:              cobra.NoArgs,
		SilenceErrors:     true, // main prints errors
		SilenceUsage:      true, // a failed save is not a usage mistake
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(*cobra.Command, []string) error {
			path, err := dataFilePath(file)
			if err != nil {
				return err
			}
			tasks, err := loadBoard(path)
			if err != nil {
				return err
			}
			return open(path, tasks)
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().StringVar(&file, "file", "", "data file to use instead of the default location")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErrln(cmd.UsageString())
		return err
	})

	root.AddCommand(&cobra.Command{
		Use:   "import <path>",
		Short: "Add tasks from a todos.txt file, leaving that file untouched",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := dataFilePath(file)
			if err != nil {
				return err
			}
			return importLegacy(args[0], path, stdout)
		},
	})

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	return root.Execute()
}

// importLegacy adds the tasks of an old todos.txt file to a data file.
func importLegacy(source, path string, stdout io.Writer) error {
	legacy, err := readLegacyBoard(source)
	if err != nil {
		return err
	}
	tasks, err := loadBoard(path)
	if err != nil {
		return err
	}

	now, count := time.Now(), 0
	for column, list := range legacy {
		for _, item := range list {
			imported := newTask(item.Text, now)
			if column == completed {
				imported.Completed = now
			}
			tasks[column] = append(tasks[column], imported)
			count++
		}
	}
	if err := saveBoard(path, tasks); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Imported %d tasks from %s to %s\n", count, source, path)
	return err
}
