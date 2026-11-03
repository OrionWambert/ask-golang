package render

import "strings"

// CodeBlocks returns the content of each fenced code block in text. An
// unterminated block runs to the end of the text.
func CodeBlocks(text string) []string {
	var blocks, current []string
	var marker string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimLeft(line, " ")
		fence := len(line)-len(trimmed) <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"))
		switch {
		case marker == "" && fence:
			marker = trimmed[:fenceLength(trimmed)]
			current = nil
		case marker != "" && fence && strings.HasPrefix(trimmed, marker) && strings.Trim(trimmed, marker[:1]+" ") == "":
			blocks = append(blocks, strings.Join(current, "\n"))
			marker = ""
		case marker != "":
			current = append(current, line)
		}
	}
	if marker != "" {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}

func fenceLength(line string) int {
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	return n
}
