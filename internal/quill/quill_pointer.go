package quill

import (
	"fmt"
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
)

type quillPointer struct {
	virtualDevice
	pointerMode
	movement movementState

	mu         sync.Mutex
	tickerStop chan struct{}
	tickerWg   sync.WaitGroup
}

func createQuillPointer() (*quillPointer, error) {
	pointerName := "Quill Pointer"

	inputID := evdev.InputID{
		BusType: evdev.BUS_VIRTUAL,
		Vendor:  0xdead,
		Product: 0xbeef,
		Version: 1,
	}

	qPointer, err := evdev.CreateDevice(
		pointerName,
		inputID,
		pointerCapabilities,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s, got: %w", pointerName, err)
	}

	qPointerVirtual := virtualDevice{
		eventWriter: qPointer,
		heldKeys:    []evdev.EvCode{},
		closeFn: func() error {
			return evdev.DestroyDevice(qPointer)
		},
	}

	qp := &quillPointer{
		virtualDevice: qPointerVirtual,
		pointerMode:   modeNormal,
		movement: movementState{
			edgeProvider: NewHyprlandClient(),
		},
	}

	go func() {
		// Wait briefly for the compositor to register the new uinput device before configuring
		time.Sleep(150 * time.Millisecond)
		AutoConfigureCompositor(qp.movement.edgeProvider)
	}()

	return qp, nil
}

func (q *quillPointer) close() error {
	q.mu.Lock()
	q.stopTicker()
	q.mu.Unlock()
	q.tickerWg.Wait()
	return q.virtualDevice.close()
}

func (q *quillPointer) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopTicker()
	q.pointerMode = modeNormal
	q.movement.reset()
}

const tickerInterval = 10 * time.Millisecond

func (q *quillPointer) startTicker() {
	if q.tickerStop != nil {
		return
	}
	stop := make(chan struct{})
	q.tickerStop = stop
	q.tickerWg.Add(1)

	go func() {
		defer q.tickerWg.Done()
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				q.mu.Lock()
				if q.tickerStop == nil || !q.movement.isMoving() {
					q.mu.Unlock()
					return
				}
				now := currentTimeDuration()
				events := q.movement.step(q.pointerMode, now)
				for _, e := range events {
					q.updateHeldKeys(e)
					_ = q.WriteOne(e)
					_ = q.WriteOne(syncEvent)
				}
				q.mu.Unlock()
			}
		}
	}()
}

func (q *quillPointer) stopTicker() {
	if q.tickerStop != nil {
		close(q.tickerStop)
		q.tickerStop = nil
	}
}

func (q *quillPointer) handleInputEvent(iEvent *evdev.InputEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// If a movement key is held and ticker is driving motion, drop hardware autorepeats
	// so the internal ticker drives smooth, hardware-independent movement.
	if iEvent.Type == evdev.EV_KEY && iEvent.Value == evRepeat {
		if _, isMotion := evKeyToTranslation[iEvent.Code]; isMotion && (q.pointerMode == modeNormal || q.pointerMode == modePrecise) {
			return
		}
	}

	for _, e := range q.process(iEvent) {
		q.updateHeldKeys(e)
		_ = q.WriteOne(e)
	}

	if q.movement.isMoving() && (q.pointerMode == modeNormal || q.pointerMode == modePrecise) {
		q.startTicker()
	} else {
		q.stopTicker()
	}
}

// process is the pure part of HandleInputEvent: it updates the pointer's mode
// and returns the events to emit (each followed by a SYN_REPORT). No I/O.
func (q *quillPointer) process(iEvent *evdev.InputEvent) []*evdev.InputEvent {
	if iEvent.Type != evdev.EV_KEY {
		return nil
	}

	if q.handleMode(iEvent) {
		q.movement.reset()
	}

	eventHandler, ok := keyToEventHandler[iEvent.Code]
	if !ok {
		return nil
	}

	var out []*evdev.InputEvent
	for _, qEvent := range eventHandler(q, iEvent) {
		out = append(out, qEvent, syncEvent)
	}
	return out
}

