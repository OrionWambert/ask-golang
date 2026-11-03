package render

import (
	"fmt"
	"io"

	"oriowambert.com/ask-golang/internal/claude"
)

// Supported output formats.
const (
	Text = "text"
	JSON = "json"
)

// Options tunes how an answer is displayed.
type Options struct {
	Styled bool
	Width  int
}

// Renderer writes an answer in one output format.
type Renderer interface {
	Render(w io.Writer, resp *claude.Response) error
}

// New returns the renderer for format.
func New(format string, opts Options) (Renderer, error) {
	switch format {
	case Text:
		return textRenderer{}, nil
	case JSON:
		return jsonRenderer{}, nil
	}
	return nil, fmt.Errorf("unknown format %q: use text or json", format)
}
