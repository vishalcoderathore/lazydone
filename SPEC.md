# lazydone: product spec and task phases

## Problem Statement

I want a fast, keyboard-driven todo app that lives in my terminal, in the style of lazygit and lazydocker.

The current app is a working, tested three-column kanban board, but it isn't yet safe or capable enough to use every day:

- One press of `d` permanently deletes a task, with no confirmation or undo.
- Tasks are saved to `todos.txt` in whichever folder the app was launched from, so the board changes depending on the folder.
- A crash during a save can damage the task file.
- A task is only a line of text, so it can't have a priority, due date, tag or note.
- Tasks can't be edited, reordered, searched or filtered.
- Tasks can't be captured or counted without opening the TUI.
- Vim navigation is incomplete, and there's no help screen.

## Solution

lazydone becomes a daily-driver task manager that keeps its kanban board and adds:

- Safe storage in one standard location, shared by every folder and every running copy.
- Structured tasks with priorities, due dates, tags, notes and timestamps.
- Full vim-style navigation, undo and a help popup.
- A Today view, search, filtering, sorting and archiving.
- Subcommands for quick capture, scripting and tmux status output.
- Configurable keys and themes, with installable releases.

## Task Phases

Each phase leaves the app usable. Finish a phase before starting the next, because later phases depend on earlier ones.

### Phase 0: Repository setup

**Goal:** the project is on GitHub, and `lazydone` launches from any folder on this machine, like `lazygit` and `lazydocker`.

- [x] Initialize git on `main`
- [x] Rename the Go module to `github.com/vishalcoderathore/lazydone`
- [x] Add a `.gitignore` covering personal task data and the built binary
- [x] Point `origin` at the GitHub repo over SSH
- [x] Choose which email address to publish in commits
- [x] Make the first commit and push it
- [x] Install the `lazydone` command locally with `go install .`, and reinstall after changes
- [x] Confirm that `lazydone` launches the TUI from a folder outside the project
- [x] Add a `--version` flag that prints the version and exits without opening the TUI
- [x] Set the version at build time, so each release binary reports its own tag
- [x] Publish the first alpha release, `v0.1.0-alpha.1`, by hand

**Done when:** typing `lazydone` in any folder opens the TUI, and `lazydone --version` on a release binary prints that release's tag.

Phase 1 has since moved the data to one fixed location, so every folder shares the same board. Alpha releases are still published by hand, and automated releases stay in Phase 6.

### Visual effects

**Goal:** make the app more fun to look at and use. This work was added alongside the phases, and it shipped in `v0.1.0-alpha.2`.

- [x] Give each column its own color for titles and tasks
- [x] Add gradient borders, dimmed on panels without focus
- [x] Add a `LAZYDONE` logo that sweeps in at startup and stays until space or enter is pressed
- [x] Add a confetti burst when a task reaches Completed
- [x] Add a pink-to-teal progress bar and completed count at the bottom right, animated as the count changes
- [x] Decide whether to keep the effects, and commit them if so
- [x] Publish the effects in `v0.1.0-alpha.2`
- [ ] Try the effects in WezTerm on WSL

### Phase 1: Safe storage and the data model

**Goal:** tasks are structured and stored somewhere safe, and nothing a user can see gets worse.

- [x] Add a structured task with ID, text, status, priority, due date, tags, notes, archived flag and created, updated and completed timestamps
- [x] Move storage behind a store module with a versioned JSON file format
- [x] Store data in `$XDG_DATA_HOME/lazydone/`, falling back to `~/.local/share/lazydone/`, and create the folder on first run
- [x] Make saves atomic (write a temp file, then rename)
- [x] Set private permissions on the data folder and file
- [x] Keep the 5 most recent backups of the data file, making at most one an hour
- [x] Reject unreadable or newer-version data files with a clear error, without overwriting them
- [x] Add Cobra with a root command that opens the TUI, keeping `--version` working
- [x] Add a `--file` flag that overrides the data file location
- [x] Add `lazydone import <path>` to bring in an existing `todos.txt`, leaving the original untouched
- [x] Split board logic, storage and the TUI into separate internal packages
- [x] Move the existing tests to the new structure, keeping their coverage

The features shipped in `v0.1.0-alpha.3`. The package split came after that release and is not yet committed. It made no visible change: board rules are in `internal/board`, storage is in `internal/store`, the TUI is in `internal/tui`, and the CLI stays in the root `main` package. Every test still passes in its new package.

