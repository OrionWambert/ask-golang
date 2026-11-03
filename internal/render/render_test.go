package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"oriowambert.com/ask-golang/internal/claude"
)

func renderString(t *testing.T, format string, opts Options, resp *claude.Response) string {
	t.Helper()
	r, err := New(format, opts)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := r.Render(&out, resp); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestNewRejectsUnknownFormat(t *testing.T) {
	if _, err := New("yaml", Options{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTextEndsWithOneNewline(t *testing.T) {
	got := renderString(t, Text, Options{}, &claude.Response{Text: "hello\n\n"})
	if got != "hello\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTextIndentsStructuredOutput(t *testing.T) {
	got := renderString(t, Text, Options{}, &claude.Response{Text: "{}", Structured: json.RawMessage(`{"n":2}`)})
	if got != "{\n  \"n\": 2\n}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMarkdownUnstyledIsRaw(t *testing.T) {
	got := renderString(t, Markdown, Options{}, &claude.Response{Text: "# Title"})
	if got != "# Title\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMarkdownStyledGoesThroughGlamour(t *testing.T) {
	got := renderString(t, Markdown, Options{Styled: true, Width: 80}, &claude.Response{Text: "some word"})
	if !strings.Contains(got, "some word") || got == "some word\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONIsValidAndKeepsMarkup(t *testing.T) {
	got := renderString(t, JSON, Options{}, &claude.Response{Text: "a <b>", SessionID: "s1", CostUSD: 0.5})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["result"] != "a <b>" || decoded["session_id"] != "s1" || decoded["cost_usd"] != 0.5 {
		t.Fatalf("unexpected json: %v", decoded)
	}
	if !strings.Contains(got, "a <b>") {
		t.Fatalf("markup escaped: %s", got)
	}
}
