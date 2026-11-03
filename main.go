package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vishalcoderathore/lazydone/internal/board"
	"github.com/vishalcoderathore/lazydone/internal/capture"
	"github.com/vishalcoderathore/lazydone/internal/store"
	"github.com/vishalcoderathore/lazydone/internal/tui"
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
type opener func(path string, tasks board.Board) error

func openTUI(path string, tasks board.Board) error {
	return tui.Run(store.File{Path: path}, tasks, version)
}

func run(args []string, stdout, stderr io.Writer, open opener) error {
	var file string
	data := func() (store.File, error) {
		path, err := store.Path(file)
		return store.File{Path: path}, err
	}

	root := &cobra.Command{
		Use:           "lazydone",
		Short:         "A keyboard-driven kanban todo app for the terminal",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceErrors: true, // main prints errors
		SilenceUsage:  true, // a failed save is not a usage mistake
		RunE: func(*cobra.Command, []string) error {
			data, err := data()
			if err != nil {
				return err
			}
			tasks, err := data.Load()
			if err != nil {
				return err
			}
			return open(data.Path, tasks)
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
			data, err := data()
			if err != nil {
				return err
			}
			return importLegacy(args[0], data, stdout)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "add <task>",
		Short: "Add a task to To Do, such as: add fix bug +work @home due:05-11-2026",
		Long: `Add a task to To Do.

Words starting with + or @ followed by a letter become tags, and due:DD-MM-YYYY
sets the due date. Both are taken out of the task's text.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := capture.Parse(strings.Join(args, " "))
			if err != nil {
				return err
			}
			data, err := data()
			if err != nil {
				return err
			}
			item := board.NewTask(fields.Text, time.Now())
			item.Tags, item.Due = fields.Tags, fields.Due
			if _, err := data.Update(func(b board.Board) board.Board { return b.Add(item) }); err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, "Added:", describe(board.ToDo, item))
			return err
		},
	})

	var asJSON bool
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List tasks, or only those with the given statuses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := data()
			if err != nil {
				return err
			}
			listed, err := list(data, statuses(cmd))
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(stdout, listed)
			}
			for _, item := range listed {
				if _, err := fmt.Fprintln(stdout, describe(item.column, item.Task)); err != nil {
					return err
				}
			}
			return nil
		},
	}
	ls.Flags().BoolVar(&asJSON, "json", false, "print the tasks as JSON")
	addStatusFlags(ls)
	root.AddCommand(ls)

	count := &cobra.Command{
		Use:   "count",
		Short: "Print only the number of tasks, or of those with the given statuses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := data()
			if err != nil {
				return err
			}
			listed, err := list(data, statuses(cmd))
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, len(listed))
			return err
		},
	}
	addStatusFlags(count)
	root.AddCommand(count)

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	return root.Execute()
}

// addStatusFlags gives a command one flag per status: --todo, --doing and
// --done.
func addStatusFlags(cmd *cobra.Command) {
	for _, status := range board.Statuses {
		cmd.Flags().Bool(status, false, "include "+status+" tasks")
	}
}

// statuses reports which columns a command's status flags ask for. With no
// status flag, every column is included.
func statuses(cmd *cobra.Command) [board.NumColumns]bool {
	var wanted [board.NumColumns]bool
	some := false
	for column, status := range board.Statuses {
		wanted[column], _ = cmd.Flags().GetBool(status)
		some = some || wanted[column]
	}
	if !some {
		return [board.NumColumns]bool{true, true, true}
	}
	return wanted
}

// listedTask is a task as ls and count see it, with its status.
type listedTask struct {
	board.Task
	column int
	Status string `json:"status"`
}

// list returns the tasks in the wanted columns, in board order. Archived tasks
// are left out.
func list(data store.File, wanted [board.NumColumns]bool) ([]listedTask, error) {
	tasks, err := data.Load()
	if err != nil {
		return nil, err
	}
	listed := []listedTask{}
	for column, items := range tasks {
		if !wanted[column] {
			continue
		}
		for _, item := range items {
			if !item.Archived {
				listed = append(listed, listedTask{item, column, board.Statuses[column]})
			}
		}
	}
	return listed, nil
}

// describe writes a task on one line, in the same form that add accepts, such
// as: [todo] fix bug +work due:05-11-2026 priority:high
func describe(column int, item board.Task) string {
	parts := append([]string{"[" + board.Statuses[column] + "]", item.Text}, item.Tags...)
	if !item.Due.IsZero() {
		parts = append(parts, "due:"+item.Due.Format(capture.DateLayout))
	}
	if item.Priority != "" {
		parts = append(parts, "priority:"+item.Priority)
	}
	return strings.Join(parts, " ")
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// importLegacy adds the tasks of an old todos.txt file to a data file.
func importLegacy(source string, data store.File, stdout io.Writer) error {
	legacy, err := store.ReadLegacy(source)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = data.Update(func(tasks board.Board) board.Board {
		for column, list := range legacy {
			for _, item := range list {
				imported := board.NewTask(item.Text, now)
				if column == board.Completed {
					imported.Completed = now
				}
				tasks[column] = append(tasks[column], imported)
			}
		}
		return tasks
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Imported %d tasks from %s to %s\n", legacy.Total(), source, data.Path)
	return err
}
