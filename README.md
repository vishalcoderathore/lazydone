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

## Usage

```sh
lazydone
```

| Key | Action |
|---|---|
| `h` / `l`, `←` / `→`, `Shift-Tab` / `Tab` | Switch column |
| `1` `2` `3` | Jump to a column |
| `j` / `k`, `↓` / `↑` | Select a task |
| `a` or `4` | Add a task |
| `d` | Delete a task |
| `Space` | Move a task to the next column |
| `H` / `L`, `Shift-←` / `Shift-→` | Move a task left or right |
| `q` or `Ctrl-C` | Quit |

While adding a task, press `Enter` to save or `Esc` to cancel.

## Things to know

- Tasks are saved to `todos.txt` in the folder you launch `lazydone` from, so each folder has its own board.
- `d` deletes a task immediately, and there's no undo yet.