// For modeNormal, evPress or evRepeat causes transition to other states, evRelease is no-op.
// For non-modeNormal, modeNormal is the only transition allowed.
// For non-modeNormal, evPress or evRepeat is no-op and evRelease transitions to modeNormal.
func (q *quillPointer) handleMode(iEvent *evdev.InputEvent) bool {
	newMode, ok := evKeyToMode[iEvent.Code]
	if !ok {
		return false
	}

	oldMode := q.pointerMode
	switch q.pointerMode {
	case modeNormal:
		if iEvent.Value == evPress || iEvent.Value == evRepeat {
			q.pointerMode = newMode
		}
	default:
		if newMode == q.pointerMode && iEvent.Value == evRelease {
			q.pointerMode = modeNormal
		}
	}
	return q.pointerMode != oldMode
}

// For translation, delegates to the movement state.
// For scroll, evRelease and evRepeat leads to scroll, evPress is no-op.
// For zoom, evRelease and evRepeat leads to zoom, evPress is no-op.
func handleMotionEvent(qPointer *quillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	switch qPointer.pointerMode {
	case modeNormal, modePrecise:
		return qPointer.movement.handle(qPointer.pointerMode, iEvent)

	case modeScroll:
		if iEvent.Value == evPress {
			return nil
		}
		scroll, ok := evKeyToScroll[iEvent.Code]
		if !ok {
			return nil
		}

		return []*evdev.InputEvent{
			{
				Type:  evdev.EV_REL,
				Code:  scroll.code,
				Value: scroll.value,
			},
		}

	case modeZoom:
		if iEvent.Value == evPress {
			return nil
		}
		zoom, ok := evKeyToZoom[iEvent.Code]
		if !ok {
			return nil
		}
		return []*evdev.InputEvent{
			{
				Type:  evdev.EV_KEY,
				Code:  evdev.KEY_LEFTCTRL,
				Value: evPress,
			},
			{
				Type:  evdev.EV_REL,
				Code:  zoom.code,
				Value: zoom.value,
			},
			{
				Type:  evdev.EV_KEY,
				Code:  evdev.KEY_LEFTCTRL,
				Value: evRelease,
			},
		}
	}

	return nil
}

// For click, each event is directly reflected.
// Clicks are ignored during scroll or zoom modes unless releasing a button
// that was pressed prior to entering scroll or zoom mode.
func handleClickEvent(qPointer *quillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	click, ok := evKeyToClick[iEvent.Code]
	if !ok {
		return nil
	}

	if (qPointer.pointerMode == modeScroll || qPointer.pointerMode == modeZoom) && !lo.Contains(qPointer.heldKeys, click.code) {
		return nil
	}

	if iEvent.Value == evRepeat {
		// Repeat events are not required for clicks.
		return nil
	}

	return []*evdev.InputEvent{
		{
			Type:  evdev.EV_KEY,
			Code:  click.code,
			Value: iEvent.Value,
		},
	}
}

// For navigation, evPress and evRelease are operational, evRepeat is no-op.
func handleNavigationEvent(_ *quillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	click, ok := evKeyToNavigation[iEvent.Code]
	if !ok {
		return nil
	}

	if iEvent.Value == evRepeat {
		return nil
	}

	return []*evdev.InputEvent{
		{
			Type:  evdev.EV_KEY,
			Code:  click.code,
			Value: iEvent.Value,
		},
	}
}

type inputVal = int32

const (
	evRelease inputVal = 0
	evPress   inputVal = 1
	evRepeat  inputVal = 2
)

//go:generate stringer -type=pointerMode
type pointerMode int

const (
	modeNormal pointerMode = iota
	modeScroll
	modeZoom
	modePrecise
)

