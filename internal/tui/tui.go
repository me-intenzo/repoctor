//go:build !clionly

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/me-intenzo/repoctor/internal/finding"
	"github.com/me-intenzo/repoctor/internal/gitutil"
	"github.com/me-intenzo/repoctor/internal/run"
	"github.com/me-intenzo/repoctor/internal/scan"
)

var (
	cBorder = lipgloss.Color("63")
	cDim    = lipgloss.Color("241")
	cText   = lipgloss.Color("252")
	cAccent = lipgloss.Color("205")
	cOk     = lipgloss.Color("42")

	headerBar = lipgloss.NewStyle().Background(cAccent).Foreground(lipgloss.Color("230")).Bold(true).Padding(0, 1)
	footerBar = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("250")).Padding(0, 1)
	boxStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cBorder).Padding(0, 1)
	dimStyle  = lipgloss.NewStyle().Foreground(cDim)
	labelStyle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	critStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
	okStyle   = lipgloss.NewStyle().Foreground(cOk)
)

type scanDoneMsg struct {
	path     string
	findings []finding.Finding
}

type model struct {
	root     string
	repos    []string
	filtered []string
	filter   string
	filtering bool
	cursor   int
	offset   int
	selected string
	findings []finding.Finding
	scanned  bool
	scanning bool
	scroll   int
	width    int
	height   int
}

func Run(root string) error {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return fmt.Errorf("cannot scan %q: no such directory (check the path, it is case-sensitive)", root)
	}
	repos := scan.DiscoverRepos(root)
	m := model{root: root, repos: repos, filtered: repos, height: 24, width: 100}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) applyFilter() model {
	m.filtered = m.filtered[:0]
	f := strings.ToLower(m.filter)
	for _, r := range m.repos {
		if f == "" || strings.Contains(strings.ToLower(r), f) {
			m.filtered = append(m.filtered, r)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.offset = 0
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case scanDoneMsg:
		m.scanning = false
		m.selected = msg.path
		m.findings = msg.findings
		m.scanned = true
		m.scroll = 0
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m = m.applyFilter()
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m = m.applyFilter()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "backspace":
			if m.scanned {
				m.scanned = false
				m.findings = nil
				return m, nil
			}
			return m, tea.Quit
		case "/":
			if !m.scanned {
				m.filtering = true
			}
		case "up", "k":
			if m.scanned {
				if m.scroll > 0 {
					m.scroll--
				}
			} else if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.scanned {
				m.scroll++
			} else if m.cursor < len(m.filtered)-1 {
				m.cursor++
				lh := m.height - 10
				if lh < 3 {
					lh = 3
				}
				if m.cursor >= m.offset+lh {
					m.offset = m.cursor - lh + 1
				}
			}
		case "enter":
			if !m.scanned && !m.scanning && len(m.filtered) > 0 {
				m.scanning = true
				repo := m.filtered[m.cursor]
				return m, func() tea.Msg {
					findings, _ := run.All(repo)
					return scanDoneMsg{path: repo, findings: findings}
				}
			}
		}
	}
	return m, nil
}

func severityIcon(s string) string {
	switch s {
	case "critical":
		return critStyle.Render("●")
	case "warning":
		return warnStyle.Render("●")
	case "info":
		return infoStyle.Render("●")
	default:
		return errStyle.Render("●")
	}
}

func (m model) statusHints() string {
	if m.filtering {
		return "type to filter · enter/esc to done"
	}
	if m.scanned {
		return "j/k scroll · esc back · q quit"
	}
	return "j/k move · / filter · enter scan · q quit"
}

func (m model) View() string {
	title := headerBar.Width(m.width).Render(fmt.Sprintf(" repoctor  ·  %d repos under %s", len(m.repos), m.root))
	footer := footerBar.Width(m.width).Render(m.statusHints())

	if m.scanned {
		var body strings.Builder
		body.WriteString(labelStyle.Render("path   ") + m.selected + "\n")
		url := gitutil.RemoteURL(m.selected)
		if url == "" {
			url = dimStyle.Render("(no origin remote)")
		}
		body.WriteString(labelStyle.Render("remote ") + url + "\n")
		user := gitutil.UserName(m.selected)
		if user == "" {
			user = dimStyle.Render("(not configured)")
		}
		body.WriteString(labelStyle.Render("user   ") + user + "\n\n")

		counts := map[string]int{}
		for _, f := range m.findings {
			counts[f.Severity]++
		}
		if len(m.findings) == 0 {
			body.WriteString(okStyle.Render("✓ no findings — all clear"))
		} else {
			for _, k := range []string{"critical", "warning", "info", "error"} {
				if counts[k] > 0 {
					body.WriteString(fmt.Sprintf("%s %-9s %d\n", severityIcon(k), k, counts[k]))
				}
			}
			body.WriteString("\n")
			lines := []string{}
			for _, f := range m.findings {
				lines = append(lines, fmt.Sprintf("%s [%s] %s", severityIcon(f.Severity), f.Check, f.Message))
				if f.Fix != "" {
					lines = append(lines, dimStyle.Render("   fix: "+f.Fix))
				}
			}
			lh := m.height - 14
			if lh < 3 {
				lh = 3
			}
			if m.scroll > len(lines)-lh {
				m.scroll = max(0, len(lines)-lh)
			}
			end := m.scroll + lh
			if end > len(lines) {
				end = len(lines)
			}
			body.WriteString(strings.Join(lines[m.scroll:end], "\n"))
		}
		return title + "\n" + boxStyle.Width(max(40, m.width-4)).Render(body.String()) + "\n" + footer
	}

	var list strings.Builder
	lh := m.height - 10
	if lh < 3 {
		lh = 3
	}
	end := m.offset + lh
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	for i := m.offset; i < end; i++ {
		name := filepath.Base(m.filtered[i])
		row := name + dimStyle.Render("  "+m.filtered[i])
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(cAccent).Bold(true).Render("▸ "+name) + dimStyle.Render("  "+m.filtered[i])
		} else {
			row = "  " + name + dimStyle.Render("  "+m.filtered[i])
		}
		list.WriteString(row + "\n")
	}
	if len(m.filtered) == 0 {
		list.WriteString(dimStyle.Render("no repos match"))
	}

	detail := dimStyle.Render("enter to scan the selected repo")
	if len(m.filtered) > 0 {
		p := m.filtered[m.cursor]
		url := gitutil.RemoteURL(p)
		if url == "" {
			url = dimStyle.Render("(no origin remote)")
		}
		user := gitutil.UserName(p)
		if user == "" {
			user = dimStyle.Render("(not configured)")
		}
		detail = labelStyle.Render("path   ") + p + "\n" +
			labelStyle.Render("remote ") + url + "\n" +
			labelStyle.Render("user   ") + user
	}

	leftW := m.width/2 - 4
	if leftW < 30 {
		leftW = 30
	}
	rightW := m.width - leftW - 6
	if rightW < 30 {
		rightW = 30
	}
	filterLine := ""
	if m.filtering || m.filter != "" {
		filterLine = dimStyle.Render("filter: ") + m.filter + "\n\n"
	}
	left := boxStyle.Width(leftW).Render(filterLine + list.String())
	right := boxStyle.Width(rightW).Render(detail)
	if m.scanning {
		right = boxStyle.Width(rightW).Render(dimStyle.Render("scanning…"))
	}
	return title + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n" + footer
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

