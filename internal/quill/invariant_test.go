package quill

import (
	"math/rand"
	"testing"

	"github.com/holoplot/go-evdev"
)

// forEveryInput runs check on every (mode, key, value) combination.
// Many combinations legitimately emit nothing (e.g. a movement key press), and
// the invariants hold trivially for those. The guard at the end makes sure the
// loop is not passing vacuously because process() stopped emitting altogether.
func forEveryInput(t *testing.T, check func(t *testing.T, mode pointerMode, k evdev.EvCode, v inputVal, out []*evdev.InputEvent)) {
	t.Helper()
	emitting := 0
	for _, mode := range allModes {
		for _, k := range allTestKeys {
			for _, v := range allValues {
				out := newTestPointer(mode, 5).process(in(k, v))
				if len(out) > 0 {
					emitting++
				}
				check(t, mode, k, v, out)
			}
		}
	}
	if emitting == 0 {
		t.Fatal("no input produced any output; this invariant is passing vacuously")
	}
}

// uinput silently drops codes the device never declared.
func TestInvariant_EveryEmittedEventIsDeclaredByThePointer(t *testing.T) {
	forEveryInput(t, func(t *testing.T, mode pointerMode, k evdev.EvCode, v inputVal, out []*evdev.InputEvent) {
		for _, e := range payload(out) {
			if e.Type == evdev.EV_KEY && e.Code == evdev.KEY_LEFTCTRL {
				continue // TODO: zoom Ctrl should go to the virtual keyboard; remove this once it is routed there
			}
			if !isDeclaredByPointer(e) {
				t.Errorf("mode=%s key=%s value=%d emitted undeclared %s", mode, evdev.KEYToString[k], v, describe(e))
			}
		}
	})
}

func TestInvariant_EveryEventIsFollowedBySyncReport(t *testing.T) {
	forEveryInput(t, func(t *testing.T, mode pointerMode, k evdev.EvCode, v inputVal, out []*evdev.InputEvent) {
		if len(out)%2 != 0 {
			t.Fatalf("mode=%s key=%s value=%d: odd event count %d", mode, evdev.KEYToString[k], v, len(out))
		}
		for i := 0; i < len(out); i += 2 {
			if out[i].Type == evdev.EV_SYN || out[i+1].Type != evdev.EV_SYN || out[i+1].Code != evdev.SYN_REPORT {
				t.Fatalf("mode=%s key=%s value=%d: event %d is not followed by SYN_REPORT", mode, evdev.KEYToString[k], v, i/2)
			}
		}
	})
}

func TestInvariant_OutputValuesAreLegal(t *testing.T) {
	forEveryInput(t, func(t *testing.T, mode pointerMode, k evdev.EvCode, v inputVal, out []*evdev.InputEvent) {
		for _, e := range payload(out) {
			switch e.Type {
			case evdev.EV_KEY:
				if e.Value != evPress && e.Value != evRelease {
					t.Errorf("mode=%s key=%s value=%d: %s (EV_KEY must be 0 or 1)", mode, evdev.KEYToString[k], v, describe(e))
				}
			case evdev.EV_REL:
				if e.Value == 0 {
					t.Errorf("mode=%s key=%s value=%d: %s (EV_REL delta must not be 0)", mode, evdev.KEYToString[k], v, describe(e))
				}
			}
		}
	})
}

func TestInvariant_UnboundKeysEmitNothingInAnyMode(t *testing.T) {
	for _, mode := range allModes {
		for _, k := range []evdev.EvCode{evdev.KEY_A, evdev.KEY_ESC} {
			for _, v := range allValues {
				if out := newTestPointer(mode, 5).process(in(k, v)); len(out) != 0 {
					t.Errorf("mode=%s key=%s value=%d emitted %v", mode, evdev.KEYToString[k], v, describeAll(payload(out)))
				}
			}
		}
	}
}

