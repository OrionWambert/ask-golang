package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/OrionWambert/ask-golang/internal/claude"
	"github.com/OrionWambert/ask-golang/internal/config"
	"github.com/OrionWambert/ask-golang/internal/input"
	"github.com/OrionWambert/ask-golang/internal/render"
)

type askFlags struct {
	noHistory    bool
	stats        bool
	copyAnswer   bool
	output       string
	code         bool
	persona      string
	budget       float64
	schema       string
	continueLast bool
	resume       string
	format       string
	model        string
	files        []string
	noStream     bool
	noStdin      bool
}

var flags askFlags

var rootCmd = &cobra.Command{
	Use:   "ask [message]",
	Short: "Ask Claude from the terminal",
	Long:  "ask sends a message to the local claude binary and prints the answer as it is written.",
	Example: `  ask "explain this error"
  ask what is a goroutine
  git diff | ask "write a commit message"
  ask -f main.go "explain this file" --format markdown`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runAsk,
}

func init() {
	f := rootCmd.Flags()
	f.BoolVar(&flags.noHistory, "no-history", false, "do not save this answer to the history")
	f.BoolVar(&flags.stats, "stats", false, "print model, duration, cost and session id on stderr")
	f.BoolVar(&flags.copyAnswer, "copy", false, "copy the answer to the clipboard")
	f.StringVarP(&flags.output, "output", "o", "", "write the answer to a file instead of the terminal")
	f.BoolVar(&flags.code, "code", false, "print only the code blocks of the answer")
	f.StringVar(&flags.persona, "as", "", "use a persona saved with ask config set persona.<name>")
	f.Float64Var(&flags.budget, "budget", 0, "maximum spend for this run, in US dollars")
	f.StringVar(&flags.schema, "schema", "", "file holding a JSON schema the answer must follow")
	f.BoolVarP(&flags.continueLast, "continue", "c", false, "continue the most recent conversation in this folder")
	f.StringVarP(&flags.resume, "resume", "r", "", "resume a conversation by session id")
	rootCmd.MarkFlagsMutuallyExclusive("continue", "resume")
	f.StringVarP(&flags.format, "format", "F", "", "output format: text, markdown or json (default text)")
	f.StringVarP(&flags.model, "model", "m", "", "model to use, such as sonnet or haiku")
	f.StringArrayVarP(&flags.files, "file", "f", nil, "add a file as context (repeatable)")
	f.BoolVar(&flags.noStream, "no-stream", false, "print the answer only once it is complete")
	f.BoolVar(&flags.noStdin, "no-stdin", false, "do not read piped input")
}

// Execute runs the CLI and exits with a non-zero status on failure.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "ask:", err)
		os.Exit(1)
	}
}

