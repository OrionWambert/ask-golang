package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrNotInstalled means the claude binary is not on the PATH.
var ErrNotInstalled = errors.New("claude binary not found on PATH: install Claude Code first")

// Runner executes the claude binary.
type Runner struct {
	Binary string
}

// NewRunner locates the claude binary on the PATH.
func NewRunner() (*Runner, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil, ErrNotInstalled
	}
	return &Runner{Binary: path}, nil
}

// Run sends req to claude and waits for the answer. onText, when set, receives
// the answer as it is written. Cancelling ctx stops the claude process.
func (r *Runner) Run(ctx context.Context, req Request, onText func(string)) (*Response, error) {
	cmd := exec.CommandContext(ctx, r.Binary, buildArgs(req)...)
	cmd.Stdin = strings.NewReader(req.Prompt)
	cmd.WaitDelay = 2 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	resp, parseErr := ParseStream(stdout, onText)
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var resultErr *ResultError
	if errors.As(parseErr, &resultErr) {
		return resp, resultErr
	}
	if parseErr != nil || waitErr != nil {
		cause := parseErr
		if cause == nil {
			cause = waitErr
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return nil, fmt.Errorf("%w: %s", cause, detail)
		}
		return nil, cause
	}
	return resp, nil
}

// buildArgs returns the claude arguments for req. The prompt itself goes
// through stdin, so long messages never hit the command line length limit.
func buildArgs(req Request) []string {
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Continue {
		args = append(args, "--continue")
	}
	if req.ResumeID != "" {
		args = append(args, "--resume", req.ResumeID)
	}
	if req.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", req.SystemPrompt)
	}
	if req.Schema != "" {
		args = append(args, "--json-schema", req.Schema)
	}
	if req.BudgetUSD > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(req.BudgetUSD, 'f', -1, 64))
	}
	return args
}
