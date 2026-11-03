package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const personaPrefix = "persona."

// Config holds the user's saved defaults.
type Config struct {
	Model        string            `json:"model,omitempty"`
	Format       string            `json:"format,omitempty"`
	SystemPrompt string            `json:"system_prompt,omitempty"`
	BudgetUSD    float64           `json:"budget,omitempty"`
	History      *bool             `json:"history,omitempty"`
	Tools        bool              `json:"tools,omitempty"`
	Personas     map[string]string `json:"personas,omitempty"`
}

// Dir returns the folder holding ask's data. ASK_CONFIG_DIR overrides it.
func Dir() (string, error) {
	if dir := os.Getenv("ASK_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ask"), nil
}

// Path returns the location of the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file. A missing file gives an empty config.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// HistoryEnabled reports whether answers are saved. It defaults to true.
func (c *Config) HistoryEnabled() bool {
	return c.History == nil || *c.History
}

// Get returns the value of key, and whether it is set.
func (c *Config) Get(key string) (string, bool, error) {
	if name, ok := strings.CutPrefix(key, personaPrefix); ok {
		value, found := c.Personas[name]
		return value, found, nil
	}
	switch key {
	case "model":
		return c.Model, c.Model != "", nil
	case "format":
		return c.Format, c.Format != "", nil
	case "system_prompt":
		return c.SystemPrompt, c.SystemPrompt != "", nil
	case "budget":
		return strconv.FormatFloat(c.BudgetUSD, 'f', -1, 64), c.BudgetUSD > 0, nil
	case "history":
		return strconv.FormatBool(c.HistoryEnabled()), c.History != nil, nil
	case "tools":
		return strconv.FormatBool(c.Tools), c.Tools, nil
	}
	return "", false, unknownKey(key)
}

// Set assigns value to key.
func (c *Config) Set(key, value string) error {
	if name, ok := strings.CutPrefix(key, personaPrefix); ok {
		if name == "" {
			return unknownKey(key)
		}
		if c.Personas == nil {
			c.Personas = map[string]string{}
		}
		c.Personas[name] = value
		return nil
	}
	switch key {
	case "model":
		c.Model = value
	case "format":
		c.Format = value
	case "system_prompt":
		c.SystemPrompt = value
	case "budget":
		budget, err := strconv.ParseFloat(value, 64)
		if err != nil || budget <= 0 {
			return fmt.Errorf("budget must be a positive number, got %q", value)
		}
		c.BudgetUSD = budget
	case "history":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("history must be true or false, got %q", value)
		}
		c.History = &enabled
	case "tools":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("tools must be true or false, got %q", value)
		}
		c.Tools = enabled
	default:
		return unknownKey(key)
	}
	return nil
}

// Unset clears key.
func (c *Config) Unset(key string) error {
	if name, ok := strings.CutPrefix(key, personaPrefix); ok {
		delete(c.Personas, name)
		return nil
	}
	switch key {
	case "model":
		c.Model = ""
	case "format":
		c.Format = ""
	case "system_prompt":
		c.SystemPrompt = ""
	case "budget":
		c.BudgetUSD = 0
	case "history":
		c.History = nil
	case "tools":
		c.Tools = false
	default:
		return unknownKey(key)
	}
	return nil
}

func unknownKey(key string) error {
	return fmt.Errorf("unknown config key %q: use model, format, system_prompt, budget, history, tools or persona.<name>", key)
}
