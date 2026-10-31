package quill

import (
	"math"
	"testing"
	"time"

	"github.com/holoplot/go-evdev"
)

type mockEdgeProvider struct {
	distance float64
	totalDim float64
	ok       bool
}

func (m *mockEdgeProvider) DistanceToEdge(_ evdev.EvCode) (float64, float64, bool) {
	return m.distance, m.totalDim, m.ok
}

type mockScreenScaleProvider struct {
	mockEdgeProvider
	scale float64
}

func (m *mockScreenScaleProvider) ScreenScaleFactor() float64 {
	return m.scale
}

func TestCalculateFriction_FarFromEdgeHasZeroFriction(t *testing.T) {
	totalDim := 1440.0
	// 50% from edge (> 35% edgeZone)
	dist := 720.0
	held := 1 * time.Second

	f := calculateFriction(dist, totalDim, held)
	if f != 0 {
		t.Errorf("calculateFriction(%v, %v, %v) = %v, want 0", dist, totalDim, held, f)
	}
}

func TestCalculateFriction_ZeroHeldDurationHasZeroFriction(t *testing.T) {
	totalDim := 1440.0
	dist := 10.0 // right at the edge
	held := 0 * time.Millisecond

	f := calculateFriction(dist, totalDim, held)
	if f != 0 {
		t.Errorf("calculateFriction at held=0 = %v, want 0", f)
	}
}

func TestCalculateFriction_IncreasesAsDistanceDecreases(t *testing.T) {
	totalDim := 1440.0
	held := 400 * time.Millisecond // full duration factor

	fFar := calculateFriction(400, totalDim, held) // within 35% zone (504px)
	fMid := calculateFriction(200, totalDim, held)
	fClose := calculateFriction(20, totalDim, held)

	if !(fFar < fMid && fMid < fClose) {
		t.Errorf("expected monotonic increase as distance shrinks: got %v, %v, %v", fFar, fMid, fClose)
	}
}

func TestCalculateFriction_IncreasesAsHeldDurationIncreases(t *testing.T) {
	totalDim := 1440.0
	dist := 50.0 // close to the edge

	fShort := calculateFriction(dist, totalDim, 50*time.Millisecond)
	fMedium := calculateFriction(dist, totalDim, 200*time.Millisecond)
	fLong := calculateFriction(dist, totalDim, 500*time.Millisecond)

	if !(fShort < fMedium && fMedium < fLong) {
		t.Errorf("expected friction to build up over duration: got %v, %v, %v", fShort, fMedium, fLong)
	}
}

func TestCalculateFriction_BoundedByMaxFriction(t *testing.T) {
	totalDim := 1440.0
	dist := 0.0 // at the boundary
	held := 10 * time.Second

	f := calculateFriction(dist, totalDim, held)
	const maxAllowed = 0.40
	if f > maxAllowed+1e-6 {
		t.Errorf("friction exceeded max: got %v, want <= %v", f, maxAllowed)
	}
}

func TestMovement_AppliesFrictionNearScreenEdge(t *testing.T) {
	// Pointer 1: without edge friction (or far from edge)
	qFree := &quillPointer{
		pointerMode: modeNormal,
		movement: movementState{
			edgeProvider: &mockEdgeProvider{distance: 1000, totalDim: 1440, ok: true},
		},
	}
	qFree.process(inAt(evdev.KEY_L, evPress, 0))
	gotFree := qFree.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))

	// Pointer 2: near the edge
	qEdge := &quillPointer{
		pointerMode: modeNormal,
		movement: movementState{
			edgeProvider: &mockEdgeProvider{distance: 20, totalDim: 1440, ok: true},
		},
	}
	qEdge.process(inAt(evdev.KEY_L, evPress, 0))
	gotEdge := qEdge.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))

	pFree := payload(gotFree)
	pEdge := payload(gotEdge)

	if len(pFree) != 1 || len(pEdge) != 1 {
		t.Fatalf("expected 1 event each, got %v and %v", len(pFree), len(pEdge))
	}

	valFree := pFree[0].Value
	valEdge := pEdge[0].Value

	if valEdge >= valFree {
		t.Errorf("expected edge movement (%v) to be less than free movement (%v)", valEdge, valFree)
	}
}

