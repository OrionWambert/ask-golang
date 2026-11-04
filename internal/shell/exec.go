package shell

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// maxOutput is how much of a command's output goes back to Claude. The end of
// the output is kept, as that is where errors usually are.
const maxOutput = 20000

// Name returns the shell commands run in.
func Name() string {
	if runtime.GOOS == "windows" {
		return "Windows PowerShell 5.1, which has no && or || (separate commands with ;)"
	}
	return unixShell()
}

func unixShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "sh"
}

// Exec runs command in the user's shell, in the current folder, and copies its
// output to w. A command that fails is a normal result: the error only means it
// could not be started. It gets no input, so it can't wait for an answer forever.
func Exec(ctx context.Context, command string, w io.Writer) (Result, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Without this, accents from native programs arrive in the console's old encoding.
		script := "[Console]::OutputEncoding = [Text.Encoding]::UTF8\n" + command
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	} else {
		cmd = exec.CommandContext(ctx, unixShell(), "-c", command)
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1")
	cmd.WaitDelay = 2 * time.Second
	tail := &tailBuffer{limit: maxOutput}
	output := io.MultiWriter(w, tail)
	cmd.Stdout, cmd.Stderr = output, output

	err := cmd.Run()
	result := Result{Output: tail.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	limit   int
	data    []byte
	dropped bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.data = append(t.data, p...)
	if extra := len(t.data) - t.limit; extra > 0 {
		t.data = append(t.data[:0], t.data[extra:]...)
		t.dropped = true
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	text := strings.ToValidUTF8(string(t.data), "")
	if t.dropped {
		return "[start of the output removed]\n" + text
	}
	return text
}
