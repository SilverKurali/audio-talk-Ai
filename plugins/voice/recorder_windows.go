//go:build windows

package voice

import (
	"fmt"
	"os/exec"
	"strings"
)

func pickCommand() (*exec.Cmd, string, error) {
	return firstFound([]candidate{
		// ffmpeg with dshow (most reliable on Windows)
		// Uses default audio input device
		{"ffmpeg", []string{
			"-f", "dshow", "-i", "audio=default",
			"-ar", "16000", "-ac", "1", "-f", "s16le",
			"-loglevel", "quiet", "-",
		}},
		// sox with waveaudio driver
		{"sox", []string{
			"-t", "waveaudio", "default",
			"-r", "16000", "-b", "16", "-c", "1",
			"-t", "raw", "-",
		}},
	})
}

func pickCommandWithDevice(device string) (*exec.Cmd, string, error) {
	if device == "" {
		return pickCommand()
	}
	return firstFound([]candidate{
		{"ffmpeg", []string{
			"-f", "dshow", "-i", "audio=" + device,
			"-ar", "16000", "-ac", "1", "-f", "s16le",
			"-loglevel", "quiet", "-",
		}},
		{"sox", []string{
			"-t", "waveaudio", device,
			"-r", "16000", "-b", "16", "-c", "1",
			"-t", "raw", "-",
		}},
	})
}

func errNoBackend(candidates []candidate) error {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.name
	}
	return fmt.Errorf("no recording backend found; install one of: %s", strings.Join(names, ", "))
}

// ListDevices returns available audio input devices on Windows using ffmpeg.
func ListDevices() ([]string, error) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found (needed to list devices)")
	}
	cmd := exec.Command(ff, "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, err := cmd.CombinedOutput()
	// ffmpeg's -list_devices always exits non-zero (it has no input stream to
	// process), so a non-nil error here is expected; the device list is in the
	// combined output. We only treat a truly empty output as a failure, which
	// surfaces real problems (e.g., dshow unavailable) to the UI instead of a
	// silently empty dropdown.
	devices := parseDeviceList(string(out))
	if len(devices) == 0 && err != nil && len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg failed to list devices: %w", err)
	}
	return devices, nil
}

func parseDeviceList(output string) []string {
	var devices []string
	lines := strings.Split(output, "\n")
	inAudio := false
	for _, line := range lines {
		// Legacy ffmpeg (pre-8.0) prints a "DirectShow audio devices" section
		// header; once seen we are inside the audio block until the video one.
		if strings.Contains(line, "DirectShow audio") {
			inAudio = true
			continue
		}
		if inAudio && strings.Contains(line, "DirectShow video") {
			break
		}
		// Extract a quoted device name. ffmpeg 8.0+ dropped the section header
		// and tags each line with its kind instead, e.g.
		//   [dshow @ ...] "麦克风 (Realtek(R) Audio)" (audio)
		// so a line counts as an audio device only if it is in the legacy
		// audio block OR explicitly tagged (audio) — never (video)/(none).
		isAudio := inAudio || strings.Contains(line, "(audio)")
		if !isAudio || strings.Contains(line, "(video)") || strings.Contains(line, "(none)") {
			continue
		}
		start := strings.Index(line, "\"")
		end := strings.LastIndex(line, "\"")
		if start >= 0 && end > start {
			devices = append(devices, line[start+1:end])
		}
	}
	return devices
}
