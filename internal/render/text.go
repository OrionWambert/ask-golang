package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"oriowambert.com/ask-golang/internal/claude"
)

type textRenderer struct{}

func (textRenderer) Render(w io.Writer, resp *claude.Response) error {
	if len(resp.Structured) > 0 {
		var indented bytes.Buffer
		if err := json.Indent(&indented, resp.Structured, "", "  "); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, indented.String())
		return err
	}
	_, err := fmt.Fprintln(w, strings.TrimRight(resp.Text, "\n"))
	return err
}
