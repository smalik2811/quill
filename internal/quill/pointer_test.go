package quill

import (
	"testing"
	"time"

	"github.com/holoplot/go-evdev"
)

func TestMovement_InitialPressStep(t *testing.T) {
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
			got := newTestPointer(modeNormal).process(in(c.key, evPress))
			assertPayload(t, got, ev(evdev.EV_REL, c.code, c.sign*initialPressStep))
		})
	}

	// Precise mode skips the initial press step.
	got := newTestPointer(modePrecise).process(in(evdev.KEY_L, evPress))
	if len(got) != 0 {
		t.Fatalf("precise mode press: got %v, want nothing", describeAll(got))
	}
}

func TestMovement_TimeBasedDistanceAcrossRepeats(t *testing.T) {
	q := newTestPointer(modeNormal)
	// Press at t=0
	q.process(inAt(evdev.KEY_L, evPress, 0))

	// First repeat at t=50ms
	dt := 50 * time.Millisecond
	got := q.process(inAt(evdev.KEY_L, evRepeat, dt))
	dist := travel(normalCurve, dt, dt)
	wantWhole, _ := takeWhole(dist)
	assertPayload(t, got, ev(evdev.EV_REL, evdev.REL_X, wantWhole))

	// Second repeat at t=100ms
	got2 := q.process(inAt(evdev.KEY_L, evRepeat, 100*time.Millisecond))
	if len(payload(got2)) != 1 || got2[0].Type != evdev.EV_REL || got2[0].Code != evdev.REL_X {
		t.Fatalf("second repeat: got %v", describeAll(got2))
	}
}

func TestMovement_RemainderCarryingBetweenEvents(t *testing.T) {
	q := newTestPointer(modePrecise)
	// In precise mode (precisePointerSpeed = 35 units/s):
	// For dt = 20ms, dist = 35 * 0.020 = 0.7 units.
	// Press at t=0
	q.process(inAt(evdev.KEY_L, evPress, 0))

	// Repeat 1 at 20ms: dist = 0.7 -> whole = 0, remainder = 0.7 -> emits nothing
	got1 := q.process(inAt(evdev.KEY_L, evRepeat, 20*time.Millisecond))
	if len(got1) != 0 {
		t.Fatalf("repeat 1: got %v, want nothing (sub-unit)", describeAll(got1))
	}

	// Repeat 2 at 40ms: dt = 20ms, dist = 0.7 + 0.7 = 1.4 -> whole = 1, remainder = 0.4 -> emits 1 unit
	got2 := q.process(inAt(evdev.KEY_L, evRepeat, 40*time.Millisecond))
	assertPayload(t, got2, ev(evdev.EV_REL, evdev.REL_X, 1))

	// Repeat 3 at 60ms: dt = 20ms, dist = 0.7 + 0.4 = 1.1 -> whole = 1, remainder = 0.1 -> emits 1 unit
	got3 := q.process(inAt(evdev.KEY_L, evRepeat, 60*time.Millisecond))
	assertPayload(t, got3, ev(evdev.EV_REL, evdev.REL_X, 1))
}

func TestMovement_DtCap(t *testing.T) {
	// First repeat: dt is capped at maxStepInterval (50ms) without extra catch-up.
	q := newTestPointer(modeNormal)
	q.process(inAt(evdev.KEY_L, evPress, 0))

	// Send first repeat with dt = 250ms (simulating keyboard autorepeat delay).
	// dt should be capped at maxStepInterval.
	got1 := q.process(inAt(evdev.KEY_L, evRepeat, 250*time.Millisecond))
	wantDist1 := travel(normalCurve, 250*time.Millisecond, maxStepInterval)
	wantWhole1, _ := takeWhole(wantDist1)
	assertPayload(t, got1, ev(evdev.EV_REL, evdev.REL_X, wantWhole1))

	// Second repeat: dt is capped at maxStepInterval (50ms).
	// Send second repeat with dt = 200ms (stall).
	got2 := q.process(inAt(evdev.KEY_L, evRepeat, 450*time.Millisecond))
	wantDist2 := travel(normalCurve, 450*time.Millisecond, maxStepInterval) + (wantDist1 - float64(wantWhole1))
	wantWhole2, _ := takeWhole(wantDist2)
	assertPayload(t, got2, ev(evdev.EV_REL, evdev.REL_X, wantWhole2))
}