func TestHyprlandClient_NilClientDoesNotPanic(t *testing.T) {
	var client *HyprlandClient
	dist, dim, ok := client.DistanceToEdge(evdev.KEY_L)
	if ok || dist != 0 || dim != 0 {
		t.Errorf("nil client DistanceToEdge = (%v, %v, %v), want (0, 0, false)", dist, dim, ok)
	}

	// Even if wrapped as a typed nil interface, it should not panic
	var ep EdgeDistanceProvider = client
	dist, dim, ok = ep.DistanceToEdge(evdev.KEY_L)
	if ok || dist != 0 || dim != 0 {
		t.Errorf("typed nil provider DistanceToEdge = (%v, %v, %v), want (0, 0, false)", dist, dim, ok)
	}

	if sf := client.ScreenScaleFactor(); sf != 1.0 {
		t.Errorf("nil client ScreenScaleFactor = %v, want 1.0", sf)
	}
}

func TestMovement_AppliesScreenResolutionScaling(t *testing.T) {
	// Screen 1: baseline 1080p (scale = 1.0)
	q1080p := &quillPointer{
		pointerMode: modeNormal,
		movement: movementState{
			edgeProvider: &mockScreenScaleProvider{
				mockEdgeProvider: mockEdgeProvider{distance: 1000, totalDim: 1920, ok: true},
				scale:            1.0,
			},
		},
	}
	q1080p.process(inAt(evdev.KEY_L, evPress, 0))
	got1080p := q1080p.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))

	// Screen 2: 768p laptop screen (scale = 0.71)
	q768p := &quillPointer{
		pointerMode: modeNormal,
		movement: movementState{
			edgeProvider: &mockScreenScaleProvider{
				mockEdgeProvider: mockEdgeProvider{distance: 1000, totalDim: 1366, ok: true},
				scale:            0.71,
			},
		},
	}
	q768p.process(inAt(evdev.KEY_L, evPress, 0))
	got768p := q768p.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))

	p1080p := payload(got1080p)
	p768p := payload(got768p)

	if len(p1080p) != 1 || len(p768p) != 1 {
		t.Fatalf("expected 1 event each, got %v and %v", len(p1080p), len(p768p))
	}

	if p768p[0].Value >= p1080p[0].Value {
		t.Errorf("expected 768p movement (%v) to scale down relative to 1080p (%v)", p768p[0].Value, p1080p[0].Value)
	}
}

func TestMovement_PreciseModeIgnoresScreenScaling(t *testing.T) {
	// In precise mode, movement speed should remain constant regardless of screen scale
	qPreciseBase := &quillPointer{
		pointerMode: modePrecise,
		movement: movementState{
			edgeProvider: &mockScreenScaleProvider{scale: 1.0},
		},
	}
	qPreciseBase.process(inAt(evdev.KEY_L, evPress, 0))
	gotBase := qPreciseBase.process(inAt(evdev.KEY_L, evRepeat, 50*time.Millisecond))

	qPreciseScaled := &quillPointer{
		pointerMode: modePrecise,
		movement: movementState{
			edgeProvider: &mockScreenScaleProvider{scale: 2.0},
		},
	}
	qPreciseScaled.process(inAt(evdev.KEY_L, evPress, 0))
	gotScaled := qPreciseScaled.process(inAt(evdev.KEY_L, evRepeat, 50*time.Millisecond))

	pBase := payload(gotBase)
	pScaled := payload(gotScaled)

	if len(pBase) != len(pScaled) {
		t.Fatalf("precise mode event count mismatch: %d vs %d", len(pBase), len(pScaled))
	}
	if len(pBase) > 0 && pBase[0].Value != pScaled[0].Value {
		t.Errorf("precise mode should not be affected by screen scale: got %v vs %v", pBase[0].Value, pScaled[0].Value)
	}
}

func TestMovement_TickerEmitsLiveEventsAndDropsHardwareRepeat(t *testing.T) {
	ptrDev, ptrReader := newPipeDevice()
	defer ptrReader.Close()
	defer ptrDev.file.Close()

	qp := &quillPointer{
		virtualDevice: virtualDevice{
			eventWriter: ptrDev,
			heldKeys:    []evdev.EvCode{},
		},
		pointerMode: modeNormal,
	}
	defer qp.close()

	// 1. Press key - emits initialPressStep and starts ticker
	qp.handleInputEvent(in(evdev.KEY_L, evPress))

	// Initial step should be emitted immediately
	time.Sleep(5 * time.Millisecond)
	initialEvents := readDeviceEvents(ptrReader)
	if len(payload(initialEvents)) == 0 {
		t.Fatalf("expected initial press event, got none")
	}

	// 2. Hardware evRepeat should be dropped by handleInputEvent while ticker is active
	qp.handleInputEvent(in(evdev.KEY_L, evRepeat))
	_ = readDeviceEvents(ptrReader)

	// 3. Wait for internal ticker (several 10ms intervals)
	time.Sleep(35 * time.Millisecond)
	tickerEvents := readDeviceEvents(ptrReader)
	if len(payload(tickerEvents)) == 0 {
		t.Errorf("expected asynchronous ticker events while holding, got none")
	}

	// 4. Release key - stops ticker
	qp.handleInputEvent(in(evdev.KEY_L, evRelease))
	if qp.movement.activeKey != 0 {
		t.Errorf("expected activeKey to reset to 0 on release")
	}
}

