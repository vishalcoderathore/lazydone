# lazydone

A keyboard-driven kanban todo app for the terminal, inspired by lazygit and lazydocker.

This is alpha software, so expect things to change between releases.

## Install

Prebuilt binaries are available for Linux on x86_64, including WSL:

```sh
mkdir -p ~/.local/bin
curl -fL https://github.com/vishalcoderathore/lazydone/releases/latest/download/lazydone-linux-amd64 -o ~/.local/bin/lazydone
chmod +x ~/.local/bin/lazydone
```

Check that it installed:

```sh
lazydone --version
```

If the command isn't found, make sure `~/.local/bin` is on your `PATH`, then open a new terminal.

### With Go

If you have Go 1.27.3 or newer:

```sh
go install github.com/vishalcoderathore/lazydone@latest
```

Installs made this way report their version as `dev`.

## Update

Close `lazydone`, then run the same download again:

```sh
curl -fL https://github.com/vishalcoderathore/lazydone/releases/latest/download/lazydone-linux-amd64 -o ~/.local/bin/lazydone
chmod +x ~/.local/bin/lazydone
```

If you installed with Go, run the `go install` command again.

### Coming from `todos.txt`

Older versions saved tasks to a `todos.txt` in each folder. Your board opens empty until you bring those tasks in, once for each file:

```sh
lazydone import path/to/todos.txt
```

The original file is left untouched. Importing a file twice adds its tasks twice.

## Usage

```sh
lazydone
```

Press `?` in the app to see every key.

| Key | Action |
|---|---|
| `h` / `l`, `←` / `→`, `Shift-Tab` / `Tab` | Switch column |
| `1` `2` `3` | Jump to a column |
| `j` / `k`, `↓` / `↑` | Select a task |
| `gg` / `G` | Jump to the first or last task |
| `Ctrl-d` / `Ctrl-u` | Move half a page down or up |
| `a` or `4` | Add a task |
| `e` | Edit a task |
| `dd` | Delete a task |
| `u` | Undo, up to the last 50 changes in this session |
| `x` | Mark a task done, or move it from Completed back to To Do |
| `p` | Cycle a task's priority |
| `Space` | Move a task to the next column |
| `H` / `L`, `Shift-←` / `Shift-→` | Move a task left or right |
| `J` / `K` | Reorder a task down or up |
| `Ctrl-e` / `Ctrl-y` | Scroll the details panel down or up |
| `r` | Retry a failed save |
| `?` | Show help |
| `q` or `Ctrl-C` | Quit |

While typing a task, press `Enter` to save or `Esc` to cancel.

If a save has failed, `q` warns you once before quitting. `Ctrl-C` always quits immediately.

## Command line

Add and check tasks without opening the app. These work while the app is open in another terminal, and the app shows the change within a second.

```sh
lazydone add fix bug +work @home due:05-11-2026
lazydone ls
lazydone count --todo
```

- `add` puts a task in To Do. A word starting with `+` or `@` followed by a letter becomes a tag, and `due:DD-MM-YYYY` sets a due date. Both are taken out of the task's text.
- `ls` lists tasks, one per line, in the same form `add` accepts. Add `--json` for output to use in scripts.
- `count` prints only the number of tasks, for a tmux status line or a shell prompt.
- `--todo`, `--doing` and `--done` limit `ls` and `count` to those columns. With none of them, every column is included.

Archived tasks are left out of `ls` and `count`.

### Shell completion

Tab completion for commands and flags:

```sh
lazydone completion bash > ~/.local/share/bash-completion/completions/lazydone
lazydone completion zsh > "${fpath[1]}/_lazydone"
lazydone completion fish > ~/.config/fish/completions/lazydone.fish
```

Then open a new terminal. Run `lazydone completion <shell> --help` for other ways to load it.

## Things to know

- Tasks are saved to `$XDG_DATA_HOME/lazydone/tasks.json`, or `~/.local/share/lazydone/tasks.json` if that isn't set, so you see the same board from every folder.
- Use `lazydone --file path/to/tasks.json` to keep a separate list.
- The 5 most recent backups sit beside the data file as `tasks.json.bak.1` to `.bak.5`, with at most one made per hour.
- Several copies of the app and the command line can use one file at the same time. Each change is applied to the latest saved tasks, so none overwrites another. A `tasks.json.lock` file beside the data file makes this work, and is safe to leave in place.
