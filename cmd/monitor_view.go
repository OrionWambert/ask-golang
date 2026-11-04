package cmd

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/OrionWambert/ask-golang/internal/monitor"
)

const (
	frameInterval = 100 * time.Millisecond
	twoColumnsAt  = 96
	maxSessions   = 5
	maxComponents = 5
	maxModels     = 4
)

var (
	neonStops  = [][3]float64{{255, 46, 151}, {255, 0, 229}, {157, 78, 221}, {0, 240, 255}}
	spinFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A6F9E"))
	textStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBD3EC"))
	boldStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	goodStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F0FF"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD300"))
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF003C"))
	cyanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F0FF"))
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#B5179E")).Padding(0, 1)
)

var banner = []string{
	"▄▀█ █▀ █▄▀   █▀▄▀█ █▀█ █▄ █ █ ▀█▀ █▀█ █▀█",
	"█▀█ ▄█ █ █   █ ▀ █ █▄█ █ ▀█ █  █  █▄█ █▀▄",
}

type (
	frameMsg       struct{}
	refreshMsg     struct{}
	refreshOnline  struct{}
	localLoadedMsg monitor.Local
	onlineLoaded   struct {
		status  *monitor.ServiceStatus
		version string
		err     error
	}
)

// monitorModel holds the dashboard state. Everything it shows comes from View,
// so a frame can be rendered without a running program.
type monitorModel struct {
	ctx       context.Context
	collector *monitor.Collector
	interval  time.Duration
	now       func() time.Time

	width, height int
	frame         int
	loaded        bool
	scanning      bool
	fetching      bool
	local         monitor.Local
	status        *monitor.ServiceStatus
	statusErr     error
	claudeVersion string
	syncedAt      time.Time

	// Gauges ease towards their real value.
	shownFive, shownSeven float64
	// catX is how far the cat has driven, in cells.
	catX float64
}

func newMonitorModel(ctx context.Context, collector *monitor.Collector, interval time.Duration) monitorModel {
	return monitorModel{ctx: ctx, collector: collector, interval: interval, now: time.Now, scanning: true, fetching: true}
}

func (m monitorModel) Init() tea.Cmd {
	return tea.Batch(frameTick(), m.loadLocal(), m.loadOnline(),
		tea.Tick(m.interval, func(time.Time) tea.Msg { return refreshMsg{} }),
		tea.Tick(onlineInterval, func(time.Time) tea.Msg { return refreshOnline{} }))
}

func frameTick() tea.Cmd {
	return tea.Tick(frameInterval, func(time.Time) tea.Msg { return frameMsg{} })
}

func (m monitorModel) loadLocal() tea.Cmd {
	collector := m.collector
	return func() tea.Msg { return localLoadedMsg(collector.Local()) }
}

func (m monitorModel) loadOnline() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		status, version, err := fetchOnline(ctx)
		return onlineLoaded{status: status, version: version, err: err}
	}
}

func (m monitorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m.refresh(true)
		}
	case frameMsg:
		m.frame++
		m.ease()
		return m, frameTick()
	case refreshMsg:
		next, cmd := m.refresh(false)
		return next, tea.Batch(cmd, tea.Tick(m.interval, func(time.Time) tea.Msg { return refreshMsg{} }))
	case refreshOnline:
		var cmd tea.Cmd
		if !m.fetching {
			m.fetching = true
			cmd = m.loadOnline()
		}
		return m, tea.Batch(cmd, tea.Tick(onlineInterval, func(time.Time) tea.Msg { return refreshOnline{} }))
	case localLoadedMsg:
		m.local = monitor.Local(msg)
		m.loaded, m.scanning = true, false
		m.syncedAt = m.now()
	case onlineLoaded:
		m.fetching = false
		m.statusErr = msg.err
		if msg.err == nil {
			m.status = msg.status
		}
		if msg.version != "" {
			m.claudeVersion = msg.version
		}
	}
	return m, nil
}

// refresh starts a local read unless one is running, as the collector can't run
// twice at once.
func (m monitorModel) refresh(online bool) (monitorModel, tea.Cmd) {
	var cmds []tea.Cmd
	if !m.scanning {
		m.scanning = true
		cmds = append(cmds, m.loadLocal())
	}
	if online && !m.fetching {
		m.fetching = true
		cmds = append(cmds, m.loadOnline())
	}
	return m, tea.Batch(cmds...)
}

func (m *monitorModel) ease() {
	m.shownFive += (windowPercent(m.local.Limits.FiveHour) - m.shownFive) * 0.15
	m.shownSeven += (windowPercent(m.local.Limits.SevenDay) - m.shownSeven) * 0.15
	m.catX += m.catSpeed()
}

