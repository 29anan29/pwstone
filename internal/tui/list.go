package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pwstore/internal/config"
	"pwstore/internal/model"
)

type listModel struct {
	filter    textinput.Model
	filtering bool
	cursor    int
	revealed  map[int]bool
	status    string
}

func newList() listModel {
	f := textinput.New()
	f.Placeholder = "搜索网站 / 账号 / 备注…"
	f.CharLimit = 64
	return listModel{filter: f, revealed: map[int]bool{}}
}

func (lm *listModel) shown(entries []model.Entry) []model.Entry {
	q := strings.ToLower(strings.TrimSpace(lm.filter.Value()))
	if q == "" {
		return entries
	}
	var out []model.Entry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Site), q) ||
			strings.Contains(strings.ToLower(e.Username), q) ||
			strings.Contains(strings.ToLower(e.Notes), q) {
			out = append(out, e)
		}
	}
	return out
}

func (lm *listModel) clamp(n int) {
	if n == 0 {
		lm.cursor = 0
		return
	}
	if lm.cursor >= n {
		lm.cursor = n - 1
	}
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	lm := &m.list
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		shown := lm.shown(m.entries)

		if lm.filtering {
			switch k {
			case "esc":
				lm.filtering = false
				lm.filter.Blur()
			case "enter":
				lm.filtering = false
				lm.filter.Blur()
			default:
				lm.filter, _ = lm.filter.Update(msg)
			}
			lm.clamp(len(lm.shown(m.entries)))
			return m, nil
		}

		switch k {
		case "up", "k":
			if lm.cursor > 0 {
				lm.cursor--
			}
		case "down", "j":
			if lm.cursor < len(shown)-1 {
				lm.cursor++
			}
		case "home", "g":
			lm.cursor = 0
		case "end", "G":
			lm.cursor = len(shown) - 1
		case "/":
			lm.filtering = true
			lm.filter.Focus()
			return m, nil
		case " ":
			if len(shown) > 0 {
				lm.revealed[lm.cursor] = !lm.revealed[lm.cursor]
			}
		case "c":
			if len(shown) > 0 {
				e := shown[lm.cursor]
				if err := clipboard.WriteAll(e.Password); err != nil {
					lm.status = "✗ 复制失败: " + err.Error()
				} else {
					lm.status = "✓ 已复制密码: " + e.Site
				}
			}
		case "a":
			return m, func() tea.Msg { return addMsg{} }
		case "e":
			if len(shown) > 0 {
				return m, func() tea.Msg { return editMsg{index: findIndex(m.entries, shown[lm.cursor].ID())} }
			}
		case "d":
			if len(shown) > 0 {
				return m, func() tea.Msg { return deleteMsg{index: findIndex(m.entries, shown[lm.cursor].ID())} }
			}
		case "x":
			return m, func() tea.Msg { return exportMsg{} }
		case "l":
			return m, func() tea.Msg { return lockMsg{} }
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func findIndex(entries []model.Entry, id string) int {
	for i, e := range entries {
		if e.ID() == id {
			return i
		}
	}
	return 0
}

func (m Model) listView() string {
	lm := &m.list
	shown := lm.shown(m.entries)
	lm.clamp(len(shown))

	widths := []int{5, 20, 16, 14, 12}
	headers := []string{"#", "网站", "账号", "密码", "备注"}
	if m.width < 90 {
		widths = []int{4, 18, 14, 12}
		headers = []string{"#", "网站", "账号", "密码"}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("pw"))
	b.WriteString(dimStyle.Render("  v" + config.Version + " · 共 " + fmt.Sprint(len(m.entries)) + " 条 · 显示 " + fmt.Sprint(len(shown)) + " 条"))
	b.WriteString("\n\n")

	if len(shown) == 0 {
		b.WriteString(boxStyle.Render(dimStyle.Render("- 暂无记录 — 按 a 添加，/ 搜索")))
		b.WriteString("\n\n")
	} else {
		b.WriteString(headStyle.Render(tableSep("╭", "┬", "╮", widths)))
		b.WriteString("\n")
		b.WriteString(headStyle.Render(tableRow(headers, widths)))
		b.WriteString("\n")
		b.WriteString(headStyle.Render(tableSep("├", "┼", "┤", widths)))
		b.WriteString("\n")

		visible := max(6, m.height-12)
		pageStart := (lm.cursor / visible) * visible
		pageEnd := min(pageStart+visible, len(shown))
		for i := pageStart; i < pageEnd; i++ {
			e := shown[i]
			pw := e.Password
			if !lm.revealed[i] {
				pw = strings.Repeat("*", min(len([]rune(pw)), 12))
			}
			cells := []string{fmt.Sprint(i + 1), e.Site, e.Username, pw}
			if len(headers) == 5 {
				cells = append(cells, e.Notes)
			}
			row := tableRow(cells, widths)
			if i == lm.cursor {
				row = selStyle.Render(row)
			}
			b.WriteString(row)
			b.WriteString("\n")
		}
		b.WriteString(headStyle.Render(tableSep("╰", "┴", "╯", widths)))
		b.WriteString("\n")
	}

	if lm.filtering {
		b.WriteString(lipgloss.NewStyle().Foreground(accent).Render("搜索> ") + lm.filter.View())
	} else {
		b.WriteString(dimStyle.Render("搜索: / · 选择: ↑↓ j k · 显示密码: 空格 · 复制: c"))
	}
	b.WriteString("\n")

	if lm.status != "" {
		b.WriteString(lm.status)
	} else {
		b.WriteString(dimStyle.Render("新增: a · 编辑: e · 删除: d · 导出: x · 锁定: l · 退出: q"))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 2).Render(b.String()))
}