// pointerCapabilities is the single source of truth for what the virtual pointer
// declares to the kernel. Events using a code outside this set are silently
// dropped by uinput, so tests assert every emitted event is covered here.
var pointerCapabilities = map[evdev.EvType][]evdev.EvCode{
	evdev.EV_KEY: {
		evdev.BTN_LEFT,
		evdev.BTN_RIGHT,
		evdev.BTN_MIDDLE,
		evdev.BTN_SIDE,
		evdev.BTN_EXTRA,
		evdev.BTN_BACK,
		evdev.BTN_FORWARD,
		evdev.KEY_LEFTCTRL,
	},
	evdev.EV_REL: {
		evdev.REL_X,
		evdev.REL_Y,
		evdev.REL_WHEEL,
		evdev.REL_HWHEEL,
	},
}

var evKeyToMode = map[evdev.EvCode]pointerMode{
	evdev.KEY_S: modeScroll,
	evdev.KEY_Z: modeZoom,
	evdev.KEY_A: modePrecise,
}

var evKeyToTranslation = map[evdev.EvCode]struct {
	code  evdev.EvCode
	value int32
}{
	evdev.KEY_H: {
		code:  evdev.REL_X,
		value: -1,
	},
	evdev.KEY_J: {
		code:  evdev.REL_Y,
		value: 1,
	},
	evdev.KEY_K: {
		code:  evdev.REL_Y,
		value: -1,
	},
	evdev.KEY_L: {
		code:  evdev.REL_X,
		value: 1,
	},
}

var evKeyToScroll = map[evdev.EvCode]struct {
	code  evdev.EvCode
	value int32
}{
	evdev.KEY_H: {
		code:  evdev.REL_HWHEEL,
		value: 1,
	},
	evdev.KEY_J: {
		code:  evdev.REL_WHEEL,
		value: -1,
	},
	evdev.KEY_K: {
		code:  evdev.REL_WHEEL,
		value: 1,
	},
	evdev.KEY_L: {
		code:  evdev.REL_HWHEEL,
		value: -1,
	},
}

var evKeyToZoom = map[evdev.EvCode]struct {
	code  evdev.EvCode
	value int32
}{
	evdev.KEY_J: {
		code:  evdev.REL_WHEEL,
		value: 1,
	},
	evdev.KEY_K: {
		code:  evdev.REL_WHEEL,
		value: -1,
	},
}

var evKeyToClick = map[evdev.EvCode]struct {
	code evdev.EvCode
}{
	evdev.KEY_SPACE: {
		code: evdev.BTN_MIDDLE,
	},

	evdev.KEY_N: {
		code: evdev.BTN_LEFT,
	},
	evdev.KEY_E: {
		code: evdev.BTN_LEFT,
	},

	evdev.KEY_M: {
		code: evdev.BTN_RIGHT,
	},
	evdev.KEY_R: {
		code: evdev.BTN_RIGHT,
	},
}

var evKeyToNavigation = map[evdev.EvCode]struct {
	code evdev.EvCode
}{
	evdev.KEY_Q: {
		code: evdev.BTN_SIDE, // Standard Linux/X11/Wayland button 8 (Browser Back)
	},
	evdev.KEY_W: {
		code: evdev.BTN_EXTRA, // Standard Linux/X11/Wayland button 9 (Browser Forward)
	},
}

var keyToEventHandler = map[evdev.EvCode]func(qPointer *quillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent{}

func init() {
	lo.ForEach(lo.Keys(evKeyToTranslation), func(key evdev.EvCode, _ int) {
		keyToEventHandler[key] = handleMotionEvent
	})

	lo.ForEach(lo.Keys(evKeyToClick), func(key evdev.EvCode, _ int) {
		keyToEventHandler[key] = handleClickEvent
	})

	lo.ForEach(lo.Keys(evKeyToNavigation), func(key evdev.EvCode, _ int) {
		keyToEventHandler[key] = handleNavigationEvent
	})
}