// settle jumps the gauges to their real value, for a single printed frame.
func (m *monitorModel) settle() {
	m.shownFive = windowPercent(m.local.Limits.FiveHour)
	m.shownSeven = windowPercent(m.local.Limits.SevenDay)
	m.catX = float64(m.width)*0.6 + catWidth
}

func windowPercent(w *monitor.Window) float64 {
	if w == nil {
		return 0
	}
	return w.UsedPercent
}

func (m monitorModel) View() string {
	width := m.width
	if width <= 0 {
		return ""
	}
	limits := panel{"// PLAN LIMITS", m.limitsBody}
	tokens := panel{"// TOKENS", m.tokensBody}
	sessions := panel{"// LIVE SESSIONS", m.sessionsBody}
	service := panel{"// CLAUDE STATUS", m.serviceBody}
	models := panel{"// MODELS · 7 DAYS", m.modelsBody}
	ask := panel{"// ASK", m.askBody}

	var rows []string
	rows = append(rows, m.header(width))
	if width >= twoColumnsAt {
		left := (width - 1) / 2
		right := width - left - 1
		rows = append(rows,
			m.row(limits, tokens, left, right),
			m.row(sessions, service, left, right),
			m.row(models, ask, left, right))
	} else {
		for _, p := range []panel{limits, sessions, tokens, service, models, ask} {
			rows = append(rows, m.render(p, width, 0))
		}
	}
	body := strings.Join(rows, "\n")
	// The cat only drives when the panels still fit on the screen.
	if width >= minSceneWidth && (m.height <= 0 || lipgloss.Height(body)+sceneHeight < m.height) {
		rows = slices.Insert(rows, 1, m.scene(width))
		body = strings.Join(rows, "\n")
	}
	footer := m.footer(width)
	if m.height > 0 {
		lines := strings.Split(body, "\n")
		if len(lines) > m.height-1 {
			lines = lines[:max(m.height-1, 0)]
		}
		body = strings.Join(lines, "\n")
	}
	return body + "\n" + footer
}

type panel struct {
	title string
	body  func(width int) []string
}

func (m monitorModel) row(a, b panel, aWidth, bWidth int) string {
	first := m.render(a, aWidth, 0)
	second := m.render(b, bWidth, 0)
	height := max(lipgloss.Height(first), lipgloss.Height(second))
	return lipgloss.JoinHorizontal(lipgloss.Top, m.render(a, aWidth, height), " ", m.render(b, bWidth, height))
}

