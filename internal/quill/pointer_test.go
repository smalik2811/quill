package quill

import (
	"testing"

	"github.com/holoplot/go-evdev"
)

func TestMovement_DirectionMapping(t *testing.T) {
	cases := []struct {
		key  evdev.EvCode
		code evdev.EvCode
		sign int32
	}{
		{evdev.KEY_H, evdev.REL_X, -1},
		{evdev.KEY_L, evdev.REL_X, 1},
		{evdev.KEY_K, evdev.REL_Y, -1},
		{evdev.KEY_J, evdev.REL_Y, 1},
	}
	for _, c := range cases {
		t.Run(evdev.KEYToString[c.key], func(t *testing.T) {
			got := newTestPointer(modeNormal, 5).process(in(c.key, evRepeat))
			assertPayload(t, got, ev(evdev.EV_REL, c.code, c.sign*5))
		})
	}
}

func TestMovement_StepSizeEqualsMotionSpeed(t *testing.T) {
	for _, speed := range []int32{1, 3, 10} {
		got := newTestPointer(modeNormal, speed).process(in(evdev.KEY_L, evRepeat))
		assertPayload(t, got, ev(evdev.EV_REL, evdev.REL_X, speed))
	}
}

func TestMovement_PreciseStepIgnoresMotionSpeed(t *testing.T) {
	got := newTestPointer(modePrecise, 10).process(in(evdev.KEY_L, evRepeat))
	assertPayload(t, got, ev(evdev.EV_REL, evdev.REL_X, preciseMotionSpeed))
}

func TestMovement_FiresOnRepeatAndReleaseButNotPress(t *testing.T) {
	want := map[inputVal]int{evPress: 0, evRepeat: 1, evRelease: 1}
	for val, n := range want {
		got := payload(newTestPointer(modeNormal, 5).process(in(evdev.KEY_H, val)))
		if len(got) != n {
			t.Errorf("value=%d: got %d movement events, want %d", val, len(got), n)
		}
	}
}

func TestMovement_DiagonalHeldKeysEmitSeparateSyncedEvents(t *testing.T) {
	q := newTestPointer(modeNormal, 5)

	// H and K are both held, so the keyboard sends a repeat for each one.
	left := q.process(in(evdev.KEY_H, evRepeat))
	up := q.process(in(evdev.KEY_K, evRepeat))

	// Each key produces exactly one axis event followed by its own SYN_REPORT.
	for name, got := range map[string][]*evdev.InputEvent{"H": left, "K": up} {
		if len(got) != 2 || got[0].Type != evdev.EV_REL ||
			got[1].Type != evdev.EV_SYN || got[1].Code != evdev.SYN_REPORT {
			t.Errorf("%s: want [axis event, SYN_REPORT], got %v", name, describeAll(got))
		}
	}

	assertPayload(t, left, ev(evdev.EV_REL, evdev.REL_X, -5))
	assertPayload(t, up, ev(evdev.EV_REL, evdev.REL_Y, -5))
}

func TestMovement_NonDirectionKeysNeverEmitRelXY(t *testing.T) {
	for _, mode := range allModes {
		for _, k := range allTestKeys {
			if k == evdev.KEY_H || k == evdev.KEY_J || k == evdev.KEY_K || k == evdev.KEY_L {
				continue
			}
			for _, v := range allValues {
				for _, e := range payload(newTestPointer(mode, 5).process(in(k, v))) {
					if e.Type == evdev.EV_REL && (e.Code == evdev.REL_X || e.Code == evdev.REL_Y) {
						t.Errorf("mode=%d key=%s value=%d emitted %s", mode, evdev.KEYToString[k], v, describe(e))
					}
				}
			}
		}
	}
}

func TestScroll_DirectionMapping(t *testing.T) {
	cases := []struct {
		key  evdev.EvCode
		code evdev.EvCode
		val  int32
	}{
		{evdev.KEY_H, evdev.REL_HWHEEL, 1},
		{evdev.KEY_L, evdev.REL_HWHEEL, -1},
		{evdev.KEY_J, evdev.REL_WHEEL, -1},
		{evdev.KEY_K, evdev.REL_WHEEL, 1},
	}
	for _, c := range cases {
		t.Run(evdev.KEYToString[c.key], func(t *testing.T) {
			got := newTestPointer(modeScroll, 5).process(in(c.key, evRepeat))
			assertPayload(t, got, ev(evdev.EV_REL, c.code, c.val))
		})
	}
}

