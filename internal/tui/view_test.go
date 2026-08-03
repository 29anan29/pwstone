package tui

import (
	"strings"
	"testing"

	"pwstore/internal/model"
)

func TestLoginViewRenders(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	v := m.View()
	if !strings.Contains(v, "PW") && !strings.Contains(v, "█") {
		t.Fatal("banner missing")
	}
	if !strings.Contains(v, "请输入主密码") {
		t.Fatal("login prompt missing")
	}
}

func TestListViewRenders(t *testing.T) {
	m := New()
	m.width, m.height = 120, 40
	m.entries = []model.Entry{
		{Site: "github.com", Username: "alice", Password: "secret-pw", Notes: "工作"},
		{Site: "gitlab.com", Username: "bob", Password: "other-pw"},
	}
	m.state = stList
	m.list.revealed[0] = true
	v := m.View()
	for _, want := range []string{"╭", "╰", "网站", "github.com", "alice", "secret-pw"} {
		if !strings.Contains(v, want) {
			t.Fatalf("list view missing %q", want)
		}
	}
	if strings.Contains(v, "🔐") || strings.Contains(v, "📭") {
		t.Fatal("emoji found in list view")
	}
}

func TestEmptyListView(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.state = stList
	v := m.View()
	if !strings.Contains(v, "暂无记录") {
		t.Fatal("empty state missing")
	}
}