// render draws a panel in width cells. A height of 0 fits the content.
func (m monitorModel) render(p panel, width, height int) string {
	inner := width - 4
	lines := []string{gradient(p.title, m.phase()*2)}
	for _, line := range p.body(inner) {
		lines = append(lines, ansi.Truncate(line, inner, "…"))
	}
	style := panelStyle.Width(width - 2)
	if height > 2 {
		style = style.Height(height - 2)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m monitorModel) phase() float64 {
	return float64(m.frame) / 80
}

func (m monitorModel) header(width int) string {
	right := []string{m.vibe()}
	if m.claudeVersion != "" {
		right = append(right, dimStyle.Render("claude "+m.claudeVersion+" · ask "+version))
	} else {
		right = append(right, dimStyle.Render("ask "+version))
	}
	if width < lipgloss.Width(banner[0])+2 {
		return gradient("ASK MONITOR", m.phase()) + "\n" + ansi.Truncate(right[0], width, "…")
	}
	var lines []string
	for i, line := range banner {
		text := gradient(line, m.phase()+float64(i)*0.05)
		if room := width - lipgloss.Width(line) - 3; room >= lipgloss.Width(right[i]) {
			text += strings.Repeat(" ", room-lipgloss.Width(right[i])+3) + right[i]
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

// vibe sums up how things are going, most urgent first.
func (m monitorModel) vibe() string {
	if m.status != nil && m.status.Indicator != "" && m.status.Indicator != "none" {
		return badStyle.Render("claude's having a moment, bro")
	}
	used := max(windowPercent(m.local.Limits.FiveHour), windowPercent(m.local.Limits.SevenDay))
	switch {
	case !m.loaded:
		return cyanStyle.Render("warming up, bro")
	case used >= 95:
		return badStyle.Render("you're cooked, bro")
	case used >= 80:
		return warnStyle.Render("easy there, bro")
	case len(m.local.Sessions) > 0:
		return goodStyle.Render("locked in, bro")
	}
	return textStyle.Render("all chill, bro")
}

func (m monitorModel) footer(width int) string {
	state := "scanning…"
	if !m.syncedAt.IsZero() {
		state = "synced " + m.syncedAt.Format("15:04:05")
	}
	text := dimStyle.Render("> q quit · r refresh · " + state)
	return ansi.Truncate(text, width, "…")
}

func (m monitorModel) spinner() string {
	return string(spinFrames[m.frame%len(spinFrames)])
}

func (m monitorModel) limitsBody(width int) []string {
	limits := m.local.Limits
	if limits.UpdatedAt.IsZero() {
		return []string{
			textStyle.Render("Not linked yet. Set it up once:"),
			cyanStyle.Render("ask monitor statusline --help"),
			dimStyle.Render("Pro, Max and Ultra plans only"),
		}
	}
	now := m.now()
	lines := []string{
		m.limitLine("5H", limits.FiveHour, m.shownFive, width, now),
		m.limitLine("7D", limits.SevenDay, m.shownSeven, width, now),
	}
	return append(lines, dimStyle.Render("reported "+shortAge(now.Sub(limits.UpdatedAt))+" ago by a Claude Code session"))
}

func (m monitorModel) limitLine(label string, window *monitor.Window, shown float64, width int, now time.Time) string {
	if window == nil {
		return boldStyle.Render(label) + " " + dimStyle.Render("just reset, waiting for the next report")
	}
	suffix := fmt.Sprintf(" %3.0f%% ↻ %s", window.UsedPercent, shortAge(time.Unix(window.ResetsAt, 0).Sub(now)))
	bar := gauge(max(width-lipgloss.Width(label)-1-lipgloss.Width(suffix), 4), shown, m.frame)
	return boldStyle.Render(label) + " " + bar + percentStyle(window.UsedPercent).Render(suffix)
}

func (m monitorModel) tokensBody(width int) []string {
	usage := m.local.Usage
	if !m.loaded {
		return []string{cyanStyle.Render(m.spinner() + " reading transcripts…")}
	}
	stat := func(label string, tokens monitor.Tokens) string {
		return dimStyle.Render(label+" ") + boldStyle.Render(shortCount(tokens.Used()))
	}
	return []string{
		stat("5h", usage.Last5h) + "   " + stat("today", usage.Today) + "   " + stat("7d", usage.Week),
		dimStyle.Render("24h ") + sparkline(usage.Hourly[:], m.frame),
		dimStyle.Render("7d  ") + sparkline(usage.Daily[:], m.frame),
		dimStyle.Render("cache reads 7d " + shortCount(usage.Week.CacheRead)),
	}
}

func (m monitorModel) sessionsBody(width int) []string {
	sessions := m.local.Sessions
	if !m.loaded {
		return []string{cyanStyle.Render(m.spinner() + " looking around…")}
	}
	if len(sessions) == 0 {
		return []string{dimStyle.Render("Nobody's home. Start claude, bro.")}
	}
	now := m.now()
	var lines []string
	for i, session := range sessions {
		if i == maxSessions {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("+ %d more", len(sessions)-maxSessions)))
			break
		}
		lines = append(lines, m.sessionLine(session, width, now))
	}
	return lines
}

func (m monitorModel) sessionLine(session monitor.Session, width int, now time.Time) string {
	marker := dimStyle.Render("○")
	if session.Status == "busy" {
		marker = pulse(m.frame).Render(m.spinner())
	} else if session.Status != "" {
		marker = goodStyle.Render("●")
	}
	stats := dimStyle.Render(fmt.Sprintf(" %s %s", shortCount(session.Tokens), shortAge(now.Sub(session.Started))))
	name := session.Name
	if name == "" {
		name = session.Folder
	}
	room := max(width-2-lipgloss.Width(stats), 4)
	return marker + " " + lipgloss.NewStyle().Width(room).Render(ansi.Truncate(textStyle.Render(name), room, "…")) + stats
}

func (m monitorModel) serviceBody(width int) []string {
	if m.status == nil {
		if m.statusErr != nil {
			return []string{warnStyle.Render("Status page unreachable"), dimStyle.Render("retrying every minute")}
		}
		return []string{cyanStyle.Render(m.spinner() + " pinging the mothership…")}
	}
	lines := []string{indicatorStyle(m.status.Indicator).Render("● " + m.status.Description)}
	for _, incident := range m.status.Incidents {
		lines = append(lines, badStyle.Render("! "+incident))
	}
	for i, component := range m.status.Components {
		if i == maxComponents {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("+ %d more", len(m.status.Components)-maxComponents)))
			break
		}
		lines = append(lines, componentStyle(component.Status).Render("● ")+textStyle.Render(component.Name))
	}
	if m.statusErr != nil {
		lines = append(lines, dimStyle.Render("last check failed, showing older data"))
	}
	return lines
}