**Done when:** an imported board looks and behaves as before from any folder, and a forced failure during a save leaves the previous data intact.

### Phase 2: Finish the MVP

**Goal:** the core TUI is complete, and destructive actions are recoverable.

- [x] Add an undo stack and bind it to `u`
- [x] Replace single-key `d` delete with `dd`
- [x] Add `gg` and `G` to jump to the top and bottom of a column
- [x] Add `Ctrl-d` and `Ctrl-u` for half-page movement
- [x] Replace the hand-written input with Bubbles `textinput`
- [x] Add `e` to edit a task's text
- [x] Add `x` to toggle a task between Completed and To Do
- [x] Add `p` to cycle priority, and show priority on the board and in the details panel
- [x] Add `J` and `K` to reorder tasks within a column
- [x] Define all keybindings in one keymap used by the handlers, footer and help popup
- [x] Add a `?` help popup generated from the keymap
- [x] Show the full task in the details panel, with scrolling for long text
- [x] Show short status messages after actions, such as "Task deleted, press u to undo"
- [x] Let the user retry a failed save, and warn before quitting with unsaved changes

This phase shipped in `v0.1.0-alpha.3`.

**Done when:** every MVP keybinding in the original feature list works, and any delete, edit, move or priority change can be undone.

### Phase 3: Quick capture from the command line

**Goal:** tasks can be captured and queried without opening the TUI.

- [x] Add a shared capture parser for task text, `+project` and `@context` tags and `due:` values
- [x] Add `lazydone add`
- [x] Add `lazydone ls` with status filters and `--json`
- [x] Add `lazydone count`, printing only a number
- [x] Add a file lock, and apply every change to the latest data on disk
- [x] Refresh an open TUI when another process changes the data file
- [x] Generate shell completions for bash, zsh and fish

**Done when:** `lazydone add "fix bug" +work` while the TUI is open shows the new task in the TUI, and neither process overwrites the other's changes.

This phase is finished but not yet committed or released. In this phase `due:` accepts only `DD-MM-YYYY`. Tags and due dates are saved but not yet shown on the board, which Phase 4 adds.

### Phase 4: Daily-driver organization

**Goal:** the board stays useful as the list grows.

- [ ] Add natural due date input: `today`, `tomorrow`, weekday names, `+3d`, `+2w` and `DD-MM-YYYY`
- [ ] Show due dates on tasks, and highlight overdue ones
- [ ] Add `--today` to `ls` and `count`
- [ ] Add a Today view that filters the board to tasks due today or overdue
- [ ] Parse tags in the TUI's add and edit inputs, and show them on tasks
- [ ] Add filtering by tag
- [ ] Add `/` fuzzy search across text, tags and notes, with `n` and `N` to jump between matches across columns
- [ ] Add sorting by manual order, priority, due date or creation date
- [ ] Add archiving, with a view for browsing and restoring archived tasks

**Done when:** I can run my day from the Today view and find any task, including archived ones, in a few keystrokes.

### Phase 5: Terminal-native polish

**Goal:** the app fits into a personal terminal setup.

- [ ] Add `E` to edit notes in `$VISUAL` or `$EDITOR`, falling back to `vi`
- [ ] Load `$XDG_CONFIG_HOME/lazydone/config.yaml`, falling back to `~/.config/lazydone/config.yaml`
- [ ] Support remapped keys, and report invalid or conflicting mappings at startup
- [ ] Support color themes, keeping Catppuccin Mocha as the default
- [ ] Respect `NO_COLOR`
- [ ] Add a no-emoji option
- [ ] Add an option to turn off animations
- [ ] Add visual mode with `v` for bulk delete, done, priority, move and archive, undone as one step
- [ ] Add optional mouse support for clicking and scrolling
- [ ] Add Markdown checklist and `todo.txt` import and export

**Done when:** the keys and theme can be changed from the config file alone, and the app works in a terminal with no color or emoji support.

### Phase 6: Release

**Goal:** other people can find, install and report problems with lazydone.

- [x] Write a README covering installation, usage and keybindings
- [ ] Record a demo GIF with `vhs`
- [ ] Choose and add a license
- [ ] Run tests, `go vet` and `golangci-lint` in GitHub Actions
- [ ] Publish binaries with GoReleaser
- [ ] Confirm that `go install` works from a clean machine

**Done when:** someone new can install lazydone and add their first task using only the README.

### Phase 7: Stretch goals

**Goal:** deeper features, in any order, each specified separately before work begins.

