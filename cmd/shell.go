package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OrionWambert/ask-golang/internal/claude"
	"github.com/OrionWambert/ask-golang/internal/config"
	"github.com/OrionWambert/ask-golang/internal/input"
	"github.com/OrionWambert/ask-golang/internal/render"
	"github.com/OrionWambert/ask-golang/internal/shell"
)

// useShell decides whether Claude may propose commands. The saved default leaves
// out runs that expect a precise output. Piped input and files can try to steer
// Claude, so with them every command needs its own approval.
func useShell(cmd *cobra.Command, cfg *config.Config, format string, hasContext bool) (bool, error) {
	wantsOutput := flags.code || flags.schema != "" || flags.output != "" || format == render.JSON
	if cmd.Flags().Changed("shell") {
		if !flags.shell {
			if flags.yes {
				return false, errors.New("--yes only works with --shell")
			}
			return false, nil
		}
		if wantsOutput {
			return false, errors.New("--shell can't be combined with --code, --schema, --output or the json format")
		}
		if flags.yes && hasContext {
			return false, errors.New("--yes can't be combined with piped input or files: text in them could make ask run commands")
		}
		return true, nil
	}
	if flags.yes {
		if !cfg.Shell {
			return false, errors.New("--yes only works with --shell")
		}
		if hasContext {
			return false, errors.New("--yes can't be combined with piped input or files: text in them could make ask run commands")
		}
	}
	tools := cfg.Tools
	if cmd.Flags().Changed("tools") {
		tools = flags.tools
	}
	return cfg.Shell && !tools && !wantsOutput, nil
}

// runShell runs the command loop. A declined command ends it without an error,
// and with no answer to show.
func runShell(ctx context.Context, runner *claude.Runner, req claude.Request, hasContext bool) (*claude.Response, error) {
	folder, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	loop := shell.Loop{
		Ask: func(ctx context.Context, req claude.Request) (*claude.Response, error) {
			spin := startSpinner(os.Stderr)
			defer spin.stop()
			return runner.Run(ctx, req, nil)
		},
		Confirm: func(step shell.Step) (shell.Decision, error) {
			return confirmCommand(step, hasContext)
		},
		Exec: func(ctx context.Context, command string) (shell.Result, error) {
			return shell.Exec(ctx, command, os.Stderr)
		},
		Instructions: shell.Instructions(runtime.GOOS, shell.Name(), folder),
	}
	if flags.yes {
		loop.Confirm = func(step shell.Step) (shell.Decision, error) {
			showCommand(step)
			return shell.Allow, nil
		}
	}
	resp, err := loop.Run(ctx, req)
	switch {
	case errors.Is(err, shell.ErrDeclined):
		fmt.Fprintln(os.Stderr, "Stopped. Nothing else was run. Continue with ask -c.")
		return resp, nil
	case errors.Is(err, shell.ErrBudget):
		return resp, fmt.Errorf("stopped: the budget of $%.2f was reached", req.BudgetUSD)
	}
	return resp, err
}

func showCommand(step shell.Step) {
	if step.Message != "" {
		fmt.Fprintln(os.Stderr, step.Message)
	}
	fmt.Fprintln(os.Stderr, "> "+step.Command)
}

// confirmCommand asks on the terminal, even when stdin holds piped input. Run
// all is only offered when no outside text can influence the commands.
func confirmCommand(step shell.Step, hasContext bool) (shell.Decision, error) {
	answers, closeAnswers, err := openTerminal()
	if err != nil {
		return shell.Deny, errors.New("Claude proposed a command but there is no terminal to approve it: use --yes to run commands without asking")
	}
	defer closeAnswers()
	showCommand(step)
	if hasContext {
		fmt.Fprint(os.Stderr, "Run it? [y/N] ")
	} else {
		fmt.Fprint(os.Stderr, "Run it? [y/N/a = all for this task] ")
	}
	line, err := bufio.NewReader(answers).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return shell.Deny, err
	}
	decision := parseAnswer(line)
	if decision == shell.AllowAll && hasContext {
		return shell.Deny, nil
	}
	return decision, nil
}

func parseAnswer(line string) shell.Decision {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "o", "oui":
		return shell.Allow
	case "a", "all":
		return shell.AllowAll
	}
	return shell.Deny
}

// openTerminal returns where to read the user's answers from.
func openTerminal() (io.Reader, func(), error) {
	if !input.IsPiped(os.Stdin) {
		return os.Stdin, func() {}, nil
	}
	name := "/dev/tty"
	if runtime.GOOS == "windows" {
		name = "CONIN$"
	}
	terminal, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return terminal, func() { terminal.Close() }, nil
}