func (m monitorModel) modelsBody(width int) []string {
	models := m.local.Usage.Models
	if len(models) == 0 {
		return []string{dimStyle.Render("No tokens burned yet.")}
	}
	top := models[0].Used
	var lines []string
	for i, model := range models {
		if i == maxModels {
			break
		}
		name := strings.TrimPrefix(model.Model, "claude-")
		count := " " + shortCount(model.Used)
		const nameWidth = 16
		bar := max(width-nameWidth-1-lipgloss.Width(count), 4)
		filled := int(math.Round(float64(bar) * float64(model.Used) / float64(max(top, 1))))
		lines = append(lines, textStyle.Render(fmt.Sprintf("%-*s ", nameWidth, ansi.Truncate(name, nameWidth, "…")))+
			gradient(strings.Repeat("▰", filled), m.phase()+float64(i)*0.2)+
			dimStyle.Render(strings.Repeat("▱", bar-filled))+dimStyle.Render(count))
	}
	return lines
}

func (m monitorModel) askBody(width int) []string {
	ask := m.local.Ask
	return []string{
		dimStyle.Render("today ") + boldStyle.Render(fmt.Sprintf("%d answers", ask.AnswersToday)) + dimStyle.Render(fmt.Sprintf(" · est. $%.2f", ask.CostToday)),
		dimStyle.Render("7d    ") + boldStyle.Render(fmt.Sprintf("%d answers", ask.AnswersWeek)) + dimStyle.Render(fmt.Sprintf(" · est. $%.2f", ask.CostWeek)),
		dimStyle.Render("estimates, not your actual bill"),
	}
}

// gradient colors each letter along the neon palette, shifted by phase.
func gradient(text string, phase float64) string {
	runes := []rune(text)
	var b strings.Builder
	for i, r := range runes {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		position := float64(i)/float64(max(len(runes), 1))*0.6 + phase
		b.WriteString(lipgloss.NewStyle().Foreground(neonAt(position)).Bold(true).Render(string(r)))
	}
	return b.String()
}

// neonAt walks the palette in a loop, so a moving phase never jumps.
func neonAt(position float64) lipgloss.Color {
	return blendAt(neonStops, position)
}

func blendAt(stops [][3]float64, position float64) lipgloss.Color {
	position -= math.Floor(position)
	scaled := position * float64(len(stops))
	index := int(scaled) % len(stops)
	from, to := stops[index], stops[(index+1)%len(stops)]
	mix := scaled - math.Floor(scaled)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		int(from[0]+(to[0]-from[0])*mix),
		int(from[1]+(to[1]-from[1])*mix),
		int(from[2]+(to[2]-from[2])*mix)))
}

// gauge draws a bar that goes from green to pink, with a light running through
// its filled part.
func gauge(width int, percent float64, frame int) string {
	filled := int(math.Round(float64(width) * min(max(percent, 0), 100) / 100))
	var b strings.Builder
	for i := range width {
		if i >= filled {
			b.WriteString(dimStyle.Render("░"))
			continue
		}
		style := percentStyle(float64(i) / float64(width) * 100)
		if i == frame/2%max(filled, 1) {
			style = boldStyle
		}
		b.WriteString(style.Render("█"))
	}
	return b.String()
}

func percentStyle(percent float64) lipgloss.Style {
	switch {
	case percent >= 80:
		return badStyle
	case percent >= 50:
		return warnStyle
	}
	return goodStyle
}

// sparkline draws values relative to the largest, with a light moving along it.
func sparkline(values []int64, frame int) string {
	levels := []rune("▁▂▃▄▅▆▇█")
	var top int64
	for _, value := range values {
		top = max(top, value)
	}
	var b strings.Builder
	for i, value := range values {
		level := 0
		if top > 0 && value > 0 {
			level = int(math.Ceil(float64(value)/float64(top)*float64(len(levels)))) - 1
		}
		style := lipgloss.NewStyle().Foreground(neonAt(float64(i)/float64(len(values))*0.5 + float64(frame)/120))
		if value == 0 {
			style = dimStyle
		}
		b.WriteString(style.Render(string(levels[level])))
	}
	return b.String()
}

// pulse fades between two colors so busy sessions look alive.
func pulse(frame int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(neonAt(0.5 + math.Sin(float64(frame)/6)*0.15))
}

func indicatorStyle(indicator string) lipgloss.Style {
	switch indicator {
	case "none":
		return goodStyle
	case "minor":
		return warnStyle
	}
	return badStyle
}

func componentStyle(status string) lipgloss.Style {
	switch status {
	case "operational":
		return goodStyle
	case "degraded_performance", "under_maintenance":
		return warnStyle
	}
	return badStyle
}

func shortCount(n int64) string {
	value := float64(n)
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", value/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", value/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fk", value/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func shortAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02d", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}
