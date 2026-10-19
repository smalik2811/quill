package quill

import (
	"slices"
	"testing"
	"time"

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

func TestRouter_MovementModeSwallowsUnboundLetters(t *testing.T) {
	r := newTestRouter()
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	for _, v := range allValues {
		kb, ptr, _ := r.route(in(evdev.KEY_T, v))
		if len(kb) != 0 || len(ptr) != 0 {
			t.Errorf("unbound letter key value=%d produced output: kb=%d ptr=%d", v, len(kb), len(ptr))
		}
	}
}

func TestRouter_MovementModeRoutesBoundKeysToPointerOnly(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_H, evdev.KEY_N, evdev.KEY_Q} {
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

func TestRouter_NonLetterKeysPassThroughToKeyboardInAnyMode(t *testing.T) {
	nonLetterKeys := []evdev.EvCode{
		evdev.KEY_SPACE,
		evdev.KEY_ENTER,
		evdev.KEY_TAB,
		evdev.KEY_BACKSPACE,
		evdev.KEY_ESC,
		evdev.KEY_LEFTMETA,
		evdev.KEY_RIGHTMETA,
		evdev.KEY_LEFTSHIFT,
		evdev.KEY_RIGHTSHIFT,
		evdev.KEY_LEFTALT,
		evdev.KEY_RIGHTALT,
		evdev.KEY_1,
		evdev.KEY_F1,
		evdev.KEY_UP,
		evdev.KEY_DOWN,
		evdev.KEY_LEFT,
		evdev.KEY_RIGHT,
	}

	for _, k := range nonLetterKeys {
		for _, v := range allValues {
			r := newTestRouter()
			// Enter pointer mode
			r.route(in(evdev.KEY_CAPSLOCK, evRelease))
			if r.target != targetPointer {
				t.Fatalf("failed to enter pointer mode")
			}

			// Route non-letter key
			kb, ptr, _ := r.route(in(k, v))
			p := payload(kb)
			if len(p) != 1 || p[0].Code != k || p[0].Value != v {
				t.Errorf("key %v value=%v in pointer mode did not pass through to keyboard: %v", k, v, describeAll(p))
			}
			if len(payload(ptr)) != 0 {
				t.Errorf("key %v leaked to pointer in pointer mode: %v", k, describeAll(payload(ptr)))
			}
		}
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

func TestRouter_TogglingWithKeysHeldEmitsReleasesInReversePressOrder(t *testing.T) {
	r := newTestRouter()
	// Press A then B in keyboard mode
	r.route(in(evdev.KEY_A, evPress))
	r.route(in(evdev.KEY_B, evPress))

	// Toggle CapsLock to enter pointer mode
	kb, _, _ := r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	p := payload(kb)

	want := []*evdev.InputEvent{
		ev(evdev.EV_KEY, evdev.KEY_B, evRelease),
		ev(evdev.EV_KEY, evdev.KEY_A, evRelease),
	}
	assertPayload(t, p, want...)
}

func TestRouter_SecondToggleEmitsNoDuplicateReleases(t *testing.T) {
	r := newTestRouter()
	r.route(in(evdev.KEY_1, evPress))

	// First toggle into pointer mode: emits release of A
	kb1, _, _ := r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if len(payload(kb1)) != 1 {
		t.Fatalf("first toggle got %v, want 1 release", describeAll(payload(kb1)))
	}

	// Second toggle back to keyboard mode: should emit nothing
	kb2, ptr2, _ := r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if len(payload(kb2)) != 0 || len(payload(ptr2)) != 0 {
		t.Fatalf("second toggle emitted events: kb=%v ptr=%v", describeAll(payload(kb2)), describeAll(payload(ptr2)))
	}

	// Third toggle into pointer mode again: heldKeys was cleared, so emit nothing
	kb3, ptr3, _ := r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if len(payload(kb3)) != 0 || len(payload(ptr3)) != 0 {
		t.Fatalf("third toggle emitted events: kb=%v ptr=%v", describeAll(payload(kb3)), describeAll(payload(ptr3)))
	}
}

func TestRouter_PointerReturnsToNormalModeAfterSwitch(t *testing.T) {
	r := newTestRouter()
	// Toggle into pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))

	// Enter scroll mode
	r.route(in(evdev.KEY_S, evPress))
	if r.pointerDevice.pointerMode != modeScroll {
		t.Fatalf("pointer mode = %v, want scroll", r.pointerDevice.pointerMode)
	}

	// Toggle back to keyboard mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.pointerDevice.pointerMode != modeNormal {
		t.Errorf("pointer mode after leaving pointer = %v, want normal", r.pointerDevice.pointerMode)
	}

	// Toggle into pointer mode again
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.pointerDevice.pointerMode != modeNormal {
		t.Errorf("pointer mode after entering pointer = %v, want normal", r.pointerDevice.pointerMode)
	}
}

func TestRouter_MovementStateClearedAfterSwitch(t *testing.T) {
	r := newTestRouter()
	// Toggle into pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))

	// Press a movement key
	r.route(inAt(evdev.KEY_L, evPress, 0))
	if r.pointerDevice.movement.activeKey != evdev.KEY_L {
		t.Fatalf("activeKey = %v, want KEY_L", r.pointerDevice.movement.activeKey)
	}

	// Toggle back to keyboard mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.pointerDevice.movement.activeKey != 0 {
		t.Errorf("activeKey after switch = %v, want 0", r.pointerDevice.movement.activeKey)
	}

	// Toggle into pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.pointerDevice.movement.activeKey != 0 {
		t.Errorf("activeKey after re-entering pointer = %v, want 0", r.pointerDevice.movement.activeKey)
	}
}

func TestRouter_PointerModeTimesOutAfterInactivity(t *testing.T) {
	r := newTestRouter()
	cur := time.Unix(1000, 0)
	r.router.now = func() time.Time { return cur }

	// Enter pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.target != targetPointer {
		t.Fatalf("target = %v, want targetPointer", r.target)
	}

	// Inactivity for 11 seconds (exceeding pointerInactivityTimeout)
	cur = cur.Add(11 * time.Second)

	// Send KEY_A press (keyboard key)
	kb, ptr, _ := r.route(in(evdev.KEY_1, evPress))

	// Verify target was restored to targetKeyboard
	if r.target != targetKeyboard {
		t.Errorf("target after 11s inactivity = %v, want targetKeyboard", r.target)
	}

	// Verify event was routed to keyboard, not pointer
	if len(payload(kb)) != 1 || payload(kb)[0].Code != evdev.KEY_1 {
		t.Errorf("expected KEY_1 on virtual keyboard, got: %v", describeAll(payload(kb)))
	}
	if len(payload(ptr)) != 0 {
		t.Errorf("expected no events on pointer, got: %v", describeAll(payload(ptr)))
	}
}

