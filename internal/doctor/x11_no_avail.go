//go:build linux && no_x11

package doctor

// x11BackendAvailable reports whether the X11 hotkey backend was compiled in.
// It is false under the no_x11 build tag; doctor uses it to avoid advertising
// X11-only dependencies (xclip) when the corresponding code path is disabled.
const x11BackendAvailable = false
