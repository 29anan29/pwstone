package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accent     = lipgloss.Color("#7D56F4")
	accentSoft = lipgloss.Color("#A78BFA")
	red        = lipgloss.Color("#F87171")
	green      = lipgloss.Color("#4ADE80")
	gray       = lipgloss.Color("#9CA3AF")

	titleStyle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	subStyle    = lipgloss.NewStyle().Foreground(gray)
	errStyle    = lipgloss.NewStyle().Foreground(red).Bold(true)
	okStyle     = lipgloss.NewStyle().Foreground(green)
	dimStyle    = lipgloss.NewStyle().Foreground(gray)
	bannerStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentSoft).
			Padding(0, 2)
	selStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3B2D6E")).
			Foreground(lipgloss.Color("#EDE9FE"))
	headStyle = lipgloss.NewStyle().Foreground(accentSoft).Bold(true)
)

const bannerArt = ` ██████╗ ██╗    ██╗   ███████╗████████╗ ██████╗ ██████╗ ███████╗
██╔═══██╗██║    ██║   ██╔════╝╚══██╔══╝██╔═══██╗██╔══██╗██╔════╝
██║   ██║██║ █╗ ██║   ███████╗   ██║   ██║   ██║██████╔╝█████╗  
██║   ██║██║███╗██║   ╚════██║   ██║   ██║   ██║██╔══██╗██╔══╝  
╚██████╔╝╚███╔███╔╝   ███████║   ██║   ╚██████╔╝██║  ██║███████║
 ╚═════╝  ╚══╝╚══╝    ╚══════╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚══════╝`

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func pad(s string, n int) string {
	diff := n - len([]rune(s))
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}

func tableSep(left, mid, right string, widths []int) string {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range widths {
		b.WriteString(strings.Repeat("─", w))
		if i < len(widths)-1 {
			b.WriteString(mid)
		} else {
			b.WriteString(right)
		}
	}
	return b.String()
}

func tableRow(cells []string, widths []int) string {
	var b strings.Builder
	b.WriteString("│")
	for i, c := range cells {
		w := widths[i]
		b.WriteString(pad(trunc(c, w), w))
		b.WriteString("│")
	}
	return b.String()
}