func TestRouter_PointerModeDoesNotTimeOutBeforeInactivity(t *testing.T) {
	r := newTestRouter()
	cur := time.Unix(1000, 0)
	r.router.now = func() time.Time { return cur }

	// Enter pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))

	// Inactivity for 9 seconds (under 10s timeout)
	cur = cur.Add(9 * time.Second)

	// Send KEY_H press (pointer movement key)
	kb, ptr, _ := r.route(in(evdev.KEY_H, evPress))

	// Should still be in pointer mode
	if r.target != targetPointer {
		t.Errorf("target after 9s inactivity = %v, want targetPointer", r.target)
	}
	if len(payload(kb)) != 0 {
		t.Errorf("expected no events on keyboard, got: %v", describeAll(payload(kb)))
	}
	if len(payload(ptr)) == 0 {
		t.Error("expected pointer movement event on pointer device, got none")
	}
}

func TestRouter_PointerModeTimeoutFlushesAndResetsPointer(t *testing.T) {
	r := newTestRouter()
	cur := time.Unix(1000, 0)
	r.router.now = func() time.Time { return cur }

	// Enter pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))

	// Enter scroll mode
	r.route(in(evdev.KEY_S, evPress))
	if r.pointerDevice.pointerMode != modeScroll {
		t.Fatalf("pointerMode = %v, want modeScroll", r.pointerDevice.pointerMode)
	}

	// Inactivity for 10.5 seconds
	cur = cur.Add(10500 * time.Millisecond)

	// Send an event
	r.route(in(evdev.KEY_B, evPress))

	// Should have reset pointer mode back to normal and restored keyboard target
	if r.target != targetKeyboard {
		t.Errorf("target = %v, want targetKeyboard", r.target)
	}
	if r.pointerDevice.pointerMode != modeNormal {
		t.Errorf("pointerMode = %v, want modeNormal", r.pointerDevice.pointerMode)
	}
}

func TestRouter_CapsLockAfterTimeoutRestoresPointerMode(t *testing.T) {
	r := newTestRouter()
	cur := time.Unix(1000, 0)
	r.router.now = func() time.Time { return cur }

	// Enter pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.target != targetPointer {
		t.Fatalf("target = %v, want targetPointer", r.target)
	}

	// 15 seconds pass
	cur = cur.Add(15 * time.Second)

	// User presses CapsLock
	r.route(in(evdev.KEY_CAPSLOCK, evPress))
	if r.target != targetKeyboard {
		t.Errorf("target after CapsLock press timeout = %v, want targetKeyboard", r.target)
	}

	// User releases CapsLock (50ms later)
	cur = cur.Add(50 * time.Millisecond)
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.target != targetPointer {
		t.Errorf("target after CapsLock release = %v, want targetPointer", r.target)
	}
}