func TestScroll_FiresOnRepeatAndReleaseButNotPress(t *testing.T) {
	want := map[inputVal]int{evPress: 0, evRepeat: 1, evRelease: 1}
	for val, n := range want {
		got := payload(newTestPointer(modeScroll, 5).process(in(evdev.KEY_J, val)))
		if len(got) != n {
			t.Errorf("value=%d: got %d scroll events, want %d", val, len(got), n)
		}
	}
}

func TestScroll_DirectionKeysNeverMoveCursor(t *testing.T) {
	for _, k := range directionKeys {
		for _, v := range allValues {
			for _, e := range payload(newTestPointer(modeScroll, 5).process(in(k, v))) {
				if e.Code == evdev.REL_X || e.Code == evdev.REL_Y {
					t.Errorf("scroll mode %s value=%d emitted %s", evdev.KEYToString[k], v, describe(e))
				}
			}
		}
	}
}

func TestZoom_InIsCtrlDownWheelUpCtrlUp(t *testing.T) {
	got := newTestPointer(modeZoom, 5).process(in(evdev.KEY_J, evRepeat))
	assertPayload(t, got,
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evPress),
		ev(evdev.EV_REL, evdev.REL_WHEEL, 1),
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evRelease),
	)
}

func TestZoom_OutIsCtrlDownWheelDownCtrlUp(t *testing.T) {
	got := newTestPointer(modeZoom, 5).process(in(evdev.KEY_K, evRepeat))
	assertPayload(t, got,
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evPress),
		ev(evdev.EV_REL, evdev.REL_WHEEL, -1),
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evRelease),
	)
}

func TestZoom_HorizontalKeysAreNoOps(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_H, evdev.KEY_L} {
		for _, v := range allValues {
			if got := newTestPointer(modeZoom, 5).process(in(k, v)); len(got) != 0 {
				t.Errorf("Z+%s value=%d emitted %v", evdev.KEYToString[k], v, describeAll(payload(got)))
			}
		}
	}
}

func TestZoom_CtrlIsBalanced(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_J, evdev.KEY_K} {
		state := deviceState{}
		downs, ups := 0, 0
		for _, e := range payload(newTestPointer(modeZoom, 5).process(in(k, evRepeat))) {
			if e.Type == evdev.EV_KEY && e.Code == evdev.KEY_LEFTCTRL {
				switch e.Value {
				case evPress:
					downs++
				case evRelease:
					ups++
				}
				state.apply([]*evdev.InputEvent{e})
			}
		}
		if downs != ups || len(state) != 0 {
			t.Errorf("%s: ctrl downs=%d ups=%d, still held=%v", evdev.KEYToString[k], downs, ups, state)
		}
	}
}

func TestZoom_CtrlIsRoutedToVirtualKeyboard(t *testing.T) {
	t.Skip("TODO needs outputs that carry a target device (keyboard vs pointer); today everything goes to the pointer")
}

func TestClick_ButtonMapping(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{
		evdev.KEY_N: evdev.BTN_LEFT, evdev.KEY_E: evdev.BTN_LEFT,
		evdev.KEY_M: evdev.BTN_RIGHT, evdev.KEY_R: evdev.BTN_RIGHT,
		evdev.KEY_SPACE: evdev.BTN_MIDDLE,
	}
	for k, btn := range cases {
		got := newTestPointer(modeNormal, 5).process(in(k, evPress))
		assertPayload(t, got, ev(evdev.EV_KEY, btn, evPress))
	}
}

func TestClick_PressAndReleaseMirror(t *testing.T) {
	for _, v := range []inputVal{evPress, evRelease} {
		got := newTestPointer(modeNormal, 5).process(in(evdev.KEY_N, v))
		assertPayload(t, got, ev(evdev.EV_KEY, evdev.BTN_LEFT, v))
	}
}

