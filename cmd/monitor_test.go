package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OrionWambert/ask-golang/internal/monitor"
)

func testModel(width, height int) monitorModel {
	now := time.Date(2026, 11, 3, 15, 0, 0, 0, time.Local)
	m := newMonitorModel(context.Background(), nil, time.Second)
	m.now = func() time.Time { return now }
	m.width, m.height = width, height
	m.loaded = true
	m.syncedAt = now
	m.claudeVersion = "2.1.302"
	reset := now.Add(90 * time.Minute).Unix()
	m.local = monitor.Local{
		Sessions: []monitor.Session{
			{ID: "s1", Name: "a very long session name that will not fit in any of these panels at all", Status: "busy", Started: now.Add(-time.Hour), Tokens: 1500},
			{ID: "s2", Folder: "project", Status: "idle", Started: now.Add(-time.Minute)},
		},
		Usage: monitor.Usage{
			Week:   monitor.Tokens{Input: 2000, Output: 1000, CacheRead: 5e6},
			Daily:  [7]int64{0, 1, 2, 3, 4, 5, 6},
			Models: []monitor.ModelUsage{{Model: "claude-opus-5-5", Used: 3000}, {Model: "claude-haiku-5-5", Used: 500}},
		},
		Limits: monitor.RateLimits{
			FiveHour:  &monitor.Window{UsedPercent: 42, ResetsAt: reset},
			SevenDay:  &monitor.Window{UsedPercent: 96, ResetsAt: reset},
			UpdatedAt: now.Add(-3 * time.Minute),
		},
		Ask: monitor.AskStats{AnswersToday: 2, AnswersWeek: 9, CostToday: 0.25, CostWeek: 1.5},
	}
	m.status = &monitor.ServiceStatus{
		Indicator:   "none",
		Description: "All Systems Operational",
		Components:  []monitor.Component{{Name: "Claude API", Status: "operational"}},
	}
	m.settle()
	return m
}

func TestViewFitsTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{160, 50}, {100, 40}, {96, 30}, {80, 40}, {60, 20}, {40, 12}} {
		view := testModel(size[0], size[1]).View()
		if got := lipgloss.Width(view); got > size[0] {
			t.Errorf("%dx%d: view is %d wide", size[0], size[1], got)
		}
		if got := lipgloss.Height(view); got > size[1] {
			t.Errorf("%dx%d: view is %d tall", size[0], size[1], got)
		}
	}
}

func TestViewShowsTheData(t *testing.T) {
	view := testModel(120, 0).View()
	for _, want := range []string{"42%", "96%", "1h30", "you're cooked", "Claude API", "opus-5-5", "3.0k", "5.0M", "2 answers", "$0.25", "claude 2.1.302", "project"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in:\n%s", want, view)
		}
	}
}

func TestViewExplainsMissingData(t *testing.T) {
	m := testModel(120, 0)
	m.local = monitor.Local{}
	m.status = nil
	m.statusErr = errors.New("offline")
	view := m.View()
	for _, want := range []string{"ask monitor statusline --help", "Status page unreachable", "Start claude"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in:\n%s", want, view)
		}
	}
}

func TestAnIncidentOutranksTheOtherVibes(t *testing.T) {
	m := testModel(120, 0)
	m.status.Indicator = "major"
	if !strings.Contains(m.vibe(), "having a moment") {
		t.Fatalf("vibe = %q", m.vibe())
	}
}

func TestSceneStaysInsideTheScreenWhileDriving(t *testing.T) {
	m := testModel(60, 0)
	for x := range 200 {
		m.catX = float64(x)
		scene := m.scene(60)
		if got := lipgloss.Width(scene); got > 60 {
			t.Fatalf("catX %d: scene is %d wide:\n%s", x, got, scene)
		}
		if got := lipgloss.Height(scene); got != sceneHeight {
			t.Fatalf("catX %d: scene is %d tall", x, got)
		}
		if !utf8.ValidString(scene) {
			t.Fatalf("catX %d: broken characters in the scene", x)
		}
		if got := lipgloss.Width(m.road(60)); got != 60 {
			t.Fatalf("catX %d: road is %d wide", x, got)
		}
	}
}

func TestCatDrivesOnlyWhenThereIsRoom(t *testing.T) {
	if view := testModel(120, 0).View(); !strings.Contains(view, "/===========\\") {
		t.Errorf("no cat in a tall terminal:\n%s", view)
	}
	if view := testModel(120, 15).View(); strings.Contains(view, "/===========\\") {
		t.Errorf("cat drawn in a short terminal:\n%s", view)
	}
}

func TestCatFollowsActivity(t *testing.T) {
	m := testModel(120, 0)
	busy := m.catSpeed()
	m.local.Sessions = []monitor.Session{{Status: "idle"}}
	idle := m.catSpeed()
	m.local.Sessions = nil
	if empty := m.catSpeed(); !(busy > idle && idle > empty) {
		t.Fatalf("busy %v, idle %v, empty %v", busy, idle, empty)
	}
}

func TestGaugeHandlesAnyPercent(t *testing.T) {
	for _, percent := range []float64{-5, 0, 50, 100, 140} {
		if got := lipgloss.Width(gauge(10, percent, 7)); got != 10 {
			t.Errorf("gauge(%v) is %d wide", percent, got)
		}
	}
}

func TestProgramAnimatesLoadsAndQuits(t *testing.T) {
	collector := &monitor.Collector{ClaudeDir: t.TempDir(), AskDir: t.TempDir(), Now: time.Now}
	// A cancelled context keeps the online check off the network.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input, writer := io.Pipe()
	defer writer.Close()
	var output bytes.Buffer
	program := tea.NewProgram(newMonitorModel(ctx, collector, 10*time.Millisecond),
		tea.WithInput(input), tea.WithOutput(&output))
	go func() {
		program.Send(tea.WindowSizeMsg{Width: 100, Height: 40})
		time.Sleep(400 * time.Millisecond)
		writer.Write([]byte("q"))
	}()
	final, err := program.Run()
	if err != nil {
		t.Fatal(err)
	}
	m := final.(monitorModel)
	if m.frame == 0 || !m.loaded || m.statusErr == nil {
		t.Fatalf("frame %d, loaded %v, status error %v", m.frame, m.loaded, m.statusErr)
	}
	if !strings.Contains(output.String(), "PLAN LIMITS") {
		t.Fatalf("nothing drawn: %q", output.String())
	}
}

func TestStatusLineSavesLimits(t *testing.T) {
	f := newFixture(t)
	f.stdin = fmt.Sprintf(`{"model":{"display_name":"Opus"},"rate_limits":{"five_hour":{"used_percentage":42,"resets_at":%d}}}`, time.Now().Add(time.Hour).Unix())
	res := f.ask("monitor", "statusline")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if got := strings.TrimSpace(res.stdout); got != "Opus · 5h 42%" {
		t.Fatalf("stdout = %q", got)
	}
	if _, err := os.Stat(filepath.Join(f.configDir, "usage.json")); err != nil {
		t.Fatal(err)
	}
}

func TestMonitorRejectsABadInterval(t *testing.T) {
	res := newFixture(t).ask("monitor", "--interval", "0s")
	if res.code == 0 || !strings.Contains(res.stderr, "interval must be positive") {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
}
