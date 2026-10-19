package quill

import (
	"log"
	"syscall"
	"time"

	"github.com/holoplot/go-evdev"
)

var debugMovement = false

type movementState struct {
	activeKey    evdev.EvCode
	pressTime    time.Duration
	lastTime     time.Duration
	remainder    float64
	edgeProvider EdgeDistanceProvider
}

func (m *movementState) reset() {
	m.activeKey = 0
	m.pressTime = 0
	m.lastTime = 0
	m.remainder = 0
}

func timevalToDuration(tv syscall.Timeval) time.Duration {
	return time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
}

// CalculateFriction evaluates the edge friction damping for inspection or plotting.
func CalculateFriction(distance, totalDim float64, held time.Duration) float64 {
	return calculateFriction(distance, totalDim, held)
}

// calculateFriction computes a friction damping factor in [0, maxFriction].
//   - As distance to the edge decreases within the edge zone (outer 35% of the screen),
//     the proximity factor P increases from 0 towards 1.
//   - As the hold duration increases, the duration factor T builds up smoothly towards 1.
func calculateFriction(distance, totalDim float64, held time.Duration) float64 {
	if totalDim <= 0 || distance < 0 {
		return 0
	}

	edgeZone := totalDim * 0.35
	if edgeZone <= 0 {
		return 0
	}

	var p float64
	if distance < edgeZone {
		p = (edgeZone - distance) / edgeZone
	}

	const frictionRampTime = 400 * time.Millisecond
	t := float64(held) / float64(frictionRampTime)
	if t > 1.0 {
		t = 1.0
	} else if t < 0 {
		t = 0
	}

	const maxFriction = 0.40
	return maxFriction * p * t
}

func (m *movementState) handle(mode pointerMode, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	translation, ok := evKeyToTranslation[iEvent.Code]
	if !ok {
		return nil
	}

	t := timevalToDuration(iEvent.Time)

	switch iEvent.Value {
	case evPress:
		m.reset()
		m.activeKey = iEvent.Code
		m.pressTime = t
		m.lastTime = t

		if mode == modePrecise {
			return nil
		}

		if initialPressStep == 0 {
			return nil
		}

		return []*evdev.InputEvent{
			{
				Type:  evdev.EV_REL,
				Code:  translation.code,
				Value: initialPressStep * translation.value,
			},
		}

	case evRepeat:
		if m.activeKey == 0 || iEvent.Code != m.activeKey {
			return nil
		}

		held := t - m.pressTime
		if held < 0 {
			held = 0
		}

		dt := t - m.lastTime
		if dt < 0 {
			dt = 0
		}

		dt = clampDuration(dt, 0, maxStepInterval)
		m.lastTime = t

		var curve speedCurve
		if mode == modePrecise {
			curve = preciseCurve
		} else {
			curve = normalCurve
		}

		dist := travel(curve, held, dt)
		if m.edgeProvider != nil {
			if distToEdge, totalDim, ok := m.edgeProvider.DistanceToEdge(iEvent.Code); ok {
				friction := calculateFriction(distToEdge, totalDim, held)
				dist *= (1.0 - friction)
				if debugMovement {
					log.Printf("edge friction: distToEdge=%.1f totalDim=%.1f held=%v friction=%.3f", distToEdge, totalDim, held, friction)
				}
			}
		}

		dist += m.remainder
		whole, rem := takeWhole(dist)
		m.remainder = rem

		if debugMovement {
			log.Printf("held=%v dt=%v dist=%.3f emitted=%d", held, dt, dist, whole)
		}

		if whole == 0 {
			return nil
		}

		return []*evdev.InputEvent{
			{
				Type:  evdev.EV_REL,
				Code:  translation.code,
				Value: whole * translation.value,
			},
		}

	case evRelease:
		if iEvent.Code == m.activeKey {
			m.reset()
		}
		return nil
	}

	return nil
}
