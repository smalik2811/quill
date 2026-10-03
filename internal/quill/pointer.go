package quill

import (
	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
)

type pointerMode int

const (
	modeNormal pointerMode = iota
	modeScroll
	modeZoom
	modePrecise
)

var evKeyToMode = map[evdev.EvCode]pointerMode{
	evdev.KEY_S:        modeScroll,
	evdev.KEY_Z:        modeZoom,
	evdev.KEY_LEFTCTRL: modePrecise,
}

var keyToEventHandler = map[evdev.EvCode]func(qPointer *QuillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent{}

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
		value: -1,
	},
	evdev.KEY_J: {
		code:  evdev.REL_WHEEL,
		value: 1,
	},
	evdev.KEY_K: {
		code:  evdev.REL_WHEEL,
		value: -1,
	},
	evdev.KEY_L: {
		code:  evdev.REL_HWHEEL,
		value: 1,
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
		code: evdev.BTN_BACK,
	},
	evdev.KEY_W: {
		code: evdev.BTN_FORWARD,
	},
}

type QuillPointer struct {
	*evdev.InputDevice
	pointerMode
	motionSpeed int32
}

func NewQuillPonter() (*QuillPointer, error) {
	pointerName := "Quill Pointing Device"

	inputID := evdev.InputID{
		BusType: evdev.BUS_VIRTUAL,
		Vendor:  0xdead,
		Product: 0xbeef,
		Version: 1,
	}

	capabilities := map[evdev.EvType][]evdev.EvCode{
		evdev.EV_KEY: {evdev.BTN_LEFT, evdev.BTN_RIGHT},
		evdev.EV_REL: {evdev.REL_X, evdev.REL_Y},
	}

	qPointer, err := evdev.CreateDevice(
		pointerName,
		inputID,
		capabilities,
	)

	return &QuillPointer{qPointer, modeNormal, 1}, err
}

// For modeNormal, evPress or evRepeat causes transtion to other states, evRelease is no-op.
// For non-modeNormal, modeNormal is the only transition allowed.
// For non-modeNormal, evPress or evRepeat is no-op and evRelease transitions to modeNormal.
func (q *QuillPointer) handleMode(iEvent *evdev.InputEvent) {
	newMode, ok := evKeyToMode[iEvent.Code]

	if !ok {
		return
	}

	switch q.pointerMode {
	case modeNormal:
		if iEvent.Value == evPress || iEvent.Value == evRepeat {
			q.pointerMode = newMode
		}
	default:
		if iEvent.Value == evRelease {
			q.pointerMode = modeNormal
		}
	}

}

// For translation, evRelease and evRepeat leads to movement, evPress is no-op.
// Translation could be precise.
// For scroll, evRelease and evRepeat leads to scroll, evPress is no-op.
// For zoom, evRelease and evRepeat leds to zoom, evPress is no-op.
func handleMotionEvent(qPointer *QuillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	if iEvent.Value == evPress {
		return nil
	}

	switch qPointer.pointerMode {
	case modeNormal, modePrecise:
		translation, ok := evKeyToTranslation[iEvent.Code]
		if !ok {
			return nil
		}

		motionSpeed := qPointer.motionSpeed
		if qPointer.pointerMode == modePrecise {
			motionSpeed = 1
		}

		return []*evdev.InputEvent{
			&evdev.InputEvent{
				Type:  evdev.EV_REL,
				Code:  translation.code,
				Value: translation.value * motionSpeed,
			},
		}

	case modeScroll:
		scroll, ok := evKeyToScroll[iEvent.Code]
		if !ok {
			return nil
		}

		return []*evdev.InputEvent{
			&evdev.InputEvent{
				Type:  evdev.EV_REL,
				Code:  scroll.code,
				Value: scroll.value,
			},
		}

	case modeZoom:
		zoom, ok := evKeyToZoom[iEvent.Code]
		if !ok {
			return nil
		}
		return []*evdev.InputEvent{
			&evdev.InputEvent{
				Type:  evdev.EV_KEY,
				Code:  evdev.KEY_LEFTCTRL,
				Value: evPress,
			},
			&evdev.InputEvent{
				Type:  evdev.EV_REL,
				Code:  zoom.code,
				Value: zoom.value,
			},
			&evdev.InputEvent{
				Type:  evdev.EV_KEY,
				Code:  evdev.KEY_LEFTCTRL,
				Value: evRelease,
			},
		}
	}

	return nil
}

// For click, each event is directly reflected.
func handleClickEvent(_ *QuillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	click, ok := evKeyToClick[iEvent.Code]

	if !ok {
		return nil
	}

	return []*evdev.InputEvent{
		&evdev.InputEvent{
			Type:  evdev.EV_KEY,
			Code:  click.code,
			Value: iEvent.Value,
		},
	}
}

// For navigation, only evRelease is operational.
func handleNavigationEvent(_ *QuillPointer, iEvent *evdev.InputEvent) []*evdev.InputEvent {
	click, ok := evKeyToNavigation[iEvent.Code]

	if !ok {
		return nil
	}

	if iEvent.Value != evRelease {
		return nil
	}

	return []*evdev.InputEvent{
		&evdev.InputEvent{
			Type:  evdev.EV_KEY,
			Code:  click.code,
			Value: iEvent.Value,
		},
	}
}

func (q *QuillPointer) HandleInputEvent(iEvent *evdev.InputEvent) {
	if iEvent.Type != evdev.EV_KEY {
		return
	}

	q.handleMode(iEvent)

	if eventHandler, ok := keyToEventHandler[iEvent.Code]; ok {
		quillEvents := eventHandler(q, iEvent)
		lo.ForEach(quillEvents, func(qEvent *evdev.InputEvent, _ int) {
			q.InputDevice.WriteOne(qEvent)
			q.InputDevice.WriteOne(syncEvent)
		})
	}
}

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
