//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func replaceExec(cur, newPath string) error {
	dir := filepath.Dir(cur)
	script := filepath.Join(dir, "pw-update.bat")
	content := "@echo off\r\n" +
		"timeout /t 1 /nobreak >nul\r\n" +
		"move /y \"" + cur + "\" \"" + cur + ".old\" >nul 2>&1\r\n" +
		"move /y \"" + newPath + "\" \"" + cur + "\" >nul 2>&1\r\n" +
		"del \"" + cur + ".old\" >nul 2>&1\r\n" +
		"del \"%~f0\"\r\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		return err
	}
	if err := exec.Command("cmd", "/c", script).Start(); err != nil {
		return fmt.Errorf("启动替换脚本失败: %w", err)
	}
	return nil
}
