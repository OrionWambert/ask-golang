package render

import (
	"fmt"
	"io"

	"github.com/charmbracelet/glamour"

	"oriowambert.com/ask-golang/internal/claude"
)

type markdownRenderer struct {
	opts Options
}

// Render styles the answer for a terminal. Without styling it writes the raw
// Markdown, which keeps pipes and files clean.
func (m markdownRenderer) Render(w io.Writer, resp *claude.Response) error {
	if !m.opts.Styled || len(resp.Structured) > 0 {
		return textRenderer{}.Render(w, resp)
	}
	styler, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(m.opts.Width))
	if err != nil {
		return err
	}
	styled, err := styler.Render(resp.Text)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, styled)
	return err
}
