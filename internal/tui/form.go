package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pwstore/internal/model"
)

type formModel struct {
	inputs  []textinput.Model
	focus   int
	editing bool
	err     string
}

func newForm(edit *model.Entry) formModel {
	labels := []string{"网站", "账号", "密码", "备注"}
	inputs := make([]textinput.Model, len(labels))
	for i, l := range labels {
		t := textinput.New()
		t.Prompt = l + ": "
		t.Placeholder = ""
		t.CharLimit = 256
		if i == 2 {
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		inputs[i] = t
	}
	f := formModel{inputs: inputs}
	if edit != nil {
		f.editing = true
		f.inputs[0].SetValue(edit.Site)
		f.inputs[1].SetValue(edit.Username)
		f.inputs[2].SetValue(edit.Password)
		f.inputs[3].SetValue(edit.Notes)
	}
	return f
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := &m.form
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			m.state = stList
			return m, nil
		case "g":
			if f.focus == 2 {
				f.inputs[2].SetValue(model.GeneratePassword(20))
				return m, nil
			}
		case "enter":
			if f.focus == len(f.inputs)-1 {
				return m.saveForm()
			}
			f.focus++
			return m, f.inputs[f.focus].Focus()
		case "tab", "down":
			if f.focus < len(f.inputs)-1 {
				f.focus++
				return m, f.inputs[f.focus].Focus()
			}
		case "shift+tab", "up":
			if f.focus > 0 {
				f.focus--
				return m, f.inputs[f.focus].Focus()
			}
		}
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return m, cmd
}

func (m Model) saveForm() (tea.Model, tea.Cmd) {
	f := &m.form
	e := model.Entry{
		Site:     strings.TrimSpace(f.inputs[0].Value()),
		Username: strings.TrimSpace(f.inputs[1].Value()),
		Password: f.inputs[2].Value(),
		Notes:    strings.TrimSpace(f.inputs[3].Value()),
	}
	if err := e.Validate(); err != nil {
		f.err = err.Error()
		return m, nil
	}
	created, err := m.vault.Upsert(e)
	if err != nil {
		f.err = err.Error()
		return m, nil
	}
	if created {
		return m, func() tea.Msg { return formSavedMsg{status: "✓ 已添加: " + e.Site} }
	}
	return m, func() tea.Msg { return formSavedMsg{status: "✓ 已更新: " + e.Site} }
}

func (m Model) formView() string {
	var b strings.Builder
	title := "新增记录"
	if m.form.editing {
		title = "编辑记录"
	}
	b.WriteString(titleStyle.Render("| " + title))
	b.WriteString("\n\n")
	for i, t := range m.form.inputs {
		b.WriteString(t.View())
		b.WriteString("\n\n")
		_ = i
	}
	if m.form.err != "" {
		b.WriteString(errStyle.Render("✗ " + m.form.err))
		b.WriteString("\n\n")
	}
	if m.form.focus == 2 {
		b.WriteString(dimStyle.Render("  密码字段按 g 生成随机强密码"))
		b.WriteString("\n\n")
	}
	b.WriteString(dimStyle.Render("  Enter 下一项 / 保存 · Tab 切换 · g 生成密码 · Esc 取消"))
	content := boxStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) confirmView() string {
	var b strings.Builder
	b.WriteString(errStyle.Render("! " + m.confirm.msg))
	b.WriteString("\n\n")
	b.WriteString(subStyle.Render("  y 确认删除 · n / Esc 取消"))
	content := boxStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "y", "Y":
			return m, func() tea.Msg { return confirmYesMsg{} }
		case "n", "N", "esc":
			return m, func() tea.Msg { return confirmNoMsg{} }
		}
	}
	return m, nil
}
