package quill

import (
	"math"
	"testing"
	"time"
)

func TestCurve_SpeedAtZeroEqualsStartSpeed(t *testing.T) {
	if got := normalCurve(0); got != curveStartSpeed {
		t.Errorf("normalCurve(0) = %v, want %v", got, curveStartSpeed)
	}
	v0 := 50.0
	vmax := 500.0
	tau := 200 * time.Millisecond
	c := exponential(v0, vmax, tau)
	if got := c(0); got != v0 {
		t.Errorf("exponential curve at t=0 = %v, want %v", got, v0)
	}
	if got := c(-10 * time.Millisecond); got != v0 {
		t.Errorf("exponential curve at t<0 = %v, want %v", got, v0)
	}
}

func TestCurve_NeverDecreasesAndNeverExceedsMax(t *testing.T) {
	prev := normalCurve(0)
	for ms := 1; ms <= 5000; ms += 5 {
		held := time.Duration(ms) * time.Millisecond
		curr := normalCurve(held)
		if curr < prev {
			t.Fatalf("normalCurve decreased at %v: %v < %v", held, curr, prev)
		}
		if curr > curveMaxSpeed {
			t.Fatalf("normalCurve exceeded max at %v: %v > %v", held, curr, curveMaxSpeed)
		}
		prev = curr
	}
}

func TestCurve_QuadraticRampProperties(t *testing.T) {
	cfg := GetCurveConfig()
	if cfg.Power != 2.0 {
		t.Errorf("expected quadratic curvePower = 2.0, got %v", cfg.Power)
	}

	// Verify that speed grows smoothly without sudden step jumps
	s0 := EvaluateNormalCurve(0)
	s150 := EvaluateNormalCurve(150 * time.Millisecond)
	s300 := EvaluateNormalCurve(300 * time.Millisecond)
	s600 := EvaluateNormalCurve(600 * time.Millisecond)

	if !(s0 < s150 && s150 < s300 && s300 < s600) {
		t.Errorf("expected strictly increasing quadratic ramp: s0=%v, s150=%v, s300=%v, s600=%v", s0, s150, s300, s600)
	}

	// Ensure micro-control at 150ms is gentle (< 300 units/s)
	if s150 > 300.0 {
		t.Errorf("initial speed at 150ms is too aggressive for quadratic ramp: %v", s150)
	}
}

func TestCurve_TakeWholeKeepsRemainderAndCarries(t *testing.T) {
	cases := []struct {
		dist      float64
		wantWhole int32
		wantRem   float64
	}{
		{0.0, 0, 0.0},
		{0.75, 0, 0.75},
		{1.0, 1, 0.0},
		{3.25, 3, 0.25},
		{5.99, 5, 0.99},
	}
	for _, c := range cases {
		whole, rem := takeWhole(c.dist)
		if whole != c.wantWhole || math.Abs(rem-c.wantRem) > 1e-9 {
			t.Errorf("takeWhole(%v) = (%v, %v), want (%v, %v)", c.dist, whole, rem, c.wantWhole, c.wantRem)
		}
	}

	// Carrying: 0.6 + 0.6 = 1.2 -> whole 1, rem 0.2
	w1, r1 := takeWhole(0.6)
	if w1 != 0 || math.Abs(r1-0.6) > 1e-9 {
		t.Errorf("step 1 = (%v, %v), want (0, 0.6)", w1, r1)
	}
	w2, r2 := takeWhole(r1 + 0.6)
	if w2 != 1 || math.Abs(r2-0.2) > 1e-9 {
		t.Errorf("step 2 = (%v, %v), want (1, 0.2)", w2, r2)
	}
}

func TestCurve_ClampDurationClampsBothEnds(t *testing.T) {
	lo := 10 * time.Millisecond
	hi := 50 * time.Millisecond

	if got := clampDuration(5*time.Millisecond, lo, hi); got != lo {
		t.Errorf("clampDuration below lo: got %v, want %v", got, lo)
	}
	if got := clampDuration(100*time.Millisecond, lo, hi); got != hi {
		t.Errorf("clampDuration above hi: got %v, want %v", got, hi)
	}
	if got := clampDuration(30*time.Millisecond, lo, hi); got != 30*time.Millisecond {
		t.Errorf("clampDuration in range: got %v, want %v", got, 30*time.Millisecond)
	}
}

func TestCurve_TravelReturnsZeroForDtNonPositive(t *testing.T) {
	if got := travel(normalCurve, 100*time.Millisecond, 0); got != 0 {
		t.Errorf("travel(dt=0) = %v, want 0", got)
	}
	if got := travel(normalCurve, 100*time.Millisecond, -10*time.Millisecond); got != 0 {
		t.Errorf("travel(dt<0) = %v, want 0", got)
	}
	if got := travel(preciseCurve, 100*time.Millisecond, 0); got != 0 {
		t.Errorf("travel precise(dt=0) = %v, want 0", got)
	}
	if got := travel(preciseCurve, 100*time.Millisecond, -5*time.Millisecond); got != 0 {
		t.Errorf("travel precise(dt<0) = %v, want 0", got)
	}
}
