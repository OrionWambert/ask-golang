package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OrionWambert/ask-golang/internal/claude"
)

// script answers each call to Ask with the next step and records the requests.
type script struct {
	steps    []Step
	requests []claude.Request
	commands []string
	asked    int
}

func (s *script) ask(_ context.Context, req claude.Request) (*claude.Response, error) {
	s.requests = append(s.requests, req)
	step := s.steps[len(s.requests)-1]
	data, _ := json.Marshal(step)
	return &claude.Response{Structured: data, SessionID: "s1", Model: "m", CostUSD: 0.5, DurationMS: 10, NumTurns: 1}, nil
}

func (s *script) exec(_ context.Context, command string) (Result, error) {
	s.commands = append(s.commands, command)
	return Result{Output: "out of " + command, ExitCode: 3}, nil
}

func (s *script) loop(decisions ...Decision) Loop {
	return Loop{
		Ask: s.ask,
		Confirm: func(Step) (Decision, error) {
			decision := decisions[s.asked]
			s.asked++
			return decision, nil
		},
		Exec:         s.exec,
		Instructions: "rules",
	}
}

func TestRunWorksThroughEveryStep(t *testing.T) {
	s := &script{steps: []Step{
		{Message: "make a branch", Command: "git switch -c one"},
		{Message: "and another", Command: "git switch -c two"},
		{Message: "all done"},
	}}
	resp, err := s.loop(Allow, Allow).Run(context.Background(), claude.Request{Prompt: "go", Continue: true, Tools: true, SystemPrompt: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "all done" || resp.CostUSD != 1.5 || resp.NumTurns != 3 || resp.SessionID != "s1" {
		t.Fatalf("got %+v", resp)
	}
	if len(s.commands) != 2 || s.commands[1] != "git switch -c two" {
		t.Fatalf("commands = %v", s.commands)
	}

	first, next := s.requests[0], s.requests[1]
	if first.Tools || first.Schema == "" || first.SystemPrompt != "rules\n\nmine" || !first.Continue || first.Prompt != "go" {
		t.Fatalf("first request = %+v", first)
	}
	if next.ResumeID != "s1" || next.Continue {
		t.Fatalf("next step does not resume: %+v", next)
	}
	for _, want := range []string{"git switch -c one", "Exit code: 3", "out of git switch -c one"} {
		if !strings.Contains(next.Prompt, want) {
			t.Errorf("result prompt lacks %q: %s", want, next.Prompt)
		}
	}
}

func TestDecliningStopsWithoutRunning(t *testing.T) {
	s := &script{steps: []Step{{Message: "m", Command: "rm -rf x"}}}
	resp, err := s.loop(Deny).Run(context.Background(), claude.Request{Prompt: "go"})
	if !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v", err)
	}
	if len(s.commands) != 0 {
		t.Fatalf("ran %v", s.commands)
	}
	if resp.CostUSD != 0.5 {
		t.Fatalf("spending lost: %+v", resp)
	}
}

func TestAllowAllStopsAsking(t *testing.T) {
	s := &script{steps: []Step{{Command: "a"}, {Command: "b"}, {Command: "c"}, {Message: "done"}}}
	if _, err := s.loop(AllowAll).Run(context.Background(), claude.Request{Prompt: "go"}); err != nil {
		t.Fatal(err)
	}
	if s.asked != 1 || len(s.commands) != 3 {
		t.Fatalf("asked %d times, ran %v", s.asked, s.commands)
	}
}

func TestNoCommandIsTheFinalAnswer(t *testing.T) {
	s := &script{steps: []Step{{Message: "a goroutine is...", Command: "   "}}}
	resp, err := s.loop().Run(context.Background(), claude.Request{Prompt: "what?"})
	if err != nil || resp.Text != "a goroutine is..." || len(s.commands) != 0 {
		t.Fatalf("resp %+v, err %v, commands %v", resp, err, s.commands)
	}
}

func TestPlainTextIsTheFinalAnswer(t *testing.T) {
	loop := Loop{Ask: func(context.Context, claude.Request) (*claude.Response, error) {
		return &claude.Response{Text: "plain"}, nil
	}}
	resp, err := loop.Run(context.Background(), claude.Request{Prompt: "go"})
	if err != nil || resp.Text != "plain" {
		t.Fatalf("resp %+v, err %v", resp, err)
	}
}

func TestBudgetCoversTheWholeRun(t *testing.T) {
	s := &script{steps: []Step{{Command: "a"}, {Command: "b"}, {Command: "c"}}}
	_, err := s.loop(Allow, Allow, Allow).Run(context.Background(), claude.Request{Prompt: "go", BudgetUSD: 0.75})
	if !errors.Is(err, ErrBudget) {
		t.Fatalf("err = %v", err)
	}
	if len(s.requests) != 2 || s.requests[0].BudgetUSD != 0.75 || s.requests[1].BudgetUSD != 0.25 {
		t.Fatalf("requests = %+v", s.requests)
	}
}

func TestRunGivesUpEventually(t *testing.T) {
	calls := 0
	loop := Loop{
		Ask: func(context.Context, claude.Request) (*claude.Response, error) {
			calls++
			return &claude.Response{Structured: []byte(`{"message":"m","command":"again"}`), SessionID: "s"}, nil
		},
		Confirm: func(Step) (Decision, error) { return Allow, nil },
		Exec:    func(context.Context, string) (Result, error) { return Result{}, nil },
	}
	if _, err := loop.Run(context.Background(), claude.Request{Prompt: "go"}); !errors.Is(err, ErrTooManySteps) || calls != MaxSteps {
		t.Fatalf("err %v after %d calls", err, calls)
	}
}

func TestStartFailureGoesBackToClaude(t *testing.T) {
	var prompts []string
	calls := 0
	loop := Loop{
		Ask: func(_ context.Context, req claude.Request) (*claude.Response, error) {
			prompts = append(prompts, req.Prompt)
			calls++
			if calls == 1 {
				return &claude.Response{Structured: []byte(`{"message":"m","command":"x"}`), SessionID: "s"}, nil
			}
			return &claude.Response{Structured: []byte(`{"message":"sorry"}`), SessionID: "s"}, nil
		},
		Confirm: func(Step) (Decision, error) { return Allow, nil },
		Exec:    func(context.Context, string) (Result, error) { return Result{}, errors.New("no shell") },
	}
	if _, err := loop.Run(context.Background(), claude.Request{Prompt: "go"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompts[1], "no shell") {
		t.Fatalf("prompt = %q", prompts[1])
	}
}

func TestInstructionsNeverDenyAccess(t *testing.T) {
	text := Instructions("windows", "PowerShell", "C:/project")
	if strings.Contains(text, "can't use tools") || !strings.Contains(text, "never say that you lack tools") {
		t.Fatalf("instructions = %s", text)
	}
	for _, want := range []string{"windows", "PowerShell", "C:/project"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestExecReportsOutputAndExitCode(t *testing.T) {
	var shown bytes.Buffer
	result, err := Exec(context.Background(), "echo hello; exit 3", &shown)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 3 || !strings.Contains(result.Output, "hello") || !strings.Contains(shown.String(), "hello") {
		t.Fatalf("result %+v, shown %q", result, shown.String())
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	tail := &tailBuffer{limit: 5}
	tail.Write([]byte("abc"))
	if got := tail.String(); got != "abc" {
		t.Fatalf("got %q", got)
	}
	tail.Write([]byte("defgh"))
	if got := tail.String(); !strings.HasSuffix(got, "defgh") || !strings.Contains(got, "removed") || strings.Contains(got, "abc") {
		t.Fatalf("got %q", got)
	}
}
