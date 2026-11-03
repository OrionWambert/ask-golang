package claude

import "encoding/json"

// Request describes one question sent to the claude binary.
type Request struct {
	Prompt       string
	Model        string
	Continue     bool
	ResumeID     string
	SystemPrompt string
	Schema       string
	BudgetUSD    float64
	Tools        bool
}

// Response is the outcome of a claude run.
type Response struct {
	Text       string          `json:"result"`
	Structured json.RawMessage `json:"structured_output,omitempty"`
	SessionID  string          `json:"session_id"`
	Model      string          `json:"model,omitempty"`
	CostUSD    float64         `json:"cost_usd"`
	DurationMS int64           `json:"duration_ms"`
	NumTurns   int             `json:"num_turns"`
}
