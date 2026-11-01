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

## Things to know

- Tasks are saved to `$XDG_DATA_HOME/lazydone/tasks.json`, or `~/.local/share/lazydone/tasks.json` if that isn't set, so you see the same board from every folder.
- Use `lazydone --file path/to/tasks.json` to keep a separate list.
- The 5 most recent backups sit beside the data file as `tasks.json.bak.1` to `.bak.5`, with at most one made per hour.
- Don't run two copies on one file at the same time, because the last one to save overwrites the other's changes.
