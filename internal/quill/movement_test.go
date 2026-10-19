package quill

import (
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

	var ep EdgeDistanceProvider = client
	if ep != nil {
		// Even if wrapped as a typed nil interface, it should not panic
		dist, dim, ok := ep.DistanceToEdge(evdev.KEY_L)
		if ok || dist != 0 || dim != 0 {
			t.Errorf("typed nil provider DistanceToEdge = (%v, %v, %v), want (0, 0, false)", dist, dim, ok)
		}
	}
}
