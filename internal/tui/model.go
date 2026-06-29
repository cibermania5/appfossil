package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cibermania5/appfossil/internal/model"
	"github.com/cibermania5/appfossil/internal/scan"
)

// Config holds the runtime options for the TUI.
type Config struct {
	ThresholdDays int
	IncludeSystem bool
}

type scanDoneMsg struct {
	apps []model.AppInfo
	diag scan.Diagnostics
}

// Model is the root bubbletea model.
type Model struct {
	cfg Config

	spinner   spinner.Model
	filterIn  textinput.Model
	loading   bool
	filtering bool

	all  []model.AppInfo // full scan result
	view []model.AppInfo // filtered + sorted

	cursor int
	offset int
	width  int
	height int

	sortMode     sortMode
	sourceFilter sourceFilter
	staleOnly    bool
	showDetail   bool

	diag scan.Diagnostics
}

// NewModel constructs the initial model.
func NewModel(cfg Config) Model {
	if cfg.ThresholdDays <= 0 {
		cfg.ThresholdDays = 90
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	ti := textinput.New()
	ti.Placeholder = "filter by name…"
	ti.Prompt = "/"
	ti.CharLimit = 64

	return Model{
		cfg:      cfg,
		spinner:  sp,
		filterIn: ti,
		loading:  true,
		width:    80,
		height:   24,
	}
}

// Init kicks off the spinner and the background scan.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runScan())
}

func (m Model) runScan() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		sc := scan.New(scan.Options{IncludeSystem: cfg.IncludeSystem})
		apps := sc.Scan()
		return scanDoneMsg{apps: apps, diag: sc.Diagnostics()}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
		return m, nil

	case scanDoneMsg:
		m.loading = false
		m.all = msg.apps
		m.diag = msg.diag
		m.refresh()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.filterIn.Blur()
		case "esc":
			m.filtering = false
			m.filterIn.Blur()
			m.filterIn.SetValue("")
			m.refresh()
		default:
			var cmd tea.Cmd
			m.filterIn, cmd = m.filterIn.Update(msg)
			m.refresh()
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.showDetail = false
	case "enter":
		if len(m.view) > 0 {
			m.showDetail = !m.showDetail
		}
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "pgup":
		m.moveCursor(-m.rowsPerPage())
	case "pgdown":
		m.moveCursor(m.rowsPerPage())
	case "home", "g":
		m.moveCursor(-len(m.view))
	case "end", "G":
		m.moveCursor(len(m.view))
	case "s":
		m.sortMode = (m.sortMode + 1) % 3
		m.refresh()
	case "f":
		m.sourceFilter = (m.sourceFilter + 1) % 5
		m.refresh()
	case "t":
		m.staleOnly = !m.staleOnly
		m.refresh()
	case "r":
		m.loading = true
		m.showDetail = false
		return m, tea.Batch(m.spinner.Tick, m.runScan())
	case "/":
		m.filtering = true
		m.filterIn.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

// refresh recomputes the filtered view and clamps the cursor.
func (m *Model) refresh() {
	m.view = applyFilters(m.all, m.sortMode, m.sourceFilter, m.staleOnly, m.cfg.ThresholdDays, m.filterIn.Value())
	if m.cursor >= len(m.view) {
		m.cursor = len(m.view) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampScroll()
}

func (m *Model) moveCursor(delta int) {
	if len(m.view) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.view) {
		m.cursor = len(m.view) - 1
	}
	m.clampScroll()
}