// Suppress unused import warning if needed
var _ = slices.Backward[[]int]

func TestRouter_ModifierCombosRouteToKeyboardInPointerMode(t *testing.T) {
	r := newTestRouter()
	// Enter pointer mode
	r.route(in(evdev.KEY_CAPSLOCK, evRelease))
	if r.target != targetPointer {
		t.Fatalf("failed to enter pointer mode")
	}

	// 1. Press Super (KEY_LEFTMETA)
	kb, ptr, _ := r.route(in(evdev.KEY_LEFTMETA, evPress))
	if len(payload(kb)) != 1 || len(payload(ptr)) != 0 {
		t.Fatalf("Super press did not route to keyboard: kb=%v ptr=%v", describeAll(payload(kb)), describeAll(payload(ptr)))
	}

	// 2. While Super is down, press F (KEY_F is a letter key, normally intercepted)
	kb, ptr, _ = r.route(in(evdev.KEY_F, evPress))
	if len(payload(kb)) != 1 || payload(kb)[0].Code != evdev.KEY_F || len(payload(ptr)) != 0 {
		t.Errorf("Super+F did not route F to keyboard: kb=%v ptr=%v", describeAll(payload(kb)), describeAll(payload(ptr)))
	}

	// 3. Super repeats
	kb, ptr, _ = r.route(in(evdev.KEY_LEFTMETA, evRepeat))
	if len(payload(kb)) != 1 || len(payload(ptr)) != 0 {
		t.Errorf("Super repeat did not route to keyboard: kb=%v ptr=%v", describeAll(payload(kb)), describeAll(payload(ptr)))
	}

	// 4. Release F
	kb, ptr, _ = r.route(in(evdev.KEY_F, evRelease))
	if len(payload(kb)) != 1 || payload(kb)[0].Code != evdev.KEY_F {
		t.Errorf("F release did not route to keyboard: kb=%v", describeAll(payload(kb)))
	}

	// 5. Release Super
	kb, ptr, _ = r.route(in(evdev.KEY_LEFTMETA, evRelease))
	if len(payload(kb)) != 1 || payload(kb)[0].Code != evdev.KEY_LEFTMETA {
		t.Errorf("Super release did not route to keyboard: kb=%v", describeAll(payload(kb)))
	}

	// 6. After Super is released, pressing a pointer navigation key (e.g. H) routes to pointer!
	kb, ptr, _ = r.route(in(evdev.KEY_H, evPress))
	if len(payload(kb)) != 0 {
		t.Errorf("H leaked to keyboard after Super was released: kb=%v", describeAll(payload(kb)))
	}
	if len(payload(ptr)) == 0 {
		t.Errorf("H did not reach pointer after Super was released")
	}
}

func TestRouter_MacKeysRouteToKeyboardInAnyMode(t *testing.T) {
	macModifiers := []evdev.EvCode{
		evdev.KEY_LEFTMETA,  // Left Command
		evdev.KEY_RIGHTMETA, // Right Command
		evdev.KEY_LEFTALT,   // Left Option
		evdev.KEY_RIGHTALT,  // Right Option
		evdev.KEY_OPTION,    // Explicit Option
		evdev.KEY_COMPOSE,   // Globe key
		evdev.KEY_FN,        // Mac Fn key
	}

	for _, mod := range macModifiers {
		r := newTestRouter()
		// Enter pointer mode
		r.route(in(evdev.KEY_CAPSLOCK, evRelease))
		if r.target != targetPointer {
			t.Fatalf("failed to enter pointer mode")
		}

		// Press Mac modifier
		kb, ptr, _ := r.route(in(mod, evPress))
		if len(payload(kb)) != 1 || len(payload(ptr)) != 0 {
			t.Fatalf("Mac modifier %v did not reach keyboard: kb=%v ptr=%v", mod, describeAll(payload(kb)), describeAll(payload(ptr)))
		}

		// Press letter key (e.g. KEY_C for Cmd+C copy)
		kb, ptr, _ = r.route(in(evdev.KEY_C, evPress))
		if len(payload(kb)) != 1 || payload(kb)[0].Code != evdev.KEY_C || len(payload(ptr)) != 0 {
			t.Fatalf("Mac combo %v+C did not reach keyboard: kb=%v ptr=%v", mod, describeAll(payload(kb)), describeAll(payload(ptr)))
		}
	}
}
