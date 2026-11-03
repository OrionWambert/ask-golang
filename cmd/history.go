package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"oriowambert.com/ask-golang/internal/claude"
	"oriowambert.com/ask-golang/internal/config"
	"oriowambert.com/ask-golang/internal/history"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List saved answers, newest first",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := historyStore()
		if err != nil {
			return err
		}
		entries, err := store.List(historyLimit)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			fmt.Printf("%s  %s\n", entry.ID, preview(entry.Prompt))
		}
		return nil
	},
}

var historyShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print a saved question and its answer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := historyStore()
		if err != nil {
			return err
		}
		entry, err := store.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("> %s\n\n%s\n", entry.Prompt, entry.Answer)
		if entry.SessionID != "" {
			fmt.Fprintf(os.Stderr, "\nresume with: ask --resume %s\n", entry.SessionID)
		}
		return nil
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all saved answers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := historyStore()
		if err != nil {
			return err
		}
		removed, err := store.Clear()
		if err != nil {
			return err
		}
		fmt.Printf("deleted %d saved answers\n", removed)
		return nil
	},
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "how many answers to list (0 for all)")
	historyCmd.AddCommand(historyShowCmd, historyClearCmd)
	rootCmd.AddCommand(historyCmd)
}

func historyStore() (history.Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return history.Store{}, err
	}
	return history.Store{Dir: filepath.Join(dir, "history")}, nil
}

// saveHistory only warns on failure: losing a history entry must not fail the run.
func saveHistory(cfg *config.Config, message string, resp *claude.Response) {
	if flags.noHistory || !cfg.HistoryEnabled() {
		return
	}
	if message == "" {
		message = "(piped or file input)"
	}
	store, err := historyStore()
	if err == nil {
		err = store.Save(&history.Entry{
			Prompt:    message,
			Answer:    resp.Text,
			SessionID: resp.SessionID,
			Model:     resp.Model,
			CostUSD:   resp.CostUSD,
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ask: could not save to history:", err)
	}
}

func preview(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if runes := []rune(line); len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return line
}
