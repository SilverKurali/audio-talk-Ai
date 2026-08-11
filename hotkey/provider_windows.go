//go:build windows

package hotkey

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows DLL procedures
var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	modKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookExW   = modUser32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = modUser32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = modUser32.NewProc("CallNextHookEx")
	procGetMessageW         = modUser32.NewProc("GetMessageW")
	procPostThreadMessageW  = modUser32.NewProc("PostThreadMessageW")
	procGetModuleHandleW    = modKernel32.NewProc("GetModuleHandleW")
)

const (
	whKeyboardLL = 13
	wmQuit       = 0x0012
	llkhfUp      = 0x8000
)

// windowsHookPoint mirrors the Win32 POINT structure.
type windowsHookPoint struct {
	X int32
	Y int32
}

// windowsHookMessage mirrors the Win32 MSG structure used by GetMessageW.
type windowsHookMessage struct {
	Window  windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   windowsHookPoint
	Private uint32
}

// Windows virtual key codes not in windows package.
const (
	vkControl = 0x11
	vkMenu    = 0x12 // Alt
	vkShift   = 0x10
	vkLWin    = 0x5B
	vkRWin    = 0x5C

	vkF1  = 0x70
	vkF2  = 0x71
	vkF3  = 0x72
	vkF4  = 0x73
	vkF5  = 0x74
	vkF6  = 0x75
	vkF7  = 0x76
	vkF8  = 0x77
	vkF9  = 0x78
	vkF10 = 0x79
	vkF11 = 0x7A
	vkF12 = 0x7B

	vkNumpad0 = 0x60
	vkNumpad1 = 0x61
	vkNumpad2 = 0x62
	vkNumpad3 = 0x63
	vkNumpad4 = 0x64
	vkNumpad5 = 0x65
	vkNumpad6 = 0x66
	vkNumpad7 = 0x67
	vkNumpad8 = 0x68
	vkNumpad9 = 0x69

	vkSpace   = 0x20
	vkReturn  = 0x0D
	vkBack    = 0x08
	vkTab     = 0x09
	vkEscape  = 0x1B
	vkCapital = 0x14
	vkUp      = 0x26
	vkDown    = 0x28
	vkLeft    = 0x25
	vkRight   = 0x27
	vkHome    = 0x24
	vkEnd     = 0x23
	vkPrior   = 0x21
	vkNext    = 0x22
	vkInsert  = 0x2D
	vkDelete  = 0x2E

	vkOem3      = 0xC0
	vkOemMinus  = 0xBD
	vkOemPlus   = 0xBB
	vkOem4      = 0xDB
	vkOem6      = 0xDD
	vkOem5      = 0xDC
	vkOem1      = 0xBA
	vkOem7      = 0xDE
	vkOemComma  = 0xBC
	vkOemPeriod = 0xBE
	vkOem2      = 0xBF
)

// Windows VK → unified KeyCode.
var winVKToKey = map[uint32]KeyCode{
	'A': KeyA, 'B': KeyB, 'C': KeyC, 'D': KeyD, 'E': KeyE,
	'F': KeyF, 'G': KeyG, 'H': KeyH, 'I': KeyI, 'J': KeyJ,
	'K': KeyK, 'L': KeyL, 'M': KeyM, 'N': KeyN, 'O': KeyO,
	'P': KeyP, 'Q': KeyQ, 'R': KeyR, 'S': KeyS, 'T': KeyT,
	'U': KeyU, 'V': KeyV, 'W': KeyW, 'X': KeyX, 'Y': KeyY, 'Z': KeyZ,

	'0': Key0, '1': Key1, '2': Key2, '3': Key3, '4': Key4,
	'5': Key5, '6': Key6, '7': Key7, '8': Key8, '9': Key9,

	vkNumpad0: KeyNum0, vkNumpad1: KeyNum1, vkNumpad2: KeyNum2,
	vkNumpad3: KeyNum3, vkNumpad4: KeyNum4, vkNumpad5: KeyNum5,
	vkNumpad6: KeyNum6, vkNumpad7: KeyNum7, vkNumpad8: KeyNum8,
	vkNumpad9: KeyNum9,

	vkControl: KeyCtrl, vkMenu: KeyAlt, vkShift: KeyShift,
	vkLWin: KeySuper, vkRWin: KeySuper,

	vkF1: KeyF1, vkF2: KeyF2, vkF3: KeyF3, vkF4: KeyF4,
	vkF5: KeyF5, vkF6: KeyF6, vkF7: KeyF7, vkF8: KeyF8,
	vkF9: KeyF9, vkF10: KeyF10, vkF11: KeyF11, vkF12: KeyF12,

	vkSpace: KeySpace, vkTab: KeyTab,
	vkReturn: KeyEnter, vkEscape: KeyEscape,
	vkBack: KeyBackspace, vkCapital: KeyCapsLock,
	vkUp: KeyArrowUp, vkDown: KeyArrowDown,
	vkLeft: KeyArrowLeft, vkRight: KeyArrowRight,
	vkHome: KeyHome, vkEnd: KeyEnd,
	vkPrior: KeyPageUp, vkNext: KeyPageDown,
	vkInsert: KeyInsert, vkDelete: KeyDelete,

	vkOem3: KeyBacktick, vkOemMinus: KeyMinus,
	vkOemPlus: KeyEqual, vkOem4: KeyLeftBracket,
	vkOem6: KeyRightBracket, vkOem5: KeyBackslash,
	vkOem1: KeySemicolon, vkOem7: KeyQuote,
	vkOemComma: KeyComma, vkOemPeriod: KeyPeriod,
	vkOem2: KeySlash,
}

