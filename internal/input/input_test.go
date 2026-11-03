package input

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMessageOnly(t *testing.T) {
	got, err := Build(Source{Message: "  hello  "})
	if err != nil || got != "hello" {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestBuildStdinOnlyIsThePrompt(t *testing.T) {
	got, err := Build(Source{Stdin: strings.NewReader("from pipe\n")})
	if err != nil || got != "from pipe" {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestBuildPutsContextBeforeMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("file body"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Build(Source{Message: "explain", Files: []string{path}, Stdin: strings.NewReader("diff")})
	if err != nil {
		t.Fatal(err)
	}
	fileAt, stdinAt, messageAt := strings.Index(got, "file body"), strings.Index(got, "<stdin>\ndiff"), strings.Index(got, "explain")
	if fileAt < 0 || stdinAt < fileAt || messageAt < stdinAt {
		t.Fatalf("wrong order: %q", got)
	}
}

func TestBuildEmpty(t *testing.T) {
	_, err := Build(Source{Message: "  ", Stdin: strings.NewReader("\n")})
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildRejectsBinaryAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(binary, []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{binary, filepath.Join(dir, "missing.txt")} {
		if _, err := Build(Source{Message: "x", Files: []string{path}}); err == nil {
			t.Errorf("no error for %s", path)
		}
	}
}
