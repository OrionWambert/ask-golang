package claude

import (
	"errors"
	"strings"
	"testing"
)

const successStream = `{"type":"system","subtype":"init","session_id":"s1"}
[claude-code:notice] not json
{"type":"stream_event","event":{"type":"message_start","message":{"model":"claude-haiku-5-5"}},"parent_tool_use_id":null}
{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text","text":""}},"parent_tool_use_id":null}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"hel"}},"parent_tool_use_id":null}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"lo"}},"parent_tool_use_id":null}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"subagent"}},"parent_tool_use_id":"tool1"}
{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text","text":""}},"parent_tool_use_id":null}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"bye"}},"parent_tool_use_id":null}
{"type":"result","subtype":"success","is_error":false,"result":"bye","session_id":"s1","total_cost_usd":0.25,"duration_ms":1200,"num_turns":2}
`

func TestParseStreamSuccess(t *testing.T) {
	var live strings.Builder
	resp, err := ParseStream(strings.NewReader(successStream), func(s string) { live.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if got, want := live.String(), "hello\n\nbye"; got != want {
		t.Errorf("live text = %q, want %q", got, want)
	}
	if resp.Text != "bye" || resp.SessionID != "s1" || resp.Model != "claude-haiku-5-5" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.CostUSD != 0.25 || resp.DurationMS != 1200 || resp.NumTurns != 2 {
		t.Errorf("unexpected stats: %+v", resp)
	}
}

func TestParseStreamStructuredOutput(t *testing.T) {
	in := `{"type":"result","is_error":false,"result":"{\"n\":2}","structured_output":{"n":2}}`
	resp, err := ParseStream(strings.NewReader(in), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Structured) != `{"n":2}` {
		t.Errorf("structured = %s", resp.Structured)
	}
}

func TestParseStreamErrors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"errors list", `{"type":"result","subtype":"error_max_budget_usd","is_error":true,"errors":["Reached maximum budget"]}`, "Reached maximum budget"},
		{"result text", `{"type":"result","subtype":"success","is_error":true,"result":"bad model"}`, "bad model"},
		{"subtype only", `{"type":"result","subtype":"error_during_execution","is_error":true}`, "claude reported an error: error_during_execution"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStream(strings.NewReader(tt.in), nil)
			var resultErr *ResultError
			if !errors.As(err, &resultErr) {
				t.Fatalf("err = %v, want ResultError", err)
			}
			if resultErr.Message != tt.want {
				t.Errorf("message = %q, want %q", resultErr.Message, tt.want)
			}
		})
	}
}

func TestParseStreamNoResult(t *testing.T) {
	_, err := ParseStream(strings.NewReader("Error: boom\n"), nil)
	if !errors.Is(err, ErrNoResult) {
		t.Fatalf("err = %v, want ErrNoResult", err)
	}
}
