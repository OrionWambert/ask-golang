package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// StatusURL is Anthropic's public status page API. It needs no login.
const StatusURL = "https://status.claude.com/api/v2/summary.json"

// Component is one service on the status page.
type Component struct {
	Name   string
	Status string
}

// ServiceStatus is the public status of Claude's services.
type ServiceStatus struct {
	Indicator   string
	Description string
	Components  []Component
	Incidents   []string
}

// FetchStatus reads the status page at url.
func FetchStatus(ctx context.Context, client *http.Client, url string) (*ServiceStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status page answered %s", resp.Status)
	}
	var summary struct {
		Status struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		} `json:"status"`
		Components []struct {
			Name               string `json:"name"`
			Status             string `json:"status"`
			Group              bool   `json:"group"`
			OnlyShowIfDegraded bool   `json:"only_show_if_degraded"`
		} `json:"components"`
		Incidents []struct {
			Name string `json:"name"`
		} `json:"incidents"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&summary); err != nil {
		return nil, fmt.Errorf("read status page: %w", err)
	}
	status := &ServiceStatus{Indicator: summary.Status.Indicator, Description: summary.Status.Description}
	for _, component := range summary.Components {
		if component.Group || (component.OnlyShowIfDegraded && component.Status == "operational") {
			continue
		}
		status.Components = append(status.Components, Component{Name: component.Name, Status: component.Status})
	}
	for _, incident := range summary.Incidents {
		status.Incidents = append(status.Incidents, incident.Name)
	}
	return status, nil
}

// ClaudeVersion returns the version of the local claude binary, or "" if it
// can't be run.
func ClaudeVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "claude", "--version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
