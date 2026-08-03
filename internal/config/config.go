package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	AppDirName   = ".pwstore"
	VaultFile    = "vault.dat"
	BackupFile   = "vault.bak"
	LockFile     = "lock.json"
	MaxAttempts  = 5
	LockDuration = 30 * time.Second
)

func AppDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = filepath.Join(os.Getenv("LOCALAPPDATA"), AppDirName)
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support", AppDirName)
	default:
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share", AppDirName)
	}
	return base
}

func InitDir() error {
	dir := AppDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Chmod(dir, 0o700)
	}
	return nil
}

func VaultPath() string  { return filepath.Join(AppDir(), VaultFile) }
func BackupPath() string { return filepath.Join(AppDir(), BackupFile) }
func LockPath() string   { return filepath.Join(AppDir(), LockFile) }
