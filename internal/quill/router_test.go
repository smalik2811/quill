package quill

import (
	"testing"

	"github.com/holoplot/go-evdev"
)

func TestRouter_ToggleFiresOnCapsLockReleaseOnly(t *testing.T) {
	for _, v := range []inputVal{evPress, evRepeat} {
		r := newTestRouter()
		if _, _, changed := r.route(in(evdev.KEY_CAPSLOCK, v)); changed || r.inMovementMode {
			t.Errorf("value=%d toggled the mode", v)
		}
	}
	r := newTestRouter()
	if _, _, changed := r.route(in(evdev.KEY_CAPSLOCK, evRelease)); !changed || !r.inMovementMode {
		t.Error("release should toggle into movement mode")
	}
}

func TestRouter_TwoTogglesReturnToNormal(t *testing.T) {
	r := newTestRouter()
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.inMovementMode {
		t.Error("still in movement mode after two toggles")
	}
}

func TestRouter_ToggleKeyNeverReachesVirtualKeyboard(t *testing.T) {
	for _, v := range allValues {
		kb, _, _ := newTestRouter().route(in(evdev.KEY_CAPSLOCK, v))
		if len(kb) != 0 {
			t.Errorf("CapsLock value=%d leaked %d event(s) to the virtual keyboard", v, len(kb))
		}
	}
}

func TestRouter_NormalModePassesEverythingThroughUnchanged(t *testing.T) {
	inputs := []*evdev.InputEvent{
		in(evdev.KEY_A, evPress),
		in(evdev.KEY_A, evRepeat),
		in(evdev.KEY_A, evRelease),
		{Type: evdev.EV_MSC, Code: evdev.MSC_SCAN, Value: 30},
	}
	for _, e := range inputs {
		kb, ptr, changed := newTestRouter().route(e)
		p := payload(kb)
		if changed || len(ptr) != 0 || len(p) != 1 || p[0].Type != e.Type || p[0].Code != e.Code || p[0].Value != e.Value {
			t.Errorf("%s was not passed through unchanged: got %v", describe(e), describeAll(kb))
		}
	}
}

func TestRouter_MovementModeSwallowsUnboundKeys(t *testing.T) {
	r := newTestRouter()
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	for _, v := range allValues {
		kb, ptr, _ := r.route(in(evdev.KEY_A, v))
		if len(kb) != 0 || len(ptr) != 0 {
			t.Errorf("unbound key value=%d produced output: kb=%d ptr=%d", v, len(kb), len(ptr))
		}
	}
}

func TestRouter_MovementModeRoutesBoundKeysToPointerOnly(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_H, evdev.KEY_N, evdev.KEY_Q, evdev.KEY_SPACE} {
		r := newTestRouter()
		r.route(in(evdev.KEY_CAPSLOCK, evRelease))
		kb, _, _ := r.route(in(k, evRelease))
		if len(kb) != 0 {
			t.Errorf("%s leaked %d event(s) to the keyboard", evdev.KEYToString[k], len(kb))
		}
	}
	r := newTestRouter()
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if _, ptr, _ := r.route(in(evdev.KEY_N, evPress)); len(payload(ptr)) != 1 {
		t.Error("click did not reach the pointer")
	}
}

func TestRouter_KeyHeldAcrossModeSwitchDoesNotStick(t *testing.T) {
	r := newTestRouter()
	kbState := deviceState{}
	feed := func(c evdev.EvCode, v inputVal) {
		kb, _, _ := r.route(in(c, v))
		kbState.apply(kb)
	}
	feed(evdev.KEY_A, evPress)
	feed(evdev.KEY_CAPSLOCK, evPress)
	feed(evdev.KEY_CAPSLOCK, evRelease)
	feed(evdev.KEY_A, evRelease)
	if len(kbState) != 0 {
		t.Errorf("virtual keyboard has stuck keys: %v", kbState)
	}
}

func TestRouter_ToggleWhileModifierHeld(t *testing.T) {
	t.Skip("TODO decide: toggling out of movement mode while S or Z is held resets the pointer mode to normal?")
}
