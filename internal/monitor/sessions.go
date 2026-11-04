package monitor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
)

// Session is a running Claude Code process.
type Session struct {
	ID      string
	Name    string
	Folder  string
	Status  string
	Kind    string
	Started time.Time
	Tokens  int64
}

// readSessions lists the sessions in Claude Code's sessions folder whose process
// is still alive. A crashed session can leave its file behind.
func readSessions(dir string, tokens map[string]int64) []Session {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var sessions []Session
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			continue
		}
		var raw struct {
			PID       int    `json:"pid"`
			SessionID string `json:"sessionId"`
			Cwd       string `json:"cwd"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Kind      string `json:"kind"`
			StartedAt int64  `json:"startedAt"`
		}
		if json.Unmarshal(data, &raw) != nil || !processAlive(raw.PID) {
			continue
		}
		sessions = append(sessions, Session{
			ID:      raw.SessionID,
			Name:    raw.Name,
			Folder:  filepath.Base(raw.Cwd),
			Status:  raw.Status,
			Kind:    raw.Kind,
			Started: time.UnixMilli(raw.StartedAt),
			Tokens:  tokens[raw.SessionID],
		})
	}
	slices.SortFunc(sessions, func(a, b Session) int {
		return b.Started.Compare(a.Started)
	})
	return sessions
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer process.Release()
	// On Windows FindProcess already fails for a process that is gone, and
	// signals other than kill are not supported.
	if runtime.GOOS == "windows" {
		return true
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || err == syscall.EPERM
}
