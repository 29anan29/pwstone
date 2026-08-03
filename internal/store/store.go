package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pwstore/internal/crypto"
	"pwstore/internal/model"
)

var ErrWrongPassword = crypto.ErrWrongPassword

type Vault struct {
	path    string
	key     []byte
	params  crypto.Params
	entries []model.Entry
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Create(path, password string) (*Vault, error) {
	params, err := crypto.NewParams()
	if err != nil {
		return nil, err
	}
	key, err := params.DeriveKey(password)
	if err != nil {
		return nil, err
	}
	v := &Vault{path: path, key: key, params: params}
	if err := v.Save(); err != nil {
		return nil, err
	}
	return v, nil
}

func Open(path, password string) (*Vault, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	params, entries, err := crypto.DecodeVault(raw, password)
	if err != nil {
		return nil, err
	}
	key, err := params.DeriveKey(password)
	if err != nil {
		return nil, err
	}
	return &Vault{path: path, key: key, params: params, entries: entries}, nil
}

func (v *Vault) Path() string { return v.path }

func (v *Vault) Entries() []model.Entry {
	out := make([]model.Entry, len(v.entries))
	copy(out, v.entries)
	return out
}

func (v *Vault) Get(site, username string) (model.Entry, bool) {
	for _, e := range v.entries {
		if e.Site == site && (username == "" || e.Username == username) {
			return e, true
		}
	}
	return model.Entry{}, false
}

func (v *Vault) Upsert(e model.Entry) (bool, error) {
	if err := e.Validate(); err != nil {
		return false, err
	}
	for i := range v.entries {
		if v.entries[i].ID() == e.ID() {
			v.entries[i] = e
			if err := v.Save(); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	v.entries = append(v.entries, e)
	if err := v.Save(); err != nil {
		return false, err
	}
	return true, nil
}

func (v *Vault) Delete(site, username string) bool {
	out := v.entries[:0]
	found := false
	for _, e := range v.entries {
		if e.Site == site && (username == "" || e.Username == username) {
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		return false
	}
	v.entries = out
	_ = v.Save()
	return true
}

func (v *Vault) Search(kw string) []model.Entry {
	kw = strings.ToLower(strings.TrimSpace(kw))
	if kw == "" {
		return v.Entries()
	}
	var out []model.Entry
	for _, e := range v.entries {
		if strings.Contains(strings.ToLower(e.Site), kw) ||
			strings.Contains(strings.ToLower(e.Username), kw) ||
			strings.Contains(strings.ToLower(e.Notes), kw) {
			out = append(out, e)
		}
	}
	return out
}

func (v *Vault) Rekey(newPassword string) error {
	params, err := crypto.NewParams()
	if err != nil {
		return err
	}
	key, err := params.DeriveKey(newPassword)
	if err != nil {
		return err
	}
	v.params, v.key = params, key
	return v.Save()
}

func (v *Vault) Export(path string) error {
	var sb strings.Builder
	sb.WriteString("# pwstore 明文导出\n")
	sb.WriteString("# 格式: 网站 | 账号 | 密码 | 备注\n")
	for _, e := range v.entries {
		sb.WriteString(fmt.Sprintf("%s | %s | %s | %s\n", e.Site, e.Username, e.Password, e.Notes))
	}
	sb.WriteString(fmt.Sprintf("# 共 %d 条记录\n", len(v.entries)))
	return atomicWrite(path, []byte(sb.String()))
}

func (v *Vault) Close() {
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
}

func (v *Vault) Save() error {
	raw, err := crypto.EncodeVault(v.key, v.params, v.entries)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(v.path); err == nil {
		if werr := atomicWrite(v.path+".bak", old); werr != nil {
			return werr
		}
	}
	return atomicWrite(v.path, raw)
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pwstore-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