func TestMovement_ReleaseResettingTheState(t *testing.T) {
	q := newTestPointer(modeNormal)
	q.process(inAt(evdev.KEY_L, evPress, 0))
	q.process(inAt(evdev.KEY_L, evRepeat, 50*time.Millisecond))

	// Release of active key emits nothing and resets state.
	gotRel := q.process(inAt(evdev.KEY_L, evRelease, 60*time.Millisecond))
	if len(gotRel) != 0 {
		t.Fatalf("release emitted %v, want nothing", describeAll(gotRel))
	}
	if q.movement.activeKey != 0 {
		t.Errorf("activeKey after release = %v, want 0", q.movement.activeKey)
	}

	// Subsequent repeats are ignored because state is reset.
	gotRep := q.process(inAt(evdev.KEY_L, evRepeat, 100*time.Millisecond))
	if len(gotRep) != 0 {
		t.Fatalf("repeat after release emitted %v, want nothing", describeAll(gotRep))
	}
}

func TestMovement_RepeatsFromOtherKeysIgnored(t *testing.T) {
	q := newTestPointer(modeNormal)
	// Lone repeat with no active key is ignored.
	if got := q.process(inAt(evdev.KEY_L, evRepeat, 50*time.Millisecond)); len(got) != 0 {
		t.Fatalf("lone repeat emitted %v, want nothing", describeAll(got))
	}

	// Key H is active.
	q.process(inAt(evdev.KEY_H, evPress, 0))

	// Repeat of K, J, L must be ignored.
	for _, k := range []evdev.EvCode{evdev.KEY_J, evdev.KEY_K, evdev.KEY_L} {
		if got := q.process(inAt(k, evRepeat, 50*time.Millisecond)); len(got) != 0 {
			t.Errorf("repeat of %s while H active emitted %v", evdev.KEYToString[k], describeAll(got))
		}
	}
}

func TestMovement_ModeChangeResettingTheRamp(t *testing.T) {
	q := newTestPointer(modeNormal)
	q.process(inAt(evdev.KEY_L, evPress, 0))
	q.process(inAt(evdev.KEY_L, evRepeat, 100*time.Millisecond))

	// Pressing Ctrl changes mode to modePrecise.
	q.process(inAt(evdev.KEY_A, evPress, 150*time.Millisecond))
	if q.pointerMode != modePrecise {
		t.Fatalf("mode = %v, want precise", q.pointerMode)
	}
	if q.movement.activeKey != 0 {
		t.Errorf("activeKey after mode change = %v, want 0", q.movement.activeKey)
	}

	// Releasing Ctrl changes mode back to normal and resets movement again.
	q.process(inAt(evdev.KEY_A, evRelease, 200*time.Millisecond))
	if q.pointerMode != modeNormal {
		t.Fatalf("mode = %v, want normal", q.pointerMode)
	}
	if q.movement.activeKey != 0 {
		t.Errorf("activeKey after release = %v, want 0", q.movement.activeKey)
	}
}

