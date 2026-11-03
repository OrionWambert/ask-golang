package render

import (
	"slices"
	"testing"
)

func TestCodeBlocks(t *testing.T) {
	tests := []struct {
		name, in string
		want     []string
	}{
		{"none", "just text", nil},
		{"one with language", "before\n```go\nx := 1\ny := 2\n```\nafter", []string{"x := 1\ny := 2"}},
		{"two", "```\na\n```\ntext\n~~~\nb\n~~~", []string{"a", "b"}},
		{"crlf", "```\r\na\r\n```\r\n", []string{"a"}},
		{"unterminated", "```\na\nb", []string{"a\nb"}},
		{"longer fence keeps inner fence", "````\n```\nx\n```\n````", []string{"```\nx\n```"}},
		{"other marker stays content", "```\n~~~\n```", []string{"~~~"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CodeBlocks(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