- [ ] Subtasks with `za` expand and collapse
- [ ] Recurring tasks, such as "every weekday"
- [ ] Git-backed sync between machines
- [ ] A stats view with streaks and tasks completed per week

## User Stories

### Storage and safety

1. As a user, I want the same tasks from any folder, so that my board doesn't depend on where I launch the app.
2. As a user, I want my tasks stored in the standard data location, so that they're easy to find and back up.
3. As a user, I want `XDG_DATA_HOME` respected, so that I control where my data lives.
4. As a user, I want to choose a different data file with a flag, so that I can keep separate lists.
5. As a user, I want a crash during a save to leave my data intact, so that I can't lose my whole list.
6. As a user, I want recent backups kept automatically, so that I can recover from a mistake that undo can't fix.
7. As a user, I want my task file readable only by me, so that other users on the machine can't see it.
8. As a user, I want a clear error for a damaged data file, so that the app never replaces my data with an empty board.
9. As a user, I want to import my existing `todos.txt`, so that I keep my current tasks.
10. As a user, I want to retry a failed save, so that a temporary disk problem doesn't cost me my changes.
11. As a user, I want a warning before quitting with unsaved changes, so that I don't lose them by accident.
12. As a user, I want the TUI and the command line used side by side without overwriting each other, so that captured tasks are never lost.

### Core TUI

13. As a user, I want to keep the kanban board, so that I can see each task's status at a glance.
14. As a vim user, I want `j/k` and `h/l` navigation, so that I can use the movements I already know.
15. As a vim user, I want `gg` and `G`, so that I can jump to either end of a long column.
16. As a vim user, I want `Ctrl-d` and `Ctrl-u`, so that I can move quickly through a long column.
17. As a vim user, I want `dd` to delete, so that a single stray key can't remove a task.
18. As a user, I want `u` to undo, so that I can recover from any mistake made in the session.
19. As a user, I want to edit a task's text, so that I don't have to delete and retype it to fix a typo.
20. As a user, I want a full text input with cursor movement, so that I can correct long tasks easily.
21. As a user, I want `x` to mark a task done or not done, so that I can finish a task without moving through columns.
22. As a user, I want `space` and `H/L` to keep moving tasks between columns, so that my existing workflow still works.
23. As a user, I want `p` to cycle priority, so that I can flag important work with one key.
24. As a user, I want to reorder tasks within a column, so that the order reflects what I plan to do next.
25. As a new user, I want a `?` help popup, so that I can discover every keybinding.
26. As a user, I want the details panel to show the whole task, so that long text is never silently cut off.
27. As a user, I want short confirmation messages, so that I can tell what an action just did.

### Command line

28. As a user, I want `lazydone add "fix bug" +work due:fri`, so that I can capture a task without opening the TUI.
29. As a user, I want `lazydone ls`, so that I can see my tasks in a normal shell.
30. As a script author, I want `lazydone ls --json`, so that I can pipe tasks into other tools.
31. As a tmux user, I want `lazydone count --today` to print only a number, so that I can show it in my status line.
32. As a user, I want shell completions, so that I can discover commands and flags with tab.
33. As a user, I want an open TUI to show tasks added from another terminal, so that the board is never out of date.

### Organization

34. As a user, I want due dates like `tomorrow`, `fri` and `+3d`, so that I can set dates quickly.
35. As a user, I want a Today view, so that I can see what's due or overdue.
36. As a user, I want overdue tasks highlighted, so that they stand out on a busy board.
37. As a user, I want `+project` and `@context` tags, so that I can group related tasks.
38. As a user, I want to filter by tag, so that I can focus on one area at a time.
39. As a user, I want `/` fuzzy search, so that I can find a task from a partial memory of it.
40. As a user, I want `n` and `N`, so that I can move between search matches across columns.
41. As a user, I want to sort by priority, due date or creation date, so that I can view my work in the order that suits the moment.
42. As a user, I want sorting to leave my manual order intact, so that I can always return to it.
43. As a user, I want to archive completed tasks, so that the Completed column stays short without losing history.
44. As a user, I want to browse and restore archived tasks, so that archiving is never a one-way step.

### Personalization and bulk actions