func TestMovement_NonDirectionKeysNeverEmitRelXY(t *testing.T) {
	for _, mode := range allModes {
		for _, k := range allTestKeys {
			if k == evdev.KEY_H || k == evdev.KEY_J || k == evdev.KEY_K || k == evdev.KEY_L {
				continue
			}
			for _, v := range allValues {
				for _, e := range payload(newTestPointer(mode).process(in(k, v))) {
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
			got := newTestPointer(modeScroll).process(in(c.key, evRepeat))
			assertPayload(t, got, ev(evdev.EV_REL, c.code, c.val))
		})
	}
}

func TestScroll_FiresOnRepeatAndReleaseButNotPress(t *testing.T) {
	want := map[inputVal]int{evPress: 0, evRepeat: 1, evRelease: 1}
	for val, n := range want {
		got := payload(newTestPointer(modeScroll).process(in(evdev.KEY_J, val)))
		if len(got) != n {
			t.Errorf("value=%d: got %d scroll events, want %d", val, len(got), n)
		}
	}
}

func TestScroll_DirectionKeysNeverMoveCursor(t *testing.T) {
	for _, k := range directionKeys {
		for _, v := range allValues {
			for _, e := range payload(newTestPointer(modeScroll).process(in(k, v))) {
				if e.Code == evdev.REL_X || e.Code == evdev.REL_Y {
					t.Errorf("scroll mode %s value=%d emitted %s", evdev.KEYToString[k], v, describe(e))
				}
			}
		}
	}
}

func TestZoom_InIsCtrlDownWheelUpCtrlUp(t *testing.T) {
	got := newTestPointer(modeZoom).process(in(evdev.KEY_J, evRepeat))
	assertPayload(t, got,
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evPress),
		ev(evdev.EV_REL, evdev.REL_WHEEL, 1),
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evRelease),
	)
}

func TestZoom_OutIsCtrlDownWheelDownCtrlUp(t *testing.T) {
	got := newTestPointer(modeZoom).process(in(evdev.KEY_K, evRepeat))
	assertPayload(t, got,
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evPress),
		ev(evdev.EV_REL, evdev.REL_WHEEL, -1),
		ev(evdev.EV_KEY, evdev.KEY_LEFTCTRL, evRelease),
	)
}

func TestZoom_HorizontalKeysAreNoOps(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_H, evdev.KEY_L} {
		for _, v := range allValues {
			if got := newTestPointer(modeZoom).process(in(k, v)); len(got) != 0 {
				t.Errorf("Z+%s value=%d emitted %v", evdev.KEYToString[k], v, describeAll(payload(got)))
			}
		}
	}
}

