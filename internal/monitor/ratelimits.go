package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Window is one plan limit, as Claude Code reports it to status line commands.
type Window struct {
	UsedPercent float64 `json:"used_percentage"`
	ResetsAt    int64   `json:"resets_at"`
}

// RateLimits is the last plan usage reported by a Claude Code session.
type RateLimits struct {
	FiveHour  *Window   `json:"five_hour,omitempty"`
	SevenDay  *Window   `json:"seven_day,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func limitsPath(askDir string) string {
	return filepath.Join(askDir, "usage.json")
}

// RecordStatusLine reads the JSON Claude Code sends to a status line command,
// saves its plan usage and returns the line to display. Saving is best effort,
// since a status line has no way to report a failure. Claude Code only sends
// rate_limits to some plans and after a session's first response, so anything
// missing keeps its saved value.
func RecordStatusLine(r io.Reader, askDir string, now time.Time) string {
	var input struct {
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
		RateLimits *RateLimits `json:"rate_limits"`
	}
	data, err := io.ReadAll(r)
	if err != nil || json.Unmarshal(data, &input) != nil {
		return "ask monitor"
	}
	limits := LoadRateLimits(askDir, now)
	if input.RateLimits != nil {
		if input.RateLimits.FiveHour != nil {
			limits.FiveHour = input.RateLimits.FiveHour
		}
		if input.RateLimits.SevenDay != nil {
			limits.SevenDay = input.RateLimits.SevenDay
		}
		limits.UpdatedAt = now
		saveRateLimits(askDir, limits)
	}

	var parts []string
	if input.Model.DisplayName != "" {
		parts = append(parts, input.Model.DisplayName)
	}
	if limits.FiveHour != nil {
		parts = append(parts, fmt.Sprintf("5h %.0f%%", limits.FiveHour.UsedPercent))
	}
	if limits.SevenDay != nil {
		parts = append(parts, fmt.Sprintf("7d %.0f%%", limits.SevenDay.UsedPercent))
	}
	if len(parts) == 0 {
		return "ask monitor"
	}
	return strings.Join(parts, " · ")
}

// LoadRateLimits returns the saved plan usage, without windows that have reset.
func LoadRateLimits(askDir string, now time.Time) RateLimits {
	var limits RateLimits
	data, err := os.ReadFile(limitsPath(askDir))
	if err != nil || json.Unmarshal(data, &limits) != nil {
		return RateLimits{}
	}
	if limits.FiveHour != nil && limits.FiveHour.ResetsAt <= now.Unix() {
		limits.FiveHour = nil
	}
	if limits.SevenDay != nil && limits.SevenDay.ResetsAt <= now.Unix() {
		limits.SevenDay = nil
	}
	return limits
}

// saveRateLimits replaces the file in one step: several sessions can write it
// while the dashboard reads it.
func saveRateLimits(askDir string, limits RateLimits) {
	data, err := json.Marshal(limits)
	if err != nil || os.MkdirAll(askDir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(askDir, "usage-*.tmp")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err != nil || closeErr != nil {
		return
	}
	os.Rename(tmp.Name(), limitsPath(askDir))
}