func TestMovement_SimultaneousDiagonalChords(t *testing.T) {
	cases := []struct {
		name      string
		xKey      evdev.EvCode
		yKey      evdev.EvCode
		wantSignX int32
		wantSignY int32
	}{
		{"Up-Right (K+L)", evdev.KEY_L, evdev.KEY_K, 1, -1},
		{"Down-Right (J+L)", evdev.KEY_L, evdev.KEY_J, 1, 1},
		{"Up-Left (K+H)", evdev.KEY_H, evdev.KEY_K, -1, -1},
		{"Down-Left (J+H)", evdev.KEY_H, evdev.KEY_J, -1, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := newTestPointer(modeNormal)
			// Press X and Y keys simultaneously
			q.process(inAt(c.xKey, evPress, 0))
			q.process(inAt(c.yKey, evPress, 0))

			// Repeat step
			got := q.process(inAt(c.xKey, evRepeat, 100*time.Millisecond))
			p := payload(got)

			if len(p) != 2 {
				t.Fatalf("expected 2 relative events (REL_X and REL_Y), got %d: %v", len(p), describeAll(p))
			}

			var hasX, hasY bool
			for _, e := range p {
				if e.Code == evdev.REL_X {
					hasX = true
					if (e.Value > 0 && c.wantSignX < 0) || (e.Value < 0 && c.wantSignX > 0) {
						t.Errorf("REL_X sign mismatch: got %d, want sign %d", e.Value, c.wantSignX)
					}
				}
				if e.Code == evdev.REL_Y {
					hasY = true
					if (e.Value > 0 && c.wantSignY < 0) || (e.Value < 0 && c.wantSignY > 0) {
						t.Errorf("REL_Y sign mismatch: got %d, want sign %d", e.Value, c.wantSignY)
					}
				}
			}

			if !hasX || !hasY {
				t.Errorf("expected both X and Y relative events in payload: got %v", describeAll(p))
			}
		})
	}
}

func TestMovement_DiagonalVectorNormalization(t *testing.T) {
	// Cardinal motion (L only): full horizontal distance
	qCardinal := newTestPointer(modeNormal)
	qCardinal.process(inAt(evdev.KEY_L, evPress, 0))
	gotCardinal := qCardinal.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))
	valCardinal := payload(gotCardinal)[0].Value

	// Diagonal motion (L + K): each axis scaled by 1/sqrt(2) ~ 0.7071
	qDiag := newTestPointer(modeNormal)
	qDiag.process(inAt(evdev.KEY_L, evPress, 0))
	qDiag.process(inAt(evdev.KEY_K, evPress, 0))
	gotDiag := qDiag.process(inAt(evdev.KEY_L, evRepeat, 300*time.Millisecond))
	pDiag := payload(gotDiag)

	if len(pDiag) != 2 {
		t.Fatalf("expected 2 events for diagonal motion, got %d", len(pDiag))
	}

	valDiagX := float64(pDiag[0].Value)
	expectedX := float64(valCardinal) * (1.0 / math.Sqrt2)

	// Verify diagonal X component is normalized within integer rounding tolerance
	if math.Abs(valDiagX-expectedX) > 2.0 {
		t.Errorf("diagonal X value %v not normalized relative to cardinal %v (expected ~%v)", valDiagX, valCardinal, expectedX)
	}
}

func TestMovement_DiagonalReleaseOneAxisContinuesOther(t *testing.T) {
	q := newTestPointer(modeNormal)
	// Press both X and Y
	q.process(inAt(evdev.KEY_L, evPress, 0))
	q.process(inAt(evdev.KEY_K, evPress, 0))

	// Release X (L)
	q.process(inAt(evdev.KEY_L, evRelease, 100*time.Millisecond))

	// Next repeat on Y (K)
	got := q.process(inAt(evdev.KEY_K, evRepeat, 200*time.Millisecond))
	p := payload(got)

	// Should now only emit REL_Y
	if len(p) != 1 || p[0].Code != evdev.REL_Y {
		t.Fatalf("expected single REL_Y event after releasing X axis, got: %v", describeAll(p))
	}

	// Release Y (K)
	q.process(inAt(evdev.KEY_K, evRelease, 300*time.Millisecond))
	if q.movement.isMoving() {
		t.Errorf("expected isMoving to be false after releasing both axes")
	}
}