func TestZoom_CtrlIsBalanced(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_J, evdev.KEY_K} {
		state := deviceState{}
		downs, ups := 0, 0
		for _, e := range payload(newTestPointer(modeZoom).process(in(k, evRepeat))) {
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

func TestClick_ButtonMapping(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{
		evdev.KEY_SPACE: evdev.BTN_MIDDLE,
		evdev.KEY_N:     evdev.BTN_LEFT,
		evdev.KEY_E:     evdev.BTN_LEFT,
		evdev.KEY_M:     evdev.BTN_RIGHT,
		evdev.KEY_R:     evdev.BTN_RIGHT,
	}
	for k, btn := range cases {
		got := newTestPointer(modeNormal).process(in(k, evPress))
		assertPayload(t, got, ev(evdev.EV_KEY, btn, evPress))
	}
}

func TestClick_PressAndReleaseMirror(t *testing.T) {
	for _, v := range []inputVal{evPress, evRelease} {
		got := newTestPointer(modeNormal).process(in(evdev.KEY_N, v))
		assertPayload(t, got, ev(evdev.EV_KEY, evdev.BTN_LEFT, v))
	}
}

func TestClick_EquivalentKeysProduceIdenticalOutput(t *testing.T) {
	pairs := [][2]evdev.EvCode{{evdev.KEY_N, evdev.KEY_E}, {evdev.KEY_M, evdev.KEY_R}}
	for _, p := range pairs {
		for _, v := range []inputVal{evPress, evRelease} {
			a := payload(newTestPointer(modeNormal).process(in(p[0], v)))
			b := payload(newTestPointer(modeNormal).process(in(p[1], v)))
			assertPayload(t, a, b...)
		}
	}
}

func TestClick_AutoRepeat(t *testing.T) {
	for k := range evKeyToClick {
		got := newTestPointer(modeNormal).process(in(k, evRepeat))
		if len(got) != 0 {
			t.Errorf("%s on repeat emitted %v, want nothing", evdev.KEYToString[k], describeAll(payload(got)))
		}
	}
}

func TestClick_WhileAnotherModeIsActive(t *testing.T) {
	for _, mode := range []pointerMode{modeScroll, modeZoom} {
		for k := range evKeyToClick {
			for _, v := range []inputVal{evPress, evRelease} {
				got := newTestPointer(mode).process(in(k, v))
				if len(got) != 0 {
					t.Errorf("mode=%s key=%s value=%d emitted %v, want ignored",
						mode, evdev.KEYToString[k], v, describeAll(payload(got)))
				}
			}
		}
	}

	for _, mode := range []pointerMode{modeNormal, modePrecise} {
		for k, click := range evKeyToClick {
			got := newTestPointer(mode).process(in(k, evPress))
			assertPayload(t, got, ev(evdev.EV_KEY, click.code, evPress))
		}
	}
}

func TestNavigation_Mapping(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{evdev.KEY_Q: evdev.BTN_SIDE, evdev.KEY_W: evdev.BTN_EXTRA}
	for k, btn := range cases {
		for _, e := range payload(newTestPointer(modeNormal).process(in(k, evRelease))) {
			if e.Code != btn {
				t.Errorf("%s emitted %s, want button %d", evdev.KEYToString[k], describe(e), btn)
			}
		}
	}
}

func TestNavigation_MirrorsPressAndRelease(t *testing.T) {
	cases := map[evdev.EvCode]evdev.EvCode{evdev.KEY_Q: evdev.BTN_SIDE, evdev.KEY_W: evdev.BTN_EXTRA}
	for k, btn := range cases {
		for _, v := range []inputVal{evPress, evRelease} {
			got := newTestPointer(modeNormal).process(in(k, v))
			assertPayload(t, payload(got), ev(evdev.EV_KEY, btn, v))
		}
	}
}

func TestNavigation_RepeatEmitsNothing(t *testing.T) {
	for _, k := range []evdev.EvCode{evdev.KEY_Q, evdev.KEY_W} {
		if got := newTestPointer(modeNormal).process(in(k, evRepeat)); len(got) != 0 {
			t.Errorf("%s repeat emitted %v", evdev.KEYToString[k], describeAll(payload(got)))
		}
	}
}

func TestMode_PressEntersModeAndReleaseLeavesIt(t *testing.T) {
	cases := map[evdev.EvCode]pointerMode{
		evdev.KEY_S: modeScroll, evdev.KEY_Z: modeZoom, evdev.KEY_A: modePrecise,
	}
	for k, want := range cases {
		q := newTestPointer(modeNormal)
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
	q := newTestPointer(modeNormal)
	q.process(in(evdev.KEY_S, evPress))
	for range 5 {
		q.process(in(evdev.KEY_S, evRepeat))
	}
	if q.pointerMode != modeScroll {
		t.Errorf("mode=%s after repeats, want scroll", q.pointerMode)
	}
}

func TestMode_NonModifierKeysNeverChangeMode(t *testing.T) {
	for _, k := range allTestKeys {
		if k == evdev.KEY_S || k == evdev.KEY_Z || k == evdev.KEY_A {
			continue
		}
		for _, mode := range allModes {
			for _, v := range allValues {
				q := newTestPointer(mode)
				q.process(in(k, v))
				if q.pointerMode != mode {
					t.Errorf("%s value=%d changed mode %s -> %s", evdev.KEYToString[k], v, mode, q.pointerMode)
				}
			}
		}
	}
}

func TestMode_ReleasingADifferentModifierKeepsActiveMode(t *testing.T) {
	q := newTestPointer(modeNormal)
	q.process(in(evdev.KEY_S, evPress))
	q.process(in(evdev.KEY_Z, evPress))
	q.process(in(evdev.KEY_Z, evRelease))
	if q.pointerMode != modeScroll {
		t.Errorf("S is still held but mode=%s, want scroll=%s", q.pointerMode, modeScroll)
	}
}
