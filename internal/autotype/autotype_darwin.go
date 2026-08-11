//go:build darwin && cgo

package autotype

// #cgo LDFLAGS: -framework ApplicationServices
//
// #include <ApplicationServices/ApplicationServices.h>
// #include <unistd.h>
//
// static void cgevent_cmd_v(void) {
// 	CGEventRef cmdDown = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, true);  // kVK_Command
// 	CGEventRef vDown   = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, true);   // kVK_ANSI_V
// 	CGEventRef vUp     = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, false);
// 	CGEventRef cmdUp   = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)55, false);
//
// 	CGEventSetFlags(vDown, kCGEventFlagMaskCommand);
// 	CGEventSetFlags(vUp, kCGEventFlagMaskCommand);
//
// 	CGEventPost(kCGSessionEventTap, cmdDown);
// 	usleep(15000);
// 	CGEventPost(kCGSessionEventTap, vDown);
// 	usleep(30000);
// 	CGEventPost(kCGSessionEventTap, vUp);
// 	usleep(15000);
// 	CGEventPost(kCGSessionEventTap, cmdUp);
//
// 	CFRelease(cmdDown); CFRelease(vDown); CFRelease(vUp); CFRelease(cmdUp);
// }
import "C"

import (
	"fmt"
	"log/slog"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/internal/clipboard"
)

func pastePlatform(text string, logger *slog.Logger) error {
	cb, err := clipboard.New()
	if err != nil {
		return fmt.Errorf("clipboard: %w", err)
	}
	// Save the user's current clipboard so it can be restored after the paste.
	// macOS pasteboard is global state, so auto-submit must not clobber
	// whatever the user was keeping there. Match the Linux X11 and Windows
	// behavior.
	orig, _ := cb.Get()
	if err := cb.Set(text); err != nil {
		return fmt.Errorf("set clipboard: %w", err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := simulatePaste(); err != nil {
		return fmt.Errorf("simulate paste: %w", err)
	}
	// Give the target app time to read the pasteboard during Cmd+V, then
	// restore the original contents. Skip if there was nothing to restore or
	// if the original already equals the transcribed text.
	time.Sleep(300 * time.Millisecond)
	if orig != "" && orig != text {
		_ = cb.Set(orig)
	}
	logger.Debug("autotype done", "text_len", len(text), "method", pasteMethod(), "restored_clipboard", orig != "" && orig != text)
	return nil
}

func simulatePaste() error {
	C.cgevent_cmd_v()
	return nil
}

func pasteMethod() string { return "darwin/CGEventPost+Cmd+V" }

func isWaylandSession() bool { return false }