45. As a user, I want to edit notes in my `$EDITOR`, so that I can write longer notes with the tools I already use.
46. As a user, I want to remap keys in a config file, so that the app fits my habits.
47. As a user, I want clear errors for a bad config, so that a typo doesn't silently change how the app behaves.
48. As a user, I want color themes, so that the app matches my terminal.
49. As a user, I want `NO_COLOR` respected, so that the app works in limited terminals and for my accessibility needs.
50. As a user, I want a no-emoji option, so that varying emoji widths can't break the panel borders.
51. As a user, I want visual mode, so that I can act on several tasks at once.
52. As a user, I want a bulk action undone in one step, so that recovering from one is as easy as making it.
53. As a user, I want to export my tasks to Markdown and `todo.txt`, so that I'm never locked in.

### Distribution

54. As a new user, I want `go install` and prebuilt binaries, so that I can install lazydone easily.
55. As a new user, I want a README with a demo, so that I can quickly decide whether it suits me.
56. As a user updating often, I want `--version`, so that I can tell whether I'm on the newest build and say exactly which version I'm using when reporting a bug.
57. As a user, I want to launch the app by typing `lazydone` in any folder, so that it starts the same way as `lazygit` and `lazydocker`.

### Visual effects

58. As a user, I want each column to have its own colors, so that I can tell them apart at a glance.
59. As a user, I want a logo that stays until I press space or enter, so that the app has a proper welcome screen.
60. As a user, I want confetti when I complete a task, so that finishing something feels rewarding.
61. As a user, I want an animated progress bar with a completed count, so that I can see how far through my tasks I am.

## Implementation Decisions

### Product

- The kanban board stays. Projects, tags and Today act as filters on the columns rather than replacing them with the original projects, tasks and details layout.
- `x` toggles between Completed and To Do. `space` keeps cycling through all three columns, and `H/L` keep moving a task one column at a time.
- Priority has four levels: none, low, medium and high.
- `x` on a completed task always sends it to To Do, not back to its previous column.
- `q` warns once before quitting with unsaved changes, but `Ctrl-C` always quits immediately.
- Undo lasts for the current session only and holds the 50 most recent actions. Redo isn't included for now.
- Delete stays available alongside archive. Archive is for finished work, and delete is for mistakes.

### Versioning

- Alpha releases are tagged `v0.1.0-alpha.N`, with `N` rising by one each release, and are published as normal GitHub releases so the `/releases/latest/` download link always finds the newest one.
- The release binary's version is set at build time from the release tag. Builds without one report `dev`.

### Visual effects

- Only space and enter dismiss the logo, and `q` and `Ctrl-C` quit from it. Other keys are ignored, so nothing happens to a task by accident.
- Animations never block key presses, and frames are scheduled only while something is moving.
- Effects are switched on when the app starts rather than by default in the model, so tests stay predictable.
- The progress bar sits in the details panel's bottom border, because the key hints already fill the footer on an 80-column terminal.
- The bar's colors are tied to position, so teal only appears as the bar nears full.

### Modules

- **Board:** tasks and every rule for changing them. It has no knowledge of the terminal or the file format.
- **Store:** loading, saving, locking, backups and format versioning, behind a small interface.
- **Capture parser:** turns text like `fix bug +work due:fri` into task fields. The TUI and the command line share it, so they can't disagree.
- **Due date parser:** turns date input into a date, given the current time.
- **TUI:** rendering and key handling, with no direct file access.
- **CLI:** Cobra commands that call the board and store.
- **Config:** loads and validates keymaps and themes, and supplies defaults.

The Board is `internal/board`, the Store is `internal/store`, the capture parser is `internal/capture`, and the TUI is `internal/tui`. The TUI loads, updates and watches the board through a small `Store` interface, so it never opens files itself. The CLI is the root `main` package for now. The due date parser and Config don't exist yet.

### Data

- The data file is versioned JSON containing a version number and a list of tasks. List order is the manual order within each column.
- IDs are generated once and never reused, so tasks stay identifiable through edits, moves and concurrent changes.
- Archived tasks stay in the same file with an archived flag.
- A task's status is the column that holds it in memory, and is written as a `status` field only in the file, so the two can't disagree.
- A backup is made at most once an hour, so that the five backups cover hours of use rather than the last five key presses.
- `lazydone import` adds to the existing tasks and doesn't check for duplicates, so importing a file twice adds its tasks twice.
- SQLite isn't needed at this scale. The store interface keeps that option open.

### Command line

- Due dates are written `DD-MM-YYYY`, in `add`, in `ls` output and in the TUI later on. A single-digit day or month, such as `5-1-2027`, is also accepted. Any other form is an error, and nothing is saved.
- `ls` and `count` take `--todo`, `--doing` and `--done`, which can be combined. With none of them, every column is included.
- `ls` prints one task per line in the form `add` accepts, such as `[todo] fix bug +work due:05-11-2026 priority:high`. `--json` prints a list of tasks, each with its `status`.
- `ls` and `count` leave out archived tasks.
- Completions come from Cobra's `completion` command, which also offers PowerShell.

