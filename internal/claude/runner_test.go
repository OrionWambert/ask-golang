package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for claude when ASK_FAKE_CLAUDE is set.
func TestMain(m *testing.M) {
	switch os.Getenv("ASK_FAKE_CLAUDE") {
	case "":
		os.Exit(m.Run())
	case "echo":
		prompt, _ := io.ReadAll(os.Stdin)
		answer, _ := json.Marshal(string(prompt) + "|" + strings.Join(os.Args[1:], " "))
		fmt.Printf(`{"type":"result","is_error":false,"result":%s,"session_id":"s1"}`+"\n", answer)
	case "crash":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(3)
	case "budget":
		fmt.Println(`{"type":"result","is_error":true,"errors":["over budget"]}`)
		os.Exit(1)
	case "hang":
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func fakeRunner(t *testing.T, mode string) *Runner {
	t.Helper()
	t.Setenv("ASK_FAKE_CLAUDE", mode)
	return &Runner{Binary: os.Args[0]}
}

func TestRunSendsPromptThroughStdin(t *testing.T) {
	resp, err := fakeRunner(t, "echo").Run(context.Background(), Request{Prompt: "hello", Model: "haiku"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	prompt, args, _ := strings.Cut(resp.Text, "|")
	if prompt != "hello" {
		t.Errorf("stdin prompt = %q", prompt)
	}
	if !strings.Contains(args, "--model haiku") || strings.Contains(args, "hello") {
		t.Errorf("args = %q", args)
	}
}

func TestRunReportsStderrOnCrash(t *testing.T) {
	_, err := fakeRunner(t, "crash").Run(context.Background(), Request{}, nil)
	if !errors.Is(err, ErrNoResult) || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPrefersResultErrorOverExitCode(t *testing.T) {
	_, err := fakeRunner(t, "budget").Run(context.Background(), Request{}, nil)
	var resultErr *ResultError
	if !errors.As(err, &resultErr) || resultErr.Message != "over budget" {
		t.Fatalf("err = %v", err)
	}
}

func TestRunStopsClaudeWhenCancelled(t *testing.T) {
	runner := fakeRunner(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runner.Run(ctx, Request{}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("took %s to stop", elapsed)
	}
}

func TestBuildArgsDefaults(t *testing.T) {
	got := buildArgs(Request{Prompt: "secret"})
	want := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--tools", "", "--strict-mcp-config"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildArgsOptions(t *testing.T) {
	got := buildArgs(Request{
		Model:        "haiku",
		Continue:     true,
		ResumeID:     "abc",
		SystemPrompt: "be brief",
		Schema:       "{}",
		BudgetUSD:    0.5,
		Tools:        true,
	})
	want := []string{
		"--model", "haiku", "--continue", "--resume", "abc",
		"--append-system-prompt", "be brief", "--json-schema", "{}", "--max-budget-usd", "0.5",
	}
	if !slices.Equal(got[5:], want) {
		t.Fatalf("got %v, want %v", got[5:], want)
	}
}
