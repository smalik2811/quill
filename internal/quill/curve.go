package quill

import (
	"math"
	"time"
)

// speedCurve maps how long a direction key has been held to a pointer speed,
// in REL units per second. It must be a pure function of its argument so it can
// be unit tested and swapped without touching the movement state.
type speedCurve func(held time.Duration) float64

// Tuning values live here so adjusting the feel is a one-line edit.
// All of them are starting points; REL units are not pixels and the right
// numbers depend on your screen and your compositor's pointer settings.
const (
	// curveStartSpeed is the speed at the instant the key goes down (units/s).
	curveStartSpeed = 60.0
	// curveMaxSpeed is the speed the curve approaches the longer the key is held (units/s).
	curveMaxSpeed = 2000.0
	// curveTimeConstant is tau: the timescale governing the acceleration ramp.
	// 650ms combined with quadratic power 2.0 keeps initial speed predictable and smooth,
	// gradually building velocity without sudden jumps across the display.
	curveTimeConstant = 650 * time.Millisecond
	// curvePower controls the initial acceleration: power = 2.0 produces quadratic acceleration
	// for smooth, highly predictable speed gain without abrupt velocity jumps.
	curvePower = 2.0

	// precisePointerSpeed is the constant speed used in precise mode (units/s).
	precisePointerSpeed = 35.0

	// maxStepInterval caps the time one event may account for, so a stall or the
	// keyboard's initial repeat delay cannot turn into a single huge jump.
	maxStepInterval = 50 * time.Millisecond
	// firstRepeatCatchUp is the larger cap used for the first repeat of a hold,
	// which partly makes up for the initial repeat delay.
	firstRepeatCatchUp = 80 * time.Millisecond
	// initialPressStep is the small fixed nudge (REL units) emitted on press so
	// the user gets immediate feedback before the first repeat arrives.
	initialPressStep int32 = 1
)

// CurveConfig holds the currently active curve parameters for inspection and graphing.
type CurveConfig struct {
	StartSpeed       float64
	MaxSpeed         float64
	TimeConstant     time.Duration
	Power            float64
	PreciseSpeed     float64
	MaxStepInterval  time.Duration
	InitialPressStep int32
}

// GetCurveConfig returns a snapshot of the current curve tuning constants.
func GetCurveConfig() CurveConfig {
	return CurveConfig{
		StartSpeed:       curveStartSpeed,
		MaxSpeed:         curveMaxSpeed,
		TimeConstant:     curveTimeConstant,
		Power:            curvePower,
		PreciseSpeed:     precisePointerSpeed,
		MaxStepInterval:  maxStepInterval,
		InitialPressStep: initialPressStep,
	}
}

// EvaluateNormalCurve evaluates the normal movement speed at a given hold duration.
func EvaluateNormalCurve(held time.Duration) float64 {
	return normalCurve(held)
}

// EvaluatePreciseCurve evaluates the precise movement speed at a given hold duration.
func EvaluatePreciseCurve(held time.Duration) float64 {
	return preciseCurve(held)
}

// normalCurve and preciseCurve are the curves the movement code uses.
var (
	normalCurve  = smoothExponential(curveStartSpeed, curveMaxSpeed, curveTimeConstant, curvePower)
	preciseCurve = constantSpeed(precisePointerSpeed)
)

// smoothExponential returns a curve with quadratic acceleration (power = 2.0): v0 at t=0, starting with
// low acceleration so speed increases slowly at first, ramping up smoothly, and
// then levelling off towards vmax. It never exceeds vmax.
//
//	v(t) = v0 + (vmax - v0) * (1 - e^(-(t/tau)^power))
func smoothExponential(v0, vmax float64, tau time.Duration, power float64) speedCurve {
	return func(held time.Duration) float64 {
		if held <= 0 {
			return v0
		}
		if tau <= 0 {
			return vmax
		}
		ratio := held.Seconds() / tau.Seconds()
		if power != 1.0 {
			ratio = math.Pow(ratio, power)
		}
		return v0 + (vmax-v0)*(1-math.Exp(-ratio))
	}
}

// exponential is the standard exponential ramp with power=1.
func exponential(v0, vmax float64, tau time.Duration) speedCurve {
	return smoothExponential(v0, vmax, tau, 1.0)
}

// constantSpeed returns a flat curve that always emits the same speed.
func constantSpeed(v float64) speedCurve {
	return func(_ time.Duration) float64 {
		return v
	}
}

// travel calculates the distance (in REL units) moved over interval dt,
// given the total duration the key has been held.
func travel(c speedCurve, held time.Duration, dt time.Duration) float64 {
	if dt <= 0 {
		return 0
	}
	speed := c(held)
	return speed * dt.Seconds()
}

// takeWhole splits a fractional distance into an integer step to emit and the
// remaining fractional travel to carry over to the next event.
func takeWhole(dist float64) (whole int32, remainder float64) {
	if dist <= 0 {
		return 0, 0
	}
	w := math.Trunc(dist)
	return int32(w), dist - w
}

// clampDuration clamps d between lo and hi inclusive.
func clampDuration(d, lo, hi time.Duration) time.Duration {
	if d < lo {
		return lo
	}
	if d > hi {
		return hi
	}
	return d
}