### Concurrency and undo

- Every change takes the file lock, reloads the file, applies the change by task ID, writes atomically, and releases the lock.
- Undo records the inverse of each action by task ID rather than a snapshot of the board. Restoring a snapshot would erase tasks added by another process.
- A bulk action is recorded as one undo entry.
- The lock is an exclusive `flock` on `tasks.json.lock` beside the real file behind any symlinks. The lock file is never removed, because removing it would let two processes lock different files of the same name. A crash releases the lock.
- The TUI saves each action as a function of task ID. If a save fails, the actions stay queued and the next save, or `r`, applies them to the latest data.
- The TUI checks the data file's modification time once a second and reloads it when it changes. Polling needs no extra dependency, and costs one `stat` a second.
- The edit input remembers the task's ID, so a refresh that moves the task while it's being edited doesn't change a different one.
- A task with no ID, as in a hand-edited file, gets an ID made from its status, position and text. Every process that reads the file then agrees on it, and the next save writes it down.

### Input rules

- `+word` and `@word` become tags only when the character after the symbol is a letter, so text like `+1` or an email address stays as text. A repeated tag is kept once.
- A task needs some text besides its tags and due date, and only one `due:` is allowed.
- Recognized tags and `due:` values move into their own fields and are removed from the task text.
- A weekday name means its next occurrence after today, so `fri` entered on a Friday means next week. Use `today` for the current day.
- `dd` and `gg` use a pending-key state that any other key clears. That other key still does its usual job, so a stray `d` never eats the next key press.
- Clearing a task's text in the edit input keeps the task, because `dd` is the way to delete.
- While a non-manual sort is active, `J` and `K` are disabled and the status message explains why.

## Testing Decisions

### What makes a good test

- A good test drives the app the way a user does and checks what the user sees or what ends up on disk. It doesn't check internal state.
- Tests never touch real user data. They use temp folders through `--file` or `XDG_DATA_HOME`.
- Anything that depends on today's date uses an injected clock.

### Proposed test seams

These are the highest available seams, and they keep the number of seams small.

1. **TUI seam (existing):** send key messages to the Bubble Tea model, then check the rendered view and the saved data. Nearly all board behavior can be tested here.
2. **CLI seam (new):** run the root command with arguments and captured output against a temp data file. This covers subcommands, flags, JSON output, error messages and import.
3. **Store seam (existing):** load and save against real temp files. This covers things hard to reach from above, such as atomic saves, backups, locking, permissions and damaged files.

The capture and due date parsers are tested with table tests, since they have many input cases and no side effects.

### Prior art

- `internal/tui/tui_test.go` drives the model with key messages, and is the pattern for new TUI behavior.
- `internal/tui/view_test.go` checks rendered output and exact terminal sizing, and is the pattern for layout behavior.
- `internal/store/store_test.go` uses real temp files, and is the pattern for storage behavior.
- `main_test.go` runs the root command with captured output, and is the pattern for the CLI seam.
- `internal/boardtest` holds the shared helpers for building and comparing boards in tests.

### Additions

- Fuzz the data file loader and both parsers, so bad input returns an error rather than a crash.
- Test concurrent writers by running several processes against one file and checking that no task is lost.
- Add `teatest` snapshot tests only for full-screen layouts that the view tests can't check well.

## Out of Scope

- GitHub issues or other issue tracker integration for planning this work. This file is the task list.
- Cloud sync or a hosted service. Git-backed sync is the only sync considered, in Phase 7.
- Multi-user collaboration, or sharing tasks between accounts.
- A web, mobile or desktop interface.
- User-defined board columns. The three statuses stay fixed.
- Redo.
- Reminders or desktop notifications.
- Encryption of the data file.
- Native Windows support as a release target, though WSL is expected to work like Linux.

## Further Notes

- Phase 1 is the most important: nearly every later feature needs its data model, and it fixes the highest data-loss risks. The exception is the single-key delete, which Phase 2 replaces.
- Phase 3 must add locking before `add` is released, because a TUI left open would otherwise overwrite tasks captured from the command line.
- The project has no glossary or ADRs yet. The terms used here (board, column, task, status, tag, archive) are the working vocabulary.
