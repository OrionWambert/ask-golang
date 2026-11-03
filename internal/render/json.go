package render

import (
	"encoding/json"
	"io"

	"github.com/OrionWambert/ask-golang/internal/claude"
)

type jsonRenderer struct{}

func (jsonRenderer) Render(w io.Writer, resp *claude.Response) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
