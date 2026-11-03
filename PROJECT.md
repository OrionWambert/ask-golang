# ask CLI: framework, structure and ideas

`ask` is a Go CLI that sends a message to the local `claude` binary and prints the answer directly in the terminal, in the format you pick.

```
ask "explain this error"
ask "list the Go CLI frameworks" --format json
git diff | ask "write a commit message"
```

The binary itself is named `ask`, so the message is the root command's argument.

## CLI framework: Cobra

Use [Cobra](https://github.com/spf13/cobra). It is the de facto Go CLI standard (kubectl, gh and Hugo use it), and it covers subcommands, inherited flags, shell completion and generated help.

| Option | Verdict | Why |
| --- | --- | --- |
| Cobra | Chosen | Subcommands such as `ask config` or `ask history` stay simple, and completion for PowerShell, bash, zsh and fish is built in |
| urfave/cli | Rejected | Solid, but fewer examples and a smaller ecosystem around it |
| Kong | Rejected | Elegant struct-based flags, but less suited to a command tree that grows over time |
| Standard `flag` | Rejected | Fine for one command, but subcommands and completion would be yours to write |

## Project structure

Cobra stays in `cmd/` and all logic in `internal/`, so calling Claude and rendering output can be tested without a terminal.

```
ask-golang/
├── main.go               # only calls cmd.Execute()
├── go.mod
├── go.sum
├── README.md
├── PROJECT.md
├── cmd/
│   ├── root.go           # ask "message": flags and wiring
│   ├── config.go         # ask config
│   └── version.go        # ask version
└── internal/
    ├── input/
    │   └── input.go      # merges the argument, stdin and --file
    ├── claude/
    │   ├── runner.go     # builds the arguments and runs the claude binary
    │   ├── stream.go     # parses stream-json events
    │   └── types.go      # Request, Response, Event
    ├── render/
    │   ├── render.go     # Renderer interface, picked from --format
    │   ├── text.go
    │   ├── markdown.go
    │   └── json.go
    └── config/
        └── config.go     # defaults: model, format, system prompt
```

How one question moves through the packages:

```
internal/input --> cmd --> internal/claude <--> claude binary
                    ^            |
                    |            | events
             internal/config     v
                           internal/render --> stdout
```

Things to know:

- **Binary name:** `go build` produces `ask-golang.exe` from the folder name, so build with `go build -o ask.exe .`.
- **Module path:** `go.mod` says `oriowambert.com`. Check whether that is a typo for `orionwambert.com`.

## Feature ideas

Start with the first version: a message in, a streamed answer out, and `--format`. Most other ideas are thin wrappers over `claude` flags checked against version 2.1.302.

### First version

| Idea | Usage | Built on |
| --- | --- | --- |
| Plain question | `ask "message"` | `claude -p` |
| Live answer as it is written | on by default | `--output-format stream-json`, `--include-partial-messages` |
| Output format | `--format text\|markdown\|json` | `render` package, `--output-format json` |
| Piped context | `git diff \| ask "review this"` | stdin passed through to `claude` |
| Model choice | `--model sonnet` | `--model` |

### Next

| Idea | Usage | Built on |
| --- | --- | --- |
| Follow-up question | `ask -c "and in Go?"` | `--continue` |
| Back to an old conversation | `ask --resume <id>` | `--resume` |
| Structured output | `ask "..." --schema schema.json` | `--json-schema` |
| Reusable personas | `ask --as reviewer "..."` | named prompts in config, `--append-system-prompt` |
| Spending limit | `--budget 0.50` | `--max-budget-usd` |
| Files as context | `ask -f main.go "explain"` | `input` package |

### Later

| Idea | Usage | Built on |
| --- | --- | --- |
| Styled Markdown | `--format markdown` | [Glamour](https://github.com/charmbracelet/glamour) |
| Code only | `--code` prints only code blocks, ready to pipe | parsing in `render` |
| Copy the answer | `--copy` | clipboard library |
| Write to a file | `-o answer.md` | `render` writer |
| Local history | `ask history` | a JSON file per answer in the user config folder |
| Cost and duration footer | `--stats` | fields of the JSON result |
| Shell completion | `ask completion powershell` | built into Cobra |

### Implementation notes

- Detect a terminal: when stdout is piped, turn off colors and spinners so `ask "..." | jq` stays clean.
- Send long messages through stdin rather than an argument, since Windows limits command line length.
- Pass a `context.Context` to `exec.CommandContext` so Ctrl+C stops the `claude` process.
- Check that `claude` is on the PATH and return a clear error if not.
- From memory, not tested here: `stream-json` with `-p` also needs `--verbose`. Confirm it on your first run.
