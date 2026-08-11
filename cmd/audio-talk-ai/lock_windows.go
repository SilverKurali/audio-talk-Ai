//go:build windows

package main

import (
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// acquireLock obtains a single-instance lock. On Windows the classic Unix
// flock(2) trick does not apply (the kernel does not auto-release a file lock
// when the owning process dies), so the lock is an exclusive file plus the
// owner's PID. If a stale lock file is left behind by a crashed/killed
// previous instance, the PID check below detects that the owner is gone and
// reclaims the lock instead of falsely reporting "already running".
func acquireLock() *os.File {
	path := lockPath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		// Lock file already exists. Decide whether the owner is still alive:
		// if not, it's an orphan from a crash/SIGKILL and we can reclaim it.
		if reclaimOrphanLock(path) {
			// Retry the exclusive create now that the orphan is gone.
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		}
		if err != nil {
			return nil
		}
	}
	// Record this process's PID so a future crash here can be detected.
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
	return f
}

func releaseLock(f *os.File) {
	if f != nil {
		path := f.Name()
		f.Close()
		os.Remove(path)
	}
}

// reclaimOrphanLock returns true if the lock file at path belongs to a process
// that is no longer running (and in that case also deletes the stale file).
func reclaimOrphanLock(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		// Corrupt/empty lock — treat as orphan; better to let the user start
		// than to wedge them out with an unreadable lock.
		_ = os.Remove(path)
		return true
	}
	if pidRunning(pid) {
		return false // real owner still running
	}
	_ = os.Remove(path)
	return true
}

var (
	modKernel32PID           = windows.NewLazySystemDLL("kernel32.dll")
	procOpenProcessPID       = modKernel32PID.NewProc("OpenProcess")
	procGetExitCodeProcessID = modKernel32PID.NewProc("GetExitCodeProcess")
	procCloseHandlePID       = modKernel32PID.NewProc("CloseHandle")
)

// pidRunning reports whether a Windows process with the given PID exists and
// is still running. It opens the process (minimal access) and queries its exit
// status. Access denied is treated as "alive" (the process exists but we lack
// rights), so we never wrongly reclaim a live instance.
func pidRunning(pid int) bool {
	const (
		processQueryLimitedInformation = 0x1000
		stillActive                    = 259
	)
	// LazyProc.Call's third return value is the syscall errno (the
	// LastError at the point of the call). windows.GetLastError() read after
	// the fact is unreliable here because intervening Go runtime calls can
	// clobber it, so use the returned err directly.
	handle, _, callErr := procOpenProcessPID.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		// ERROR_INVALID_PARAMETER means the PID identifies no process; any
		// other error (e.g. access denied) means the process exists but we
		// can't query it — assume alive.
		return callErr != windows.ERROR_INVALID_PARAMETER
	}
	defer procCloseHandlePID.Call(handle)
	var code uint32
	procGetExitCodeProcessID.Call(handle, uintptr(unsafe.Pointer(&code)))
	return code == stillActive
}
