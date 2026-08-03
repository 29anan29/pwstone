package tui

import (
	"path/filepath"
	"time"

	"pwstore/internal/config"
	"pwstore/internal/store"
)

type loginSuccessMsg struct{ vault *store.Vault }
type loginFailedMsg struct{ err error }
type lockMsg struct{}
type addMsg struct{}
type editMsg struct{ index int }
type deleteMsg struct{ index int }
type confirmYesMsg struct{}
type confirmNoMsg struct{}
type formSavedMsg struct{ status string }
type exportMsg struct{}
type exportDoneMsg struct {
	path string
	err  error
}
type statusMsg struct{ text string }

type confirmModel struct {
	index int
	msg   string
}

func vaultPath() string { return config.VaultPath() }

func exportPath() (string, error) {
	ts := time.Now().Format("20060102_150405")
	return filepath.Join(".", "pwstore_export_"+ts+".txt"), nil
}