func TestClick_EquivalentKeysProduceIdenticalOutput(t *testing.T) {
	pairs := [][2]evdev.EvCode{{evdev.KEY_N, evdev.KEY_E}, {evdev.KEY_M, evdev.KEY_R}}
	for _, p := range pairs {
		for _, v := range []inputVal{evPress, evRelease} {
			a := payload(newTestPointer(modeNormal, 5).process(in(p[0], v)))
			b := payload(newTestPointer(modeNormal, 5).process(in(p[1], v)))
			assertPayload(t, a, b...)
		}
	}
}

func TestClick_AutoRepeat(t *testing.T) {
	t.Skip("TODO decide: are held-key repeats (value 2) dropped or forwarded to the button?")
}

func TestClick_WhileAnotherModeIsActive(t *testing.T) {
	t.Skip("TODO decide: does N still click while S, Z or Ctrl is held, or is it ignored?")
}

func TestNavigation_Mapping(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{evdev.KEY_Q: evdev.BTN_BACK, evdev.KEY_W: evdev.BTN_FORWARD}
	for k, btn := range cases {
		for _, e := range payload(newTestPointer(modeNormal, 5).process(in(k, evRelease))) {
			if e.Code != btn {
				t.Errorf("%s emitted %s, want button %d", evdev.KEYToString[k], describe(e), btn)
			}
		}
	}
}

func TestNavigation_MirrorsPressAndRelease(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{evdev.KEY_Q: evdev.BTN_BACK, evdev.KEY_W: evdev.BTN_FORWARD}
	for k, btn := range cases {
		for _, v := range []inputVal{evPress, evRelease} {
			got := newTestPointer(modeNormal, 5).process(in(k, v))
			assertPayload(t, payload(got), ev(evdev.EV_KEY, btn, v))
		}
	}
}

func TestNavigation_RepeatEmitsNothing(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_Q, evdev.KEY_W} {
		if got := newTestPointer(modeNormal, 5).process(in(k, evRepeat)); len(got) != 0 {
			t.Errorf("%s repeat emitted %v", evdev.KEYToString[k], describeAll(payload(got)))
		}
	}
}

func TestMode_PressEntersModeAndReleaseLeavesIt(t *testing.T) {
	cases := map[evdev.EvCode]pointerMode{
		evdev.KEY_S: modeScroll, evdev.KEY_Z: modeZoom, evdev.KEY_LEFTCTRL: modePrecise,
	}
	for k, want := range cases {
		q := newTestPointer(modeNormal, 5)
		q.process(in(k, evPress))
		if q.pointerMode != want {
			t.Errorf("%s press: mode=%s want %s", evdev.KEYToString[k], q.pointerMode, want)
		}
		q.process(in(k, evRelease))
		if q.pointerMode != modeNormal {
			t.Errorf("%s release: mode=%s want normal", evdev.KEYToString[k], q.pointerMode)
		}
	}
}

func TestMode_RepeatOfHeldModifierDoesNotChangeMode(t *testing.T) {
	q := newTestPointer(modeNormal, 5)
	q.process(in(evdev.KEY_S, evPress))
	for _ = range 5 {
		q.process(in(evdev.KEY_S, evRepeat))
	}
	if q.pointerMode != modeScroll {
		t.Errorf("mode=%s after repeats, want scroll", q.pointerMode)
	}
}

func TestMode_NonModifierKeysNeverChangeMode(t *testing.T) {
	for _, k := range allTestKeys {
		if k == evdev.KEY_S || k == evdev.KEY_Z || k == evdev.KEY_LEFTCTRL || k == evdev.KEY_RIGHTCTRL {
			continue
		}
		for _, mode := range allModes {
			for _, v := range allValues {
				q := newTestPointer(mode, 5)
				q.process(in(k, v))
				if q.pointerMode != mode {
					t.Errorf("%s value=%d changed mode %s -> %s", evdev.KEYToString[k], v, mode, q.pointerMode)
				}
			}
		}
	}
}

func TestMode_ReleasingADifferentModifierKeepsActiveMode(t *testing.T) {
	q := newTestPointer(modeNormal, 5)
	q.process(in(evdev.KEY_S, evPress))
	q.process(in(evdev.KEY_Z, evPress))
	q.process(in(evdev.KEY_Z, evRelease))
	if q.pointerMode != modeScroll {
		t.Errorf("S is still held but mode=%s, want scroll=%s", q.pointerMode, modeScroll)
	}
}
