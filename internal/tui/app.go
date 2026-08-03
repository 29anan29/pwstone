package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"pwstore/internal/auth"
	"pwstore/internal/model"
	"pwstore/internal/store"
)

type state int

const (
	stLogin state = iota
	stList
	stForm
	stConfirm
)

type Model struct {
	state   state
	width   int
	height  int
	auth    *auth.Auth
	vault   *store.Vault
	entries []model.Entry

	login   loginModel
	list    listModel
	form    formModel
	confirm confirmModel
}

func New() Model {
	return Model{
		auth:  auth.New(),
		state: stLogin,
		login: newLogin(),
		list:  newList(),
		form:  newForm(nil),
	}
}

func (m Model) Init() tea.Cmd {
	return m.login.input.Focus()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeInputs()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch msg.(type) {
	case loginSuccessMsg:
		return m.applyLogin(msg.(loginSuccessMsg)), nil
	case loginFailedMsg:
		m.login.err = msg.(loginFailedMsg).err.Error()
		return m, nil
	case lockMsg:
		m.logout()
		return m, nil
	case addMsg:
		m.form = newForm(nil)
		m.state = stForm
		return m, m.form.inputs[0].Focus()
	case editMsg:
		e := m.entries[msg.(editMsg).index]
		m.form = newForm(&e)
		m.state = stForm
		return m, m.form.inputs[0].Focus()
	case deleteMsg:
		i := msg.(deleteMsg).index
		e := m.entries[i]
		m.confirm = confirmModel{index: i, msg: "确定删除 " + e.Site + " / " + e.Username + " ? (y/n)"}
		m.state = stConfirm
		return m, nil
	case confirmYesMsg:
		e := m.entries[m.confirm.index]
		m.vault.Delete(e.Site, e.Username)
		m.entries = m.vault.Entries()
		m.list.status = "✓ 已删除: " + e.Site
		m.state = stList
		return m, nil
	case confirmNoMsg:
		m.state = stList
		return m, nil
	case formSavedMsg:
		msg := msg.(formSavedMsg)
		m.entries = m.vault.Entries()
		m.list.status = msg.status
		m.state = stList
		return m, nil
	case exportMsg:
		return m, exportCmd(m.vault)
	case exportDoneMsg:
		msg := msg.(exportDoneMsg)
		if msg.err != nil {
			m.list.status = "✗ 导出失败: " + msg.err.Error()
		} else {
			m.list.status = "✓ 已导出: " + msg.path
		}
		return m, nil
	case statusMsg:
		m.list.status = msg.(statusMsg).text
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case stLogin:
		return m.updateLogin(msg)
	case stList:
		return m.updateList(msg)
	case stForm:
		return m.updateForm(msg)
	case stConfirm:
		return m.updateConfirm(msg)
	}
	return m, cmd
}

func (m Model) applyLogin(msg loginSuccessMsg) Model {
	m.vault = msg.vault
	m.entries = m.vault.Entries()
	m.list = newList()
	m.state = stList
	return m
}

func (m *Model) logout() {
	if m.vault != nil {
		m.vault.Close()
	}
	m.vault = nil
	m.entries = nil
	m.login = newLogin()
	m.state = stLogin
}

func (m *Model) resizeInputs() {
	w := max(20, m.width-12)
	m.login.input.Width = w
	m.list.filter.Width = max(20, w-4)
	for i := range m.form.inputs {
		m.form.inputs[i].Width = w
	}
}

func (m Model) View() string {
	switch m.state {
	case stLogin:
		return m.loginView()
	case stList:
		return m.listView()
	case stForm:
		return m.formView()
	case stConfirm:
		return m.confirmView()
	}
	return ""
}

func unlockCmd(a *auth.Auth, password string) tea.Cmd {
	return func() tea.Msg {
		v, err := a.Unlock(vaultPath(), password)
		if err != nil {
			return loginFailedMsg{err: err}
		}
		return loginSuccessMsg{vault: v}
	}
}

func exportCmd(v *store.Vault) tea.Cmd {
	return func() tea.Msg {
		path, err := exportPath()
		if err != nil {
			return exportDoneMsg{err: err}
		}
		return exportDoneMsg{path: path, err: v.Export(path)}
	}
}