func (m *Model) clampScroll() {
	rows := m.rowsPerPage()
	if rows <= 0 {
		m.offset = 0
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// headerHeight returns the number of lines the header occupies.
func (m Model) headerHeight() int {
	if !m.diag.KnowledgeReadable {
		return 2 // summary + accuracy hint
	}
	return 1
}

// rowsPerPage returns how many data rows fit given the chrome height.
func (m Model) rowsPerPage() int {
	// header + column header(1) + footer(2) lines of chrome.
	r := m.height - m.headerHeight() - 3
	if r < 1 {
		r = 1
	}
	return r
}

// totals counts stale apps and reclaimable size in the current view.
func (m Model) totals() (stale int, staleBytes int64) {
	for _, a := range m.view {
		if a.IsStale(m.cfg.ThresholdDays) {
			stale++
			staleBytes += a.SizeBytes
		}
	}
	return
}

func (m Model) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Scanning installed apps…\n", m.spinner.View())
	}

	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	if m.showDetail {
		b.WriteString(m.detailView())
	} else {
		b.WriteString(m.tableView())
	}
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m Model) headerView() string {
	stale, staleBytes := m.totals()
	title := titleStyle.Render(" appfossil ")
	summary := subtitleStyle.Render(fmt.Sprintf(
		"  %d apps · %d stale (>%dd) · %s reclaimable",
		len(m.view), stale, m.cfg.ThresholdDays, model.HumanSize(staleBytes),
	))
	header := title + summary
	if !m.diag.KnowledgeReadable {
		header += "\n" + accuracyHintStyle.Render(
			"  ! limited accuracy: run with sudo (or grant Full Disk Access) for precise usage history")
	}
	return header
}

func (m Model) tableView() string {
	name, last, size, source := colWidths(m.width)

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(colorHeaderBg).
		Render(fmt.Sprintf("%s %s %s %s",
			fit("App", name), fit("Last Used", last), fit("Size", size), fit("Source", source)))

	var lines []string
	lines = append(lines, header)

	if len(m.view) == 0 {
		lines = append(lines, subtitleStyle.Render("  no apps match the current filters"))
		return strings.Join(lines, "\n") + strings.Repeat("\n", clampZero(m.rowsPerPage()-1))
	}

	rows := m.rowsPerPage()
	end := m.offset + rows
	if end > len(m.view) {
		end = len(m.view)
	}

	for i := m.offset; i < end; i++ {
		a := m.view[i]
		rowColor := stalenessColor(a.DaysSinceUsed, m.cfg.ThresholdDays)
		line := fmt.Sprintf("%s %s %s %s",
			fit(a.Name, name),
			fit(a.LastUsedLabel(), last),
			fit(a.SizeLabel(), size),
			fit(a.SourceLabel(), source),
		)
		style := lipgloss.NewStyle().Foreground(rowColor)
		if i == m.cursor {
			style = style.Background(lipgloss.Color("237")).Bold(true)
		}
		lines = append(lines, style.Render(line))
	}

	// Pad to a stable height so the footer doesn't jump.
	for len(lines)-1 < rows {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) detailView() string {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return ""
	}
	a := m.view[m.cursor]

	lastExact := "Never recorded"
	if a.LastUsed != nil {
		lastExact = a.LastUsed.Format("Mon 2 Jan 2006, 15:04")
		if a.LastUsedApprox {
			lastExact += "  (approx)"
		}
	}

	row := func(k, v string) string {
		return detailKeyStyle.Render(k) + detailValStyle.Render(v)
	}

	body := strings.Join([]string{
		row("Name", a.Name),
		row("Version", orDash(a.Version)),
		row("Bundle ID", orDash(a.BundleID)),
		row("Source", a.SourceLabel()),
		row("Last used", lastExact),
		row("Date from", a.LastUsedSource.Label()),
		row("Days idle", daysLabel(a.DaysSinceUsed)),
		row("Size", a.SizeLabel()),
		row("Path", a.Path),
	}, "\n")

	box := detailBoxStyle.Render(body)

	// Pad to keep total height stable with the table view.
	lines := strings.Count(box, "\n") + 1
	pad := m.rowsPerPage() + 1 - lines
	if pad > 0 {
		box += strings.Repeat("\n", pad)
	}
	return box
}

func (m Model) footerView() string {
	if m.filtering {
		return filterPromptStyle.Render(m.filterIn.View())
	}

	status := statusStyle.Render(fmt.Sprintf(
		" sort:%s  source:%s  stale-only:%s ",
		m.sortMode, m.sourceFilter, onOff(m.staleOnly),
	))
	help := helpStyle.Render("↑↓ move · enter details · s sort · f source · t stale · / filter · r rescan · q quit")
	return status + "\n" + help
}

func clampZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func daysLabel(d int) string {
	if d < 0 {
		return "unknown / never"
	}
	return fmt.Sprintf("%d days", d)
}
