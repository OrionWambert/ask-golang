package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)

func assistantLine(request, session, model string, at time.Time, output int) string {
	return fmt.Sprintf(`{"type":"assistant","requestId":%q,"sessionId":%q,"timestamp":%q,"message":{"id":"m","model":%q,"usage":{"input_tokens":10,"output_tokens":%d,"cache_creation_input_tokens":5,"cache_read_input_tokens":100}}}`+"\n",
		request, session, at.Format(time.RFC3339), model, output)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestScanCountsEachRequestOnce(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	at := now.Add(-time.Minute)
	content := assistantLine("r1", "s1", "opus", at, 20) +
		assistantLine("r1", "s1", "opus", at, 40) +
		`{"type":"user","message":"the word \"usage\" appears here"}` + "\n" +
		"not json\n"
	writeFile(t, filepath.Join(root, "p", "a.jsonl"), content)
	writeFile(t, filepath.Join(root, "p", "copy.jsonl"), assistantLine("r1", "s1", "opus", at, 40))

	var tail tailer
	usage := tail.scan(root, now)
	want := Tokens{Input: 10, Output: 40, CacheWrite: 5, CacheRead: 100}
	if usage.Last5h != want || usage.Week != want {
		t.Fatalf("got %+v, want %+v", usage, want)
	}
	if usage.BySession["s1"] != 55 || usage.Hourly[23] != 55 {
		t.Fatalf("unexpected buckets: %+v", usage)
	}
}

func TestSummarizeWindows(t *testing.T) {
	requests := map[string]request{
		"now":       {time: testNow.Add(-time.Minute), model: "opus", tokens: Tokens{Output: 1}},
		"today":     {time: testNow.Add(-10 * time.Hour), model: "haiku", tokens: Tokens{Output: 2}},
		"yesterday": {time: testNow.Add(-24 * time.Hour), model: "haiku", tokens: Tokens{Output: 4}},
		"lastweek":  {time: testNow.Add(-7 * 24 * time.Hour), model: "haiku", tokens: Tokens{Output: 8}},
	}
	usage := summarize(requests, testNow)
	if usage.Last5h.Used() != 1 || usage.Today.Used() != 3 || usage.Week.Used() != 7 {
		t.Fatalf("wrong windows: %+v", usage)
	}
	if usage.Daily[6] != 3 || usage.Daily[5] != 4 {
		t.Fatalf("wrong days: %v", usage.Daily)
	}
	if usage.Hourly[23] != 1 || usage.Hourly[13] != 2 {
		t.Fatalf("wrong hours: %v", usage.Hourly)
	}
	if usage.Models[0].Model != "haiku" || usage.Models[0].Used != 6 {
		t.Fatalf("wrong models: %+v", usage.Models)
	}
}

func TestScanReadsOnlyNewCompleteLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.jsonl")
	now := time.Now()
	first := assistantLine("r1", "s", "opus", now, 1)
	second := assistantLine("r2", "s", "opus", now, 1)
	writeFile(t, path, first+second[:20])

	var tail tailer
	if got := tail.scan(root, now); got.Last5h.Output != 1 {
		t.Fatalf("partial line counted: %+v", got.Last5h)
	}
	writeFile(t, path, first+second)
	if got := tail.scan(root, now); got.Last5h.Output != 2 {
		t.Fatalf("new line missed: %+v", got.Last5h)
	}
	if got := tail.scan(root, now); got.Last5h.Output != 2 {
		t.Fatalf("lines counted twice: %+v", got.Last5h)
	}
}

func TestReadSessionsSkipsDeadProcesses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alive.json"), fmt.Sprintf(`{"pid":%d,"sessionId":"s1","cwd":"/x/proj","status":"busy","startedAt":1000}`, os.Getpid()))
	writeFile(t, filepath.Join(dir, "dead.json"), `{"pid":0,"sessionId":"s2"}`)
	writeFile(t, filepath.Join(dir, "broken.json"), `nope`)
	writeFile(t, filepath.Join(dir, "1.abc.key"), `secret`)
	sessions := readSessions(dir, map[string]int64{"s1": 7})
	if len(sessions) != 1 || sessions[0].ID != "s1" || sessions[0].Folder != "proj" || sessions[0].Tokens != 7 {
		t.Fatalf("got %+v", sessions)
	}
}

func TestRecordStatusLine(t *testing.T) {
	dir := t.TempDir()
	future := testNow.Add(time.Hour).Unix()
	input := fmt.Sprintf(`{"model":{"display_name":"Opus"},"rate_limits":{"five_hour":{"used_percentage":42.4,"resets_at":%d},"seven_day":{"used_percentage":18,"resets_at":%d}}}`, future, future)
	if got := RecordStatusLine(strings.NewReader(input), dir, testNow); got != "Opus · 5h 42% · 7d 18%" {
		t.Fatalf("line = %q", got)
	}

	// A later payload with one window or none must not lose what was saved.
	input = fmt.Sprintf(`{"rate_limits":{"five_hour":{"used_percentage":50,"resets_at":%d}}}`, future)
	if got := RecordStatusLine(strings.NewReader(input), dir, testNow); got != "5h 50% · 7d 18%" {
		t.Fatalf("line = %q", got)
	}
	if got := RecordStatusLine(strings.NewReader(`{"model":{"display_name":"Opus"}}`), dir, testNow); got != "Opus · 5h 50% · 7d 18%" {
		t.Fatalf("line = %q", got)
	}

	if limits := LoadRateLimits(dir, testNow.Add(2*time.Hour)); limits.FiveHour != nil || limits.SevenDay != nil {
		t.Fatalf("expired windows kept: %+v", limits)
	}
	if got := RecordStatusLine(strings.NewReader(`not json`), t.TempDir(), testNow); got != "ask monitor" {
		t.Fatalf("line = %q", got)
	}
}

func TestFetchStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":{"indicator":"minor","description":"Partial outage"},
			"components":[{"name":"API","status":"degraded_performance"},{"name":"Group","group":true,"status":"operational"},{"name":"Hidden","only_show_if_degraded":true,"status":"operational"}],
			"incidents":[{"name":"Elevated errors"}]}`)
	}))
	defer server.Close()
	status, err := FetchStatus(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if status.Indicator != "minor" || len(status.Components) != 1 || status.Components[0].Name != "API" || status.Incidents[0] != "Elevated errors" {
		t.Fatalf("got %+v", status)
	}
}

func TestFetchStatusReportsHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := FetchStatus(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("expected an error")
	}
}
