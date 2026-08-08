//go:build darwin && cgo

package voice

// #cgo LDFLAGS: -framework AudioToolbox -framework CoreAudio
// #include "audioqueue_darwin.h"
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type darwinAudioReadCloser struct {
	file *os.File
	rec  *C.jt_audio_recorder_t
}

func (r *darwinAudioReadCloser) Read(p []byte) (int, error) {
	return r.file.Read(p)
}

func (r *darwinAudioReadCloser) Close() error {
	if r.rec != nil {
		C.jt_audio_stop(r.rec)
		r.rec = nil
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// deviceNameToUID caches the mapping from user-friendly device names to
// CoreAudio device UIDs. Populated by ListDevices(), used by startCaptureWithDevice.
var (
	deviceNameToUID map[string]string
	deviceMapMu     sync.RWMutex
)

func startCaptureWithDevice(logger *slog.Logger, device string) (io.ReadCloser, string, func() error, error) {
	readFile, writeFile, err := os.Pipe()
	if err != nil {
		return nil, "", nil, err
	}
	cWriteFD, err := syscall.Dup(int(writeFile.Fd()))
	if err != nil {
		_ = readFile.Close()
		_ = writeFile.Close()
		return nil, "", nil, err
	}

	// Resolve device name to CoreAudio UID if a device was configured.
	var deviceUIDC *C.char
	var deviceUIDGo string
	if device != "" {
		deviceMapMu.RLock()
		uid, ok := deviceNameToUID[device]
		deviceMapMu.RUnlock()
		if ok {
			deviceUIDGo = uid
		} else {
			// Not found in cache; use the configured value directly as UID.
			// This covers cases where a user manually configured a UID.
			deviceUIDGo = device
		}
		deviceUIDC = C.CString(deviceUIDGo)
		defer C.free(unsafe.Pointer(deviceUIDC))
	}

	var rec *C.jt_audio_recorder_t
	rc := int(C.jt_audio_start(C.int(cWriteFD), deviceUIDC, &rec))
	_ = writeFile.Close()
	if rc != 0 {
		_ = syscall.Close(cWriteFD)
		_ = readFile.Close()
		return nil, "", nil, fmt.Errorf("AudioQueue start failed: %d", rc)
	}

	closer := &darwinAudioReadCloser{file: readFile, rec: rec}
	stop := func() error { return closer.Close() }
	backendName := "coreaudio"
	if deviceUIDGo != "" {
		backendName = "coreaudio:" + deviceUIDGo
	}
	return closer, backendName, stop, nil
}

// ListDevices returns available audio input device names on macOS.
// It queries CoreAudio via C and returns user-friendly display names.
func ListDevices() ([]string, error) {
	cStr := C.jt_audio_list_devices()
	if cStr == nil {
		return nil, fmt.Errorf("failed to enumerate audio input devices")
	}
	defer C.free(unsafe.Pointer(cStr))

	output := C.GoString(cStr)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var names []string
	newMap := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "display-name\tuid"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		uid := strings.TrimSpace(parts[1])
		if name == "" || uid == "" {
			continue
		}
		names = append(names, name)
		newMap[name] = uid
	}

	// Atomically update the cache
	deviceMapMu.Lock()
	deviceNameToUID = newMap
	deviceMapMu.Unlock()

	return names, nil
}
