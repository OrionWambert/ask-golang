package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/OrionWambert/ask-golang/internal/config"
	"github.com/OrionWambert/ask-golang/internal/monitor"
)

const (
	onlineInterval = time.Minute
	onlineTimeout  = 5 * time.Second
)

var (
	monitorOnce     bool
	monitorInterval time.Duration
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Watch Claude Code on a live dashboard",
	Long: `Shows running Claude Code sessions, token use, plan limits, ask's spending and
the public Claude status. Press q to quit and r to refresh.

Plan limits need one setup step: see ask monitor statusline --help.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if monitorInterval <= 0 {
			return errors.New("interval must be positive")
		}
		dir, err := config.Dir()
		if err != nil {
			return err
		}
		collector, err := monitor.NewCollector(dir)
		if err != nil {
			return err
		}
		if monitorOnce || !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
			return printMonitorOnce(cmd.Context(), collector)
		}
		model := newMonitorModel(cmd.Context(), collector, monitorInterval)
		_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(cmd.Context())).Run()
		if errors.Is(err, tea.ErrProgramKilled) {
			return context.Canceled
		}
		return err
	},
}

var monitorStatusLineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Feed plan limits to ask monitor from Claude Code's status line",
	Long: `Claude Code only shares your plan limits with status line commands, so this
one saves them for ask monitor and prints a short status line. They appear on
Pro, Max and Ultra plans, once a session has had its first response.

To turn it on, add this to ~/.claude/settings.json. It replaces any status line
you already have:

  "statusLine": { "type": "command", "command": "ask monitor statusline" }`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.Dir()
		if err != nil {
			return err
		}
		fmt.Println(monitor.RecordStatusLine(os.Stdin, dir, time.Now()))
		return nil
	},
}

func init() {
	monitorCmd.Flags().BoolVar(&monitorOnce, "once", false, "print one frame and exit")
	monitorCmd.Flags().DurationVar(&monitorInterval, "interval", 2*time.Second, "how often to reread local data")
	monitorCmd.AddCommand(monitorStatusLineCmd)
	rootCmd.AddCommand(monitorCmd)
}

func printMonitorOnce(ctx context.Context, collector *monitor.Collector) error {
	model := newMonitorModel(ctx, collector, monitorInterval)
	model.width = 100
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		model.width = width
	}
	updated, _ := model.Update(model.loadLocal()())
	updated, _ = updated.Update(model.loadOnline()())
	final := updated.(monitorModel)
	final.settle()
	fmt.Println(final.View())
	return nil
}

func fetchOnline(ctx context.Context) (*monitor.ServiceStatus, string, error) {
	ctx, cancel := context.WithTimeout(ctx, onlineTimeout)
	defer cancel()
	version := monitor.ClaudeVersion(ctx)
	status, err := monitor.FetchStatus(ctx, http.DefaultClient, monitor.StatusURL)
	return status, version, err
}