func runAsk(cmd *cobra.Command, args []string) error {
	message := strings.Join(args, " ")
	var stdin io.Reader
	if !flags.noStdin && input.IsPiped(os.Stdin) {
		stdin = os.Stdin
	}
	if message == "" && stdin == nil && len(flags.files) == 0 {
		return cmd.Help()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	format := firstNonEmpty(flags.format, cfg.Format, render.Text)
	renderer, err := render.New(format, renderOptions())
	if err != nil {
		return err
	}
	prompt, err := input.Build(input.Source{Message: message, Files: flags.files, Stdin: stdin})
	if err != nil {
		return err
	}
	req, err := buildRequest(prompt, cfg)
	if err != nil {
		return err
	}
	runner, err := claude.NewRunner()
	if err != nil {
		return err
	}

	live := format == render.Text && !flags.noStream && flags.output == "" && !flags.code && req.Schema == ""
	var stream liveWriter
	var onText func(string)
	spin := startSpinner(os.Stderr)
	if live {
		onText = func(text string) {
			spin.stop()
			stream.write(text)
		}
	}
	resp, err := runner.Run(cmd.Context(), req, onText)
	spin.stop()
	stream.finish()
	if err != nil {
		return err
	}
	saveHistory(cfg, message, resp)
	if err := writeAnswer(renderer, resp, live && stream.wrote); err != nil {
		return err
	}
	if flags.stats {
		fmt.Fprintf(os.Stderr, "%s | %.1fs | $%.4f | turns: %d | session %s\n",
			resp.Model, float64(resp.DurationMS)/1000, resp.CostUSD, resp.NumTurns, resp.SessionID)
	}
	if flags.copyAnswer {
		copyToClipboard(resp)
	}
	return nil
}

// copyToClipboard only warns on failure: the answer was already delivered.
func copyToClipboard(resp *claude.Response) {
	text := resp.Text
	if flags.code {
		code, err := codeOnly(resp)
		if err != nil {
			return
		}
		text = code
	}
	if err := clipboard.WriteAll(text); err != nil {
		fmt.Fprintln(os.Stderr, "ask: could not copy to the clipboard:", err)
	}
}

func buildRequest(prompt string, cfg *config.Config) (claude.Request, error) {
	req := claude.Request{Prompt: prompt, Model: firstNonEmpty(flags.model, cfg.Model)}
	req.Continue = flags.continueLast
	req.ResumeID = flags.resume
	if flags.schema != "" {
		schema, err := os.ReadFile(flags.schema)
		if err != nil {
			return req, fmt.Errorf("read schema: %w", err)
		}
		if !json.Valid(schema) {
			return req, fmt.Errorf("read schema: %s is not valid JSON", flags.schema)
		}
		req.Schema = string(schema)
	}
	req.BudgetUSD = cfg.BudgetUSD
	if flags.budget < 0 {
		return req, errors.New("budget must be positive")
	}
	if flags.budget > 0 {
		req.BudgetUSD = flags.budget
	}
	system, err := systemPrompt(cfg)
	if err != nil {
		return req, err
	}
	req.SystemPrompt = system
	return req, nil
}

func systemPrompt(cfg *config.Config) (string, error) {
	var parts []string
	if cfg.SystemPrompt != "" {
		parts = append(parts, cfg.SystemPrompt)
	}
	if flags.persona != "" {
		persona, ok := cfg.Personas[flags.persona]
		if !ok {
			return "", fmt.Errorf("unknown persona %q: add it with ask config set persona.%s \"...\"", flags.persona, flags.persona)
		}
		parts = append(parts, persona)
	}
	return strings.Join(parts, "\n\n"), nil
}

// writeAnswer delivers the final answer, unless it was already streamed.
func writeAnswer(renderer render.Renderer, resp *claude.Response, shown bool) error {
	if shown {
		return nil
	}
	if flags.output == "" {
		return renderAnswer(os.Stdout, renderer, resp)
	}
	var content bytes.Buffer
	if err := renderAnswer(&content, renderer, resp); err != nil {
		return err
	}
	if err := os.WriteFile(flags.output, content.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "saved to", flags.output)
	return nil
}

func renderAnswer(w io.Writer, renderer render.Renderer, resp *claude.Response) error {
	if flags.code {
		code, err := codeOnly(resp)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, code)
		return err
	}
	return renderer.Render(w, resp)
}

func codeOnly(resp *claude.Response) (string, error) {
	blocks := render.CodeBlocks(resp.Text)
	if len(blocks) == 0 {
		return "", errors.New("the answer has no code block")
	}
	return strings.Join(blocks, "\n\n"), nil
}

func renderOptions() render.Options {
	opts := render.Options{
		Styled: isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" && flags.output == "",
		Width:  80,
	}
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		opts.Width = width
	}
	return opts
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// liveWriter prints the answer as it arrives and ends it with a newline.
type liveWriter struct {
	wrote       bool
	endsNewline bool
}

func (l *liveWriter) write(text string) {
	fmt.Fprint(os.Stdout, text)
	l.wrote = true
	l.endsNewline = strings.HasSuffix(text, "\n")
}

func (l *liveWriter) finish() {
	if l.wrote && !l.endsNewline {
		fmt.Fprintln(os.Stdout)
	}
}
