package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pwstore/internal/config"
	"pwstore/internal/store"
)

var ErrLocked = errors.New("auth: locked")

type LockState struct {
	Failed    int   `json:"failed"`
	LockUntil int64 `json:"lock_until"`
}

type Auth struct {
	lockPath string
}

func New() *Auth {
	return &Auth{lockPath: config.LockPath()}
}

func (a *Auth) load() LockState {
	var s LockState
	data, err := os.ReadFile(a.lockPath)
	if err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func (a *Auth) save(s LockState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return atomicWrite(a.lockPath, data)
}

func (a *Auth) RemainingLock() time.Duration {
	s := a.load()
	if s.LockUntil == 0 {
		return 0
	}
	until := time.Unix(0, s.LockUntil)
	if until.After(time.Now()) {
		return time.Until(until)
	}
	a.save(LockState{})
	return 0
}

func (a *Auth) CheckLock() error {
	if d := a.RemainingLock(); d > 0 {
		return fmt.Errorf("%w: 已锁定，剩余 %v", ErrLocked, d.Round(time.Second))
	}
	return nil
}

func (a *Auth) Unlock(vaultPath, password string) (*store.Vault, error) {
	if err := a.CheckLock(); err != nil {
		return nil, err
	}
	v, err := store.Open(vaultPath, password)
	if err == nil {
		_ = a.save(LockState{})
		return v, nil
	}
	if !errors.Is(err, store.ErrWrongPassword) {
		return nil, err
	}
	s := a.load()
	s.Failed++
	if s.Failed >= config.MaxAttempts {
		s.LockUntil = time.Now().Add(config.LockDuration).UnixNano()
		s.Failed = 0
		_ = a.save(s)
		return nil, fmt.Errorf("%w: 输错次数过多，账户已锁定 %v", ErrLocked, config.LockDuration)
	}
	_ = a.save(s)
	return nil, fmt.Errorf("主密码错误，还剩 %d 次机会", config.MaxAttempts-s.Failed)
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".lock-*.tmp")
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