func TestInvariant_NonKeyEventsAreIgnoredByThePointer(t *testing.T) {
	for _, mode := range allModes {
		for _, e := range []*evdev.InputEvent{
			{Type: evdev.EV_REL, Code: evdev.REL_X, Value: 3},
			{Type: evdev.EV_MSC, Code: evdev.MSC_SCAN, Value: 30},
			{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT},
		} {
			if out := newTestPointer(mode, 5).process(e); len(out) != 0 {
				t.Errorf("mode=%s %s produced output", mode, describe(e))
			}
		}
	}
}

// Randomised legal press/release sequences; once everything is let go, neither
// virtual device may have a key or button still down.
func TestInvariant_NoStuckKeysAfterRandomSequences(t *testing.T) {
	keys := []evdev.EvCode{
		evdev.KEY_A, evdev.KEY_H, evdev.KEY_N, evdev.KEY_Q,
		evdev.KEY_S, evdev.KEY_Z, evdev.KEY_LEFTCTRL, evdev.KEY_CAPSLOCK,
	}
	rng := rand.New(rand.NewSource(1))
	for range 2000 {
		r := newTestRouter()
		kbState, ptrState := deviceState{}, deviceState{}
		held := map[evdev.EvCode]bool{}
		var trace []string

		step := func(c evdev.EvCode, v inputVal) {
			arrow := "↓"
			if v == evRelease {
				arrow = "↑"
			}
			trace = append(trace, evdev.KEYToString[c]+arrow)
			kb, ptr, _ := r.route(in(c, v))
			kbState.apply(kb)
			ptrState.apply(ptr)
		}

		for range 12 {
			c := keys[rng.Intn(len(keys))]
			if held[c] {
				step(c, evRelease)
				delete(held, c)
			} else {
				step(c, evPress)
				held[c] = true
			}
		}
		for c := range held {
			step(c, evRelease)
		}

		if len(kbState) != 0 || len(ptrState) != 0 {
			t.Fatalf("stuck keys after %v\n  keyboard=%v pointer=%v", trace, kbState, ptrState)
		}
	}
}

func TestInvariant_ArbitraryInputNeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	r := newTestRouter()
	for range 20000 {
		e := &evdev.InputEvent{
			Type:  evdev.EvType(rng.Intn(0x20)),
			Code:  evdev.EvCode(rng.Intn(0x300)),
			Value: int32(rng.Intn(5) - 1),
		}
		r.route(e)
	}
}

// Derived from the binding tables, so a new binding without a matching
// capability fails here instead of silently doing nothing.
func TestConfig_DeclaredCapabilitiesCoverEveryBoundCode(t *testing.T) {
	declared := func(typ evdev.EvType, code evdev.EvCode) bool {
		return isDeclaredByPointer(&evdev.InputEvent{Type: typ, Code: code})
	}
	for _, m := range evKeyToTranslation {
		if !declared(evdev.EV_REL, m.code) {
			t.Errorf("translation REL code %d is not declared", m.code)
		}
	}
	for _, m := range evKeyToScroll {
		if !declared(evdev.EV_REL, m.code) {
			t.Errorf("scroll REL code %d is not declared", m.code)
		}
	}
	for _, m := range evKeyToZoom {
		if !declared(evdev.EV_REL, m.code) {
			t.Errorf("zoom REL code %d is not declared", m.code)
		}
	}
	for _, m := range evKeyToClick {
		if !declared(evdev.EV_KEY, m.code) {
			t.Errorf("click button %d is not declared", m.code)
		}
	}
	for _, m := range evKeyToNavigation {
		if !declared(evdev.EV_KEY, m.code) {
			t.Errorf("navigation button %d is not declared", m.code)
		}
	}
}

func TestConfig_NormalSpeedExceedsPreciseSpeed(t *testing.T) {
	if !(defaultMotionSpeed > preciseMotionSpeed) {
		t.Errorf("precise mode has no effect: default step %d, precise step %d", defaultMotionSpeed, preciseMotionSpeed)
	}
}
