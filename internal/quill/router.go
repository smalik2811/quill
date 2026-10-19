package quill

import (
	"slices"
	"time"

	"github.com/holoplot/go-evdev"
)

//go:generate stringer -type=routeTarget
type routeTarget int

const (
	targetKeyboard routeTarget = iota
	targetPointer
)

const pointerInactivityTimeout = 10 * time.Second

type router struct {
	pointerDevice       *quillPointer
	keyboardDevice      *quillKeyboard
	target              routeTarget
	lastEventTime       time.Time
	now                 func() time.Time
	activeKeyTarget     map[evdev.EvCode]routeTarget
}

func createRouter(pointerDevice *quillPointer, keyboardDevice *quillKeyboard) *router {
	return &router{
		pointerDevice:   pointerDevice,
		keyboardDevice:  keyboardDevice,
		target:          targetKeyboard,
		activeKeyTarget: make(map[evdev.EvCode]routeTarget),
	}
}

func (r *router) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// isPointerModeInterceptedKey returns true if the key is managed by Quill's pointer mode:
// A-Z keys and KEY_LEFTCTRL.
func isPointerModeInterceptedKey(code evdev.EvCode) bool {
	// In pointer mode, only letter keys A-Z are intercepted.
	// KEY_Q(16) through KEY_P(25)
	// KEY_A(30) through KEY_L(38)
	// KEY_Z(44) through KEY_M(50)
	if (code >= evdev.KEY_Q && code <= evdev.KEY_P) ||
		(code >= evdev.KEY_A && code <= evdev.KEY_L) ||
		(code >= evdev.KEY_Z && code <= evdev.KEY_M) {
		return true
	}
	return false
}

// isModifierKey returns true for keys that modify regular keystrokes:
// - Windows/Super & Mac Command: KEY_LEFTMETA, KEY_RIGHTMETA
// - Alt & Mac Option: KEY_LEFTALT, KEY_RIGHTALT, KEY_OPTION
// - Shift: KEY_LEFTSHIFT, KEY_RIGHTSHIFT
// - Ctrl: KEY_LEFTCTRL, KEY_RIGHTCTRL
// - Escape: KEY_ESC
// - Compose / Globe: KEY_COMPOSE
// - Fn keys & Apple/ThinkPad Fn shortcuts: KEY_FN, KEY_FN_*
func isModifierKey(code evdev.EvCode) bool {
	switch code {
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA, // Windows / Super / Mac Command
		evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT, // Alt / Mac Option
		evdev.KEY_OPTION,                          // Explicit Mac Option key code in evdev
		evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT, // Shift
		evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL, // Control
		evdev.KEY_ESC,     // Escape
		evdev.KEY_COMPOSE, // Compose / Mac Globe key
		evdev.KEY_FN, evdev.KEY_FN_ESC,
		evdev.KEY_FN_F1, evdev.KEY_FN_F2, evdev.KEY_FN_F3, evdev.KEY_FN_F4,
		evdev.KEY_FN_F5, evdev.KEY_FN_F6, evdev.KEY_FN_F7, evdev.KEY_FN_F8,
		evdev.KEY_FN_F9, evdev.KEY_FN_F10, evdev.KEY_FN_F11, evdev.KEY_FN_F12,
		evdev.KEY_FN_1, evdev.KEY_FN_2, evdev.KEY_FN_D, evdev.KEY_FN_E,
		evdev.KEY_FN_F, evdev.KEY_FN_S, evdev.KEY_FN_B, evdev.KEY_FN_RIGHT_SHIFT:
		return true
	default:
		return false
	}
}

// hasHeldModifier returns true if any modifier key (Super/Command, Alt/Option, Shift, Right Ctrl, Esc, Fn, Globe)
// is currently held on the virtual keyboard device. Note that Left Ctrl is intentionally
// excluded from this check if held in pointer mode because Left Ctrl activates pointer precision mode.
func (r *router) hasHeldModifier() bool {
	for _, code := range r.keyboardDevice.heldKeys {
		if isModifierKey(code) {
			return true
		}
	}
	return false
}

func (r *router) route(iEvent *evdev.InputEvent) {
	now := r.currentTime()
	if r.target == targetPointer && !r.lastEventTime.IsZero() && now.Sub(r.lastEventTime) >= pointerInactivityTimeout {
		r.pointerDevice.flush()
		r.pointerDevice.reset()
		r.activeKeyTarget = make(map[evdev.EvCode]routeTarget)
		r.target = targetKeyboard
	}
	r.lastEventTime = now

	// CapsLock is the dedicated router switcher key.
	if iEvent.Type == evdev.EV_KEY && iEvent.Code == evdev.KEY_CAPSLOCK {
		if iEvent.Value != evRelease {
			return
		}

		switch r.target {
		case targetKeyboard:
			r.keyboardDevice.flush()
			r.target = targetPointer
			r.activeKeyTarget = make(map[evdev.EvCode]routeTarget)
		case targetPointer:
			r.pointerDevice.flush()
			r.target = targetKeyboard
			r.activeKeyTarget = make(map[evdev.EvCode]routeTarget)
		}
		r.pointerDevice.reset()
		return
	}

	if r.target == targetPointer {
		if iEvent.Type == evdev.EV_KEY {
			// If key is currently active (already pressed), route repeats and releases
			// to the exact same device that handled the press.
			if dest, ok := r.activeKeyTarget[iEvent.Code]; ok {
				if iEvent.Value == evRelease {
					delete(r.activeKeyTarget, iEvent.Code)
				}
				if dest == targetPointer {
					r.pointerDevice.handleInputEvent(iEvent)
				} else {
					r.keyboardDevice.handleInputEvent(iEvent)
				}
				return
			}

			// For new press events:
			// In pointer mode, letter keys A-Z route to pointer UNLESS a modifier
			// (Super, Alt, Shift, Ctrl, Fn, etc.) is held down on the keyboard.
			dest := targetKeyboard
			if isPointerModeInterceptedKey(iEvent.Code) && !r.hasHeldModifier() {
				dest = targetPointer
			}

			if iEvent.Value == evPress {
				r.activeKeyTarget[iEvent.Code] = dest
			}

			if dest == targetPointer {
				r.pointerDevice.handleInputEvent(iEvent)
			} else {
				r.keyboardDevice.handleInputEvent(iEvent)
			}
			return
		}

		r.keyboardDevice.handleInputEvent(iEvent)
		return
	}

	// Keyboard mode: route to keyboard and clear pointer tracking
	if iEvent.Type == evdev.EV_KEY {
		if iEvent.Value == evRelease {
			delete(r.activeKeyTarget, iEvent.Code)
		} else if iEvent.Value == evPress {
			r.activeKeyTarget[iEvent.Code] = targetKeyboard
		}
	}
	r.keyboardDevice.handleInputEvent(iEvent)
}

// Suppress unused import warnings if needed
var _ = slices.Backward[[]int]
