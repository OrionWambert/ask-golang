package monitor

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// retention is how far back token use is counted. It covers the seven calendar
// days on the dashboard.
const retention = 8 * 24 * time.Hour

var (
	assistantMarker = []byte(`"type":"assistant"`)
	usageMarker     = []byte(`"usage"`)
)

// Tokens counts the tokens of one or more requests.
type Tokens struct {
	Input      int64
	Output     int64
	CacheWrite int64
	CacheRead  int64
}

// Used leaves out cache reads, which would otherwise dwarf everything else.
func (t Tokens) Used() int64 {
	return t.Input + t.Output + t.CacheWrite
}

func (t *Tokens) add(other Tokens) {
	t.Input += other.Input
	t.Output += other.Output
	t.CacheWrite += other.CacheWrite
	t.CacheRead += other.CacheRead
}

// ModelUsage is one model's share of the last seven days.
type ModelUsage struct {
	Model string
	Used  int64
}

// Usage is the token use read from Claude Code's transcripts.
type Usage struct {
	Last5h Tokens
	Today  Tokens
	Week   Tokens
	// Daily holds the last seven calendar days, ending with today.
	Daily [7]int64
	// Hourly holds the last 24 hours, ending with the current one.
	Hourly    [24]int64
	Models    []ModelUsage
	BySession map[string]int64
}

type request struct {
	time    time.Time
	model   string
	session string
	tokens  Tokens
}

// transcriptLine holds the fields used from a transcript entry. The format is
// internal to Claude Code and can change, so anything else is ignored.
type transcriptLine struct {
	Type      string    `json:"type"`
	RequestID string    `json:"requestId"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input      int64 `json:"input_tokens"`
			Output     int64 `json:"output_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// tailer reads transcripts incrementally. A request can span several lines and
// be copied into another file on resume, so requests are kept by id.
type tailer struct {
	offsets  map[string]int64
	requests map[string]request
}

func (t *tailer) scan(root string, now time.Time) Usage {
	if t.offsets == nil {
		t.offsets = map[string]int64{}
		t.requests = map[string]request{}
	}
	cutoff := now.Add(-retention)
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(cutoff) {
			return nil
		}
		t.readFile(path, info.Size(), cutoff)
		return nil
	})
	for id, req := range t.requests {
		if req.time.Before(cutoff) {
			delete(t.requests, id)
		}
	}
	return summarize(t.requests, now)
}

func (t *tailer) readFile(path string, size int64, cutoff time.Time) {
	offset := t.offsets[path]
	if size == offset {
		return
	}
	if size < offset {
		offset = 0
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return
	}
	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// A line without its newline is still being written: read it next time.
			break
		}
		offset += int64(len(line))
		t.readLine(line, cutoff)
	}
	t.offsets[path] = offset
}

func (t *tailer) readLine(line []byte, cutoff time.Time) {
	if !bytes.Contains(line, assistantMarker) || !bytes.Contains(line, usageMarker) {
		return
	}
	var entry transcriptLine
	if json.Unmarshal(line, &entry) != nil || entry.Type != "assistant" || entry.Message.Usage == nil {
		return
	}
	id := entry.RequestID
	if id == "" {
		id = entry.Message.ID
	}
	if id == "" || entry.Timestamp.Before(cutoff) || entry.Message.Model == "<synthetic>" {
		return
	}
	usage := entry.Message.Usage
	t.requests[id] = request{
		time:    entry.Timestamp,
		model:   entry.Message.Model,
		session: entry.SessionID,
		tokens: Tokens{
			Input:      usage.Input,
			Output:     usage.Output,
			CacheWrite: usage.CacheWrite,
			CacheRead:  usage.CacheRead,
		},
	}
}

func summarize(requests map[string]request, now time.Time) Usage {
	usage := Usage{BySession: map[string]int64{}}
	today := startOfDay(now)
	week := today.AddDate(0, 0, -6)
	byModel := map[string]int64{}
	for _, req := range requests {
		used := req.tokens.Used()
		usage.BySession[req.session] += used
		age := now.Sub(req.time)
		if age < 5*time.Hour {
			usage.Last5h.add(req.tokens)
		}
		if age >= 0 && age < 24*time.Hour {
			usage.Hourly[23-int(age/time.Hour)] += used
		}
		local := req.time.In(now.Location())
		if local.Before(week) {
			continue
		}
		usage.Week.add(req.tokens)
		byModel[req.model] += used
		if !local.Before(today) {
			usage.Today.add(req.tokens)
		}
		// Rounding keeps the day count right across a daylight saving change.
		days := int(today.Sub(startOfDay(local)).Hours()/24 + 0.5)
		if days >= 0 && days < len(usage.Daily) {
			usage.Daily[len(usage.Daily)-1-days] += used
		}
	}
	for model, used := range byModel {
		usage.Models = append(usage.Models, ModelUsage{Model: model, Used: used})
	}
	slices.SortFunc(usage.Models, func(a, b ModelUsage) int {
		return cmp.Or(cmp.Compare(b.Used, a.Used), strings.Compare(a.Model, b.Model))
	})
	return usage
}
