package quill

import (
	"log"
	"syscall"
	"time"

	"github.com/holoplot/go-evdev"
)

var debugMovement = false

// ScreenScaleProvider can be implemented by edge/monitor providers to adapt
// velocity dynamically to screen resolution and scaling.
type ScreenScaleProvider interface {
	ScreenScaleFactor() float64
}

type movementState struct {
	activeKey    evdev.EvCode // Tracks most recently pressed active key
	activeX      evdev.EvCode // Active horizontal key: KEY_H or KEY_L
	activeY      evdev.EvCode // Active vertical key: KEY_J or KEY_K
	pressTime    time.Duration
	lastTime     time.Duration
	remainderX   float64
	remainderY   float64
	edgeProvider EdgeDistanceProvider
}

func (m *movementState) reset() {
	m.activeKey = 0
	m.activeX = 0
	m.activeY = 0
	m.pressTime = 0
	m.lastTime = 0
	m.remainderX = 0
	m.remainderY = 0
}

func (m *movementState) isMoving() bool {
	return m.activeX != 0 || m.activeY != 0
}

func isXAxis(code evdev.EvCode) bool {
	return code == evdev.KEY_H || code == evdev.KEY_L
}

func isYAxis(code evdev.EvCode) bool {
	return code == evdev.KEY_J || code == evdev.KEY_K
}

func timevalToDuration(tv syscall.Timeval) time.Duration {
	return time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
}

func currentTimeDuration() time.Duration {
	now := time.Now()
	return time.Duration(now.Unix())*time.Second + time.Duration(now.Nanosecond())
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

// step advances movement calculation to time t and emits any accumulated integer travel.
// Supports simultaneous diagonal movement across X and Y axes with vector normalization.
func (m *movementState) step(mode pointerMode, t time.Duration) []*evdev.InputEvent {
	if !m.isMoving() {
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

	// Dynamically scale travel according to active monitor metrics in normal mode
	if mode == modeNormal {
		if sp, ok := m.edgeProvider.(ScreenScaleProvider); ok {
			if s := sp.ScreenScaleFactor(); s > 0 {
				dist *= s
			}
		}
	}

	// When moving diagonally, normalize vector velocity by 1/sqrt(2) so scalar speed
	// matches cardinal movement, preventing 41% diagonal overspeed.
	diagScale := 1.0
	if m.activeX != 0 && m.activeY != 0 {
		const invSqrt2 = 0.7071067811865475
		diagScale = invSqrt2
	}

	var events []*evdev.InputEvent

	// Process X-axis movement
	if m.activeX != 0 {
		distX := dist * diagScale
		if m.edgeProvider != nil {
			if distToEdge, totalDim, ok := m.edgeProvider.DistanceToEdge(m.activeX); ok {
				friction := calculateFriction(distToEdge, totalDim, held)
				distX *= (1.0 - friction)
				if debugMovement {
					log.Printf("edge friction X: distToEdge=%.1f totalDim=%.1f held=%v friction=%.3f", distToEdge, totalDim, held, friction)
				}
			}
		}

		distX += m.remainderX
		wholeX, remX := takeWhole(distX)
		m.remainderX = remX

		if wholeX > 0 {
			transX := evKeyToTranslation[m.activeX]
			events = append(events, &evdev.InputEvent{
				Type:  evdev.EV_REL,
				Code:  transX.code,
				Value: wholeX * transX.value,
			})
		}
	}

	// Process Y-axis movement
	if m.activeY != 0 {
		distY := dist * diagScale
		if m.edgeProvider != nil {
			if distToEdge, totalDim, ok := m.edgeProvider.DistanceToEdge(m.activeY); ok {
				friction := calculateFriction(distToEdge, totalDim, held)
				distY *= (1.0 - friction)
				if debugMovement {
					log.Printf("edge friction Y: distToEdge=%.1f totalDim=%.1f held=%v friction=%.3f", distToEdge, totalDim, held, friction)
				}
			}
		}

		distY += m.remainderY
		wholeY, remY := takeWhole(distY)
		m.remainderY = remY

		if wholeY > 0 {
			transY := evKeyToTranslation[m.activeY]
			events = append(events, &evdev.InputEvent{
				Type:  evdev.EV_REL,
				Code:  transY.code,
				Value: wholeY * transY.value,
			})
		}
	}

	return events
}

func (m *movementState) handle(mode pointerMode, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	translation, ok := evKeyToTranslation[iEvent.Code]
	if !ok {
		return nil
	}

	t := timevalToDuration(iEvent.Time)

	switch iEvent.Value {
	case evPress:
		wasMoving := m.isMoving()

		if isXAxis(iEvent.Code) {
			if m.activeX != 0 && m.activeX != iEvent.Code {
				// Reversing X direction: reset ramp for smooth turnaround
				m.pressTime = t
			}
			m.activeX = iEvent.Code
			m.remainderX = 0
		} else if isYAxis(iEvent.Code) {
			if m.activeY != 0 && m.activeY != iEvent.Code {
				// Reversing Y direction: reset ramp for smooth turnaround
				m.pressTime = t
			}
			m.activeY = iEvent.Code
			m.remainderY = 0
		}

		m.activeKey = iEvent.Code

		if !wasMoving {
			m.pressTime = t
			m.lastTime = t
		}

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
		if !m.isMoving() {
			return nil
		}
		if iEvent.Code != m.activeX && iEvent.Code != m.activeY {
			return nil
		}
		return m.step(mode, t)

	case evRelease:
		if iEvent.Code == m.activeX {
			m.activeX = 0
			m.remainderX = 0
		}
		if iEvent.Code == m.activeY {
			m.activeY = 0
			m.remainderY = 0
		}

		if iEvent.Code == m.activeKey {
			if m.activeX != 0 {
				m.activeKey = m.activeX
			} else if m.activeY != 0 {
				m.activeKey = m.activeY
			} else {
				m.activeKey = 0
			}
		}

		if !m.isMoving() {
			m.reset()
		}
		return nil
	}

	return nil
}
