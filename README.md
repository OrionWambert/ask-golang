# ask

Ask Claude from your terminal. `ask` sends a message to the local `claude` binary and prints the answer as it is written, in the format you pick.

```
ask "explain this error"
git diff | ask "write a commit message"
ask -f main.go "explain this file" --format markdown
```

## Requirements

- [Claude Code](https://code.claude.com/docs): the `claude` binary must be on your PATH and logged in.
- [Go](https://go.dev/dl/) 1.24 or later, to build from source.

## Install

Run this from the project folder.

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```

This installs `ask.exe` to `%LOCALAPPDATA%\Programs\ask` and adds that folder to your user PATH. Open a new terminal afterwards. Use `-InstallDir <folder>` to change the location and `-NoPath` to leave your PATH alone.

**Linux and macOS**

```sh
./scripts/install.sh
```

This installs `ask` to `~/.local/bin`. Set `BIN_DIR` to change the location. If the folder is not on your PATH, the script prints the line to add. `make install` does the same.

**Uninstall**

```
powershell -ExecutionPolicy Bypass -File .\scripts\uninstall.ps1
./scripts/uninstall.sh
```

Your config and history are kept.

## Usage

```
ask [message] [flags]
```

Quotes are optional: `ask what is a goroutine` works. If your message starts with a command name (`config`, `history`, `version`, `completion`, `help`), quote the whole message or write `ask -- version`.

### Output

| Flag | Effect |
| --- | --- |
| `-F, --format <name>` | `text` (default, streamed live), `markdown` (styled in a terminal, raw in a pipe or file), or `json` |
| `--no-stream` | Print the answer only once it is complete |
| `--code` | Print only the code blocks, ready to pipe. Fails if the answer has none |
| `-o, --output <file>` | Write the answer to a file instead of the terminal |
| `--copy` | Also copy the answer to the clipboard. On Linux this needs `xclip`, `xsel` or similar |
| `--stats` | Print model, duration, cost and session id on stderr |

Set `NO_COLOR` to turn off styling. The spinner, stats and notices go to stderr, so stdout stays clean for pipes:

```
ask "list 3 colors as a JSON array" --code | jq .
```

### Input

| Flag | Effect |
| --- | --- |
| `-f, --file <path>` | Add a file as context. Repeatable |
| `--no-stdin` | Do not read piped input. Use it if `ask` waits forever when run from a script or another program |

Piped input is read automatically. Files and piped input come before your message in the prompt.

### Claude options

| Flag | Effect |
| --- | --- |
| `-m, --model <name>` | Model to use, such as `sonnet` or `haiku` |
| `-c, --continue` | Continue the most recent conversation in this folder |
| `-r, --resume <id>` | Resume a conversation by session id |
| `--schema <file>` | JSON schema the answer must follow |
| `--as <persona>` | Use a saved persona |
| `--budget <usd>` | Maximum spend for this run. `claude` checks it between steps, so a run can end slightly over the limit, and it then exits with an error |

`ask` runs `claude -p`, so Claude can use its tools according to your Claude Code permission settings, in the current folder.

### Examples

```
# Follow up on the last answer
ask "what is a mutex?"
ask -c "show me an example in Go"

# Structured output
ask "extract the name and age: Ana, 31" --schema person.json

# Save a generated script
ask "bash script that lists large files" --code -o big.sh

# Reuse a persona
ask config set persona.reviewer "You review code and only list real bugs."
git diff | ask --as reviewer "review this"
```

## Piping

Pipe any command into `ask`. Add a message to say what you want, or leave it out if the piped text is the question itself.

```powershell
git diff | ask "write a commit message"
Get-ChildItem | ask "which file looks the largest?"
"translate this into English: bonjour" | ask
```

Errors are not piped by default. Add `2>&1` to include them, which is what you want for a failing build or test run:

```
npm test 2>&1 | ask "why does this fail?"
```

Piping combines with the other flags:

```
git diff | ask "fix this" --code -o fix.patch
```

In Windows PowerShell, if accents turn into `?`, set the pipe encoding to UTF-8. Add this line to your `$PROFILE` to keep it:

```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new()
```

## Config

Flags always win over saved defaults.

```
ask config set model haiku
ask config set format markdown
ask config get model
ask config unset model
ask config list
ask config path
```

| Key | Value |
| --- | --- |
| `model` | Default model |
| `format` | Default format |
| `system_prompt` | Text added to Claude's system prompt on every run |
| `budget` | Default maximum spend per run, in US dollars |
| `history` | `true` or `false` |
| `persona.<name>` | Text added to the system prompt with `--as <name>` |

The config lives in `ask/config.json` in your user config folder (`%APPDATA%` on Windows, `~/.config` on Linux, `~/Library/Application Support` on macOS). Set `ASK_CONFIG_DIR` to use another folder.

## History

Each answer is saved as a JSON file in the `history` folder next to the config.

```
ask history              # latest 20
ask history -n 0         # all
ask history show <id>
ask history clear        # deletes everything, without asking
```

Your message and the answer are saved. File and piped content is not. Skip one run with `--no-history`, or turn it off with `ask config set history false`.

## Shell completion

```
ask completion powershell | Out-String | Invoke-Expression
ask completion bash
ask completion zsh
ask completion fish
```

Run `ask completion <shell> --help` to make it permanent.

## Development

```
make build    # or: go build -o ask.exe .
make test     # or: go vet ./... && go test ./...
```

`cmd/` holds the Cobra commands, and `internal/` holds the logic. See [PROJECT.md](PROJECT.md) for the design.
