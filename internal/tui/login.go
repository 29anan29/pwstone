package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pwstore/internal/config"
)

type loginModel struct {
	input textinput.Model
	err   string
}

func newLogin() loginModel {
	t := textinput.New()
	t.Placeholder = "主密码"
	t.EchoMode = textinput.EchoPassword
	t.EchoCharacter = '•'
	t.CharLimit = 128
	t.Focus()
	return loginModel{input: t}
}

func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		pw := m.login.input.Value()
		if pw == "" {
			m.login.err = "请输入主密码"
			return m, nil
		}
		m.login.err = ""
		return m, unlockCmd(m.auth, pw)
	}
	var cmd tea.Cmd
	m.login.input, cmd = m.login.input.Update(msg)
	return m, cmd
}

func (m Model) loginView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔐 pwstore 密码管理器"))
	b.WriteString(dimStyle.Render("  v" + config.Version))
	b.WriteString("\n\n")
	b.WriteString(subStyle.Render("请输入主密码解锁（连续输错 5 次将锁定 30 秒）"))
	b.WriteString("\n\n")
	b.WriteString(m.login.input.View())
	b.WriteString("\n\n")
	if m.login.err != "" {
		b.WriteString(errStyle.Render("❌ " + m.login.err))
	} else {
		b.WriteString(subStyle.Render("Esc/Ctrl+C 退出"))
	}
	content := boxStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
