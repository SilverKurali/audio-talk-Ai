//go:build windows

package main

import (
	"os"
)

func acquireLock() *os.File {
	// Windows: O_EXCL provides atomic file creation as a lock mechanism.
	f, err := os.OpenFile(lockPath(), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil
	}
	return f
}

func releaseLock(f *os.File) {
	if f != nil {
		path := f.Name()
		f.Close()
		os.Remove(path)
	}
}