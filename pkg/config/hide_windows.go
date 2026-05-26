//go:build windows

package config

import (
	"os"
	"syscall"
)

func secureFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN)
}
