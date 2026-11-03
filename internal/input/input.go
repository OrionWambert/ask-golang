package input

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// ErrEmpty means there is nothing to send to claude.
var ErrEmpty = errors.New("nothing to ask: give a message, a file or piped input")

// Source holds everything the user gave for one question.
type Source struct {
	Message string
	Files   []string
	Stdin   io.Reader
}

// Build merges the source into one prompt, with the context before the message.
func Build(s Source) (string, error) {
	var parts []string
	for _, path := range s.Files {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			return "", fmt.Errorf("read file: %s looks like a binary file", path)
		}
		parts = append(parts, fmt.Sprintf("<file path=%q>\n%s\n</file>", path, content))
	}

	message := strings.TrimSpace(s.Message)
	if s.Stdin != nil {
		content, err := io.ReadAll(s.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if piped := strings.TrimSpace(string(content)); piped != "" {
			if message == "" && len(parts) == 0 {
				return piped, nil
			}
			parts = append(parts, "<stdin>\n"+piped+"\n</stdin>")
		}
	}

	if message != "" {
		parts = append(parts, message)
	}
	if len(parts) == 0 {
		return "", ErrEmpty
	}
	return strings.Join(parts, "\n\n"), nil
}

// IsPiped reports whether f is a pipe or a redirected file, not a console.
// Git Bash terminals look like pipes on Windows, so they are ruled out first.
func IsPiped(f *os.File) bool {
	if isatty.IsCygwinTerminal(f.Fd()) {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice == 0
}
