package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesEmptyConfig(t *testing.T) {
	t.Setenv("ASK_CONFIG_DIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "" || !cfg.HistoryEnabled() {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("ASK_CONFIG_DIR", filepath.Join(t.TempDir(), "nested"))
	cfg := &Config{}
	for key, value := range map[string]string{
		"model":            "haiku",
		"format":           "markdown",
		"system_prompt":    "be brief",
		"budget":           "0.5",
		"history":          "false",
		"tools":            "true",
		"persona.reviewer": "review code",
	} {
		if err := cfg.Set(key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "haiku" || got.Format != "markdown" || got.SystemPrompt != "be brief" || got.BudgetUSD != 0.5 {
		t.Errorf("unexpected config: %+v", got)
	}
	if got.HistoryEnabled() {
		t.Error("history should be off")
	}
	if !got.Tools {
		t.Error("tools should be on")
	}
	if value, ok, _ := got.Get("persona.reviewer"); !ok || value != "review code" {
		t.Errorf("persona = %q, %v", value, ok)
	}
}

func TestSetRejectsBadValues(t *testing.T) {
	cfg := &Config{}
	for _, pair := range [][2]string{{"budget", "abc"}, {"budget", "-1"}, {"history", "maybe"}, {"tools", "maybe"}, {"nope", "x"}, {"persona.", "x"}} {
		if err := cfg.Set(pair[0], pair[1]); err == nil {
			t.Errorf("no error for %v", pair)
		}
	}
}

func TestUnset(t *testing.T) {
	cfg := &Config{}
	_ = cfg.Set("model", "haiku")
	_ = cfg.Set("persona.a", "b")
	for _, key := range []string{"model", "persona.a"} {
		if err := cfg.Unset(key); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := cfg.Get(key); ok {
			t.Errorf("%s still set", key)
		}
	}
}

func TestLoadReportsInvalidFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ASK_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected an error")
	}
}
