//go:build !windows

package main

import (
	"os"
	"syscall"
)

func acquireLock() *os.File {
	path := lockPath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil
	}
	return f
}

func releaseLock(f *os.File) {
	if f != nil {
		path := f.Name()
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		os.Remove(path)
	}
}