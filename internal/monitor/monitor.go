// Package monitor gathers what the ask monitor dashboard shows: running Claude
// Code sessions, token use, plan limits, ask's own spending and the public
// Claude status.
package monitor

import (
	"os"
	"path/filepath"
	"time"

	"github.com/OrionWambert/ask-golang/internal/history"
)

// Local is everything that can be read from this machine.
type Local struct {
	Sessions []Session
	Usage    Usage
	Limits   RateLimits
	Ask      AskStats
}

// AskStats sums the answers saved in ask's history.
type AskStats struct {
	AnswersToday int
	AnswersWeek  int
	CostToday    float64
	CostWeek     float64
}

// Collector reads local data. Its methods must not run concurrently.
type Collector struct {
	ClaudeDir string
	AskDir    string
	Now       func() time.Time

	tail       tailer
	historyMod time.Time
	ask        []history.Entry
}

// NewCollector returns a Collector for the given ask data folder.
func NewCollector(askDir string) (*Collector, error) {
	claudeDir, err := ClaudeDir()
	if err != nil {
		return nil, err
	}
	return &Collector{ClaudeDir: claudeDir, AskDir: askDir, Now: time.Now}, nil
}

// ClaudeDir returns Claude Code's data folder. CLAUDE_CONFIG_DIR overrides it.
func ClaudeDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// Local reads the current state. Anything missing or unreadable stays empty, so
// the dashboard keeps running.
func (c *Collector) Local() Local {
	now := c.Now()
	usage := c.tail.scan(filepath.Join(c.ClaudeDir, "projects"), now)
	return Local{
		Sessions: readSessions(filepath.Join(c.ClaudeDir, "sessions"), usage.BySession),
		Usage:    usage,
		Limits:   LoadRateLimits(c.AskDir, now),
		Ask:      c.askStats(now),
	}
}

// askStats only rereads the history when its folder has changed.
func (c *Collector) askStats(now time.Time) AskStats {
	dir := filepath.Join(c.AskDir, "history")
	if info, err := os.Stat(dir); err == nil && !info.ModTime().Equal(c.historyMod) {
		if entries, err := (history.Store{Dir: dir}).List(0); err == nil {
			c.ask = entries
			c.historyMod = info.ModTime()
		}
	}
	today := startOfDay(now)
	week := today.AddDate(0, 0, -6)
	var stats AskStats
	for _, entry := range c.ask {
		if entry.Time.Before(week) {
			continue
		}
		stats.AnswersWeek++
		stats.CostWeek += entry.CostUSD
		if !entry.Time.Before(today) {
			stats.AnswersToday++
			stats.CostToday += entry.CostUSD
		}
	}
	return stats
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
