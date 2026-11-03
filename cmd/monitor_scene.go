package cmd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	catWidth      = 14
	trailLength   = 28
	sceneHeight   = 5
	minSceneWidth = 40
)

// catCar is the cat at the wheel. Eyes and wheels are filled in on each frame.
var catCar = []string{
	`   /\_/\    _ `,
	`  ( %s )__/ |`,
	` /===========\`,
	` '-(%c)---(%c)-'`,
}

var (
	skylineCells = []rune("▂▂▅▂▃▇▂▂▄▂▆▃▂")
	wheelFrames  = []rune("@*o*")
	rainbowStops = [][3]float64{{255, 0, 60}, {255, 140, 0}, {255, 230, 0}, {0, 230, 90}, {0, 170, 255}, {170, 60, 255}}

	skylineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5A189A"))
	roadStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF2E97"))
)

// catSpeed follows the real activity: the cat floors it while Claude works.
func (m monitorModel) catSpeed() float64 {
	speed := 0.15
	for _, session := range m.local.Sessions {
		if session.Status == "busy" {
			return 1
		}
		speed = 0.4
	}
	return speed
}

// scene draws the cat driving through the city with a rainbow behind it.
func (m monitorModel) scene(width int) string {
	// The car comes in from the left edge and leaves by the right one.
	x := int(m.catX)%(width+catWidth) - catWidth
	lines := make([]string, 0, sceneHeight)
	for row, part := range m.catSprite() {
		lines = append(lines, m.sceneRow(row, part, x, width))
	}
	return strings.Join(append(lines, m.road(width)), "\n")
}

func (m monitorModel) catSprite() []string {
	eyes := "o.o"
	if len(m.local.Sessions) == 0 {
		eyes = "-.-"
	} else if m.frame%50 >= 48 {
		eyes = "-.-"
	}
	wheel := wheelFrames[int(m.catX)%len(wheelFrames)]
	return []string{
		catCar[0],
		strings.Replace(catCar[1], "%s", eyes, 1),
		catCar[2],
		strings.NewReplacer("%c", string(wheel)).Replace(catCar[3]),
	}
}

func (m monitorModel) sceneRow(row int, sprite string, x, width int) string {
	var b strings.Builder
	for i := range max(x, 0) {
		b.WriteString(m.backdropCell(row, i, x))
	}
	cells := []rune(sprite)
	for i, cell := range cells {
		if at := x + i; at >= 0 && at < width {
			b.WriteString(catStyle(row, i, m.phase()).Render(string(cell)))
		}
	}
	if row == 0 {
		for i := x + len(cells); i < width; i++ {
			b.WriteString(m.backdropCell(row, i, x))
		}
	}
	return b.String()
}

// backdropCell is the skyline on the top row, and the rainbow behind the car on
// the others.
func (m monitorModel) backdropCell(row, i, x int) string {
	if row == 0 {
		cell := skylineCells[(i+int(m.catX)/4)%len(skylineCells)]
		return skylineStyle.Render(string(cell))
	}
	if x-i > trailLength {
		return " "
	}
	color := blendAt(rainbowStops, float64(row)*0.16+float64(x-i)*0.012-m.phase())
	return lipgloss.NewStyle().Foreground(color).Render("━")
}

func catStyle(row, cell int, phase float64) lipgloss.Style {
	switch row {
	case 0, 1:
		return boldStyle
	case 2:
		return lipgloss.NewStyle().Foreground(neonAt(float64(cell)*0.03 + phase)).Bold(true)
	}
	return cyanStyle
}

// road scrolls its dashes backwards, so the car looks like it is moving.
func (m monitorModel) road(width int) string {
	dash := []rune("━━━  ")
	cells := []rune(strings.Repeat(string(dash), width/len(dash)+2))
	offset := len(dash) - int(m.catX)%len(dash)
	return roadStyle.Render(string(cells[offset%len(dash):][:width]))
}