// Global reference for the low-level keyboard hook callback.
var (
	globalWinProvider   *windowsProvider
	globalWinProviderMu sync.Mutex
)

func setGlobalWinProvider(p *windowsProvider) {
	globalWinProviderMu.Lock()
	defer globalWinProviderMu.Unlock()
	globalWinProvider = p
}

func getGlobalWinProvider() *windowsProvider {
	globalWinProviderMu.Lock()
	defer globalWinProviderMu.Unlock()
	return globalWinProvider
}

// ---- Provider ----

type windowsProvider struct {
	mu       sync.Mutex
	channels map[Combo]chan<- Event
	tracker  *KeyStateTracker
	stopped  bool

	hook     windows.Handle
	threadID uint32
	logger   *slog.Logger
}

// export windowsNewProvider
func NewProvider() (Provider, error) {
	return &windowsProvider{
		channels: make(map[Combo]chan<- Event),
		tracker:  NewKeyStateTracker(),
		logger:   slog.Default().With("platform", "windows"),
	}, nil
}

func (p *windowsProvider) Register(combo Combo) (<-chan Event, error) {
	return p.RegisterWithOptions(combo, RegisterOptions{})
}

func (p *windowsProvider) RegisterWithOptions(combo Combo, opts RegisterOptions) (<-chan Event, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil, fmt.Errorf("provider is stopped")
	}
	if _, exists := p.channels[combo]; exists {
		return nil, fmt.Errorf("hotkey %s already registered", combo)
	}

	ch := make(chan Event, 32)
	p.channels[combo] = ch
	p.tracker.Watch(combo, ch)
	return ch, nil
}

func (p *windowsProvider) Unregister(combo Combo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch, exists := p.channels[combo]
	if !exists {
		return fmt.Errorf("hotkey %s not registered", combo)
	}

	p.tracker.Unwatch(combo)
	close(ch)
	delete(p.channels, combo)
	return nil
}

func (p *windowsProvider) Start(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	setGlobalWinProvider(p)
	defer setGlobalWinProvider(nil)

	p.threadID = windows.GetCurrentThreadId()

	// Install WH_KEYBOARD_LL hook
	modHandle, _, _ := procGetModuleHandleW.Call(0)
	cb := syscall.NewCallback(windowsHookProc)
	hook, _, err := procSetWindowsHookExW.Call(
		whKeyboardLL, cb, modHandle, 0,
	)
	if hook == 0 {
		return fmt.Errorf("SetWindowsHookEx failed: %v", err)
	}
	p.hook = windows.Handle(hook)

	p.logger.Info("keyboard hook installed, starting message pump")

// Message pump
		var msg windowsHookMessage
		for {
		select {
		case <-ctx.Done():
			procUnhookWindowsHookEx.Call(uintptr(p.hook))
			return ctx.Err()
		default:
		}

		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)), 0, 0, 0,
		)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
	}

	procUnhookWindowsHookEx.Call(uintptr(p.hook))
	return ctx.Err()
}

func (p *windowsProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil
	}
	p.stopped = true

	// Unblock message pump
	procPostThreadMessageW.Call(uintptr(p.threadID), wmQuit, 0, 0)

	// Close channels
	for c, ch := range p.channels {
		close(ch)
		delete(p.channels, c)
		p.tracker.Unwatch(c)
	}

	return nil
}

func (p *windowsProvider) Info() ProviderInfo {
	return ProviderInfo{
		Platform: "windows",
		Backend:  "WH_KEYBOARD_LL",
		Features: []string{
			FeatureKeyDown, FeatureKeyUp, FeatureKeyPress,
			FeatureModifierOnly, FeatureFunctionKey, FeatureCombo,
			FeatureSuppressEvent,
		},
	}
}

// ---- Hook callback ----

func windowsHookProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 {
		p := getGlobalWinProvider()
		if p != nil {
			p.processHookEvent(wParam, lParam)
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

type kbdllhookstruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

func (p *windowsProvider) processHookEvent(wParam uintptr, lParam uintptr) {
	kbd := (*kbdllhookstruct)(unsafe.Pointer(lParam))
	key := winVKToKey[kbd.vkCode]
	if key == KeyNone {
		return
	}

	isUp := (kbd.flags & llkhfUp) != 0
	now := time.Now()

	var events []Event
	if isUp {
		events = p.tracker.KeyUp(key, now)
	} else {
		events = p.tracker.KeyDown(key, now)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, e := range events {
		if ch, ok := p.channels[e.Combo]; ok {
			select {
			case ch <- e:
			default:
			}
		}
	}
}
