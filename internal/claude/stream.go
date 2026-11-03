package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ResultError is an error reported by claude in its final result event.
type ResultError struct {
	Message string
}

func (e *ResultError) Error() string {
	return e.Message
}

// ErrNoResult means the stream ended without a result event.
var ErrNoResult = errors.New("claude exited without a result")

type streamLine struct {
	Type       string       `json:"type"`
	Subtype    string       `json:"subtype"`
	SessionID  string       `json:"session_id"`
	ParentTool *string      `json:"parent_tool_use_id"`
	Event      *streamEvent `json:"event"`

	IsError    bool            `json:"is_error"`
	Result     string          `json:"result"`
	Structured json.RawMessage `json:"structured_output"`
	Errors     []string        `json:"errors"`
	CostUSD    float64         `json:"total_cost_usd"`
	DurationMS int64           `json:"duration_ms"`
	NumTurns   int             `json:"num_turns"`
}

type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

// ParseStream reads claude's stream-json output and returns the final response.
// onText, when set, receives the answer as it is written. Lines that are not
// JSON are ignored, since claude can print diagnostics among its events.
func ParseStream(r io.Reader, onText func(string)) (*Response, error) {
	resp := &Response{}
	var resultErr error
	gotResult, wroteText := false, false

	br := bufio.NewReader(r)
	for {
		raw, readErr := br.ReadBytes('\n')
		var line streamLine
		if err := json.Unmarshal(bytes.TrimSpace(raw), &line); err == nil {
			switch line.Type {
			case "stream_event":
				if line.Event == nil || line.ParentTool != nil {
					break
				}
				switch line.Event.Type {
				case "message_start":
					resp.Model = line.Event.Message.Model
				case "content_block_start":
					if line.Event.ContentBlock.Type == "text" && wroteText && onText != nil {
						onText("\n\n")
					}
				case "content_block_delta":
					if line.Event.Delta.Type == "text_delta" && line.Event.Delta.Text != "" {
						wroteText = true
						if onText != nil {
							onText(line.Event.Delta.Text)
						}
					}
				}
			case "result":
				gotResult = true
				resp.Text = line.Result
				resp.Structured = line.Structured
				resp.SessionID = line.SessionID
				resp.CostUSD = line.CostUSD
				resp.DurationMS = line.DurationMS
				resp.NumTurns = line.NumTurns
				if line.IsError {
					resultErr = &ResultError{Message: resultMessage(line)}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return resp, readErr
		}
	}

	if !gotResult {
		return resp, ErrNoResult
	}
	return resp, resultErr
}

func resultMessage(line streamLine) string {
	if len(line.Errors) > 0 {
		return strings.Join(line.Errors, "; ")
	}
	if line.Result != "" {
		return line.Result
	}
	return "claude reported an error: " + line.Subtype
}
