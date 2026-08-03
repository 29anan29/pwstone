//go:build !windows

package updater

import "os"

func replaceExec(cur, newPath string) error {
	old := cur + ".old"
	_ = os.Remove(old)
	if err := os.Rename(cur, old); err != nil {
		return err
	}
	if err := os.Rename(newPath, cur); err != nil {
		_ = os.Rename(old, cur)
		return err
	}
	_ = os.Chmod(cur, 0o755)
	_ = os.Remove(old)
	return nil
}
