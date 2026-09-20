package quill

import (
	"github.com/holoplot/go-evdev"
)

type QuillPointer struct {
	*evdev.InputDevice
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

	return &QuillPointer{qPointer}, err
}

func (q *QuillPointer) HandleInput(iEvent *evdev.InputEvent) {
	if iEvent.Type == evdev.EV_KEY {
		switch iEvent.Code {
		case evdev.KEY_J:
			if iEvent.Value != 0 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_REL,
					Code:  evdev.REL_Y,
					Value: 5,
				})
			}
		case evdev.KEY_K:
			if iEvent.Value != 0 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_REL,
					Code:  evdev.REL_Y,
					Value: -5,
				})
			}
		case evdev.KEY_H:
			if iEvent.Value != 0 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_REL,
					Code:  evdev.REL_X,
					Value: -5,
				})
			}
		case evdev.KEY_L:
			if iEvent.Value != 0 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_REL,
					Code:  evdev.REL_X,
					Value: 5,
				})
			}

		case evdev.KEY_N:
			if iEvent.Value != 2 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_KEY,
					Code:  evdev.BTN_LEFT,
					Value: iEvent.Value,
				})
			}
		case evdev.KEY_M:
			if iEvent.Value != 2 {
				q.InputDevice.WriteOne(&evdev.InputEvent{
					Type:  evdev.EV_KEY,
					Code:  evdev.BTN_RIGHT,
					Value: iEvent.Value,
				})
			}
		}

		q.InputDevice.WriteOne(&evdev.InputEvent{Type: evdev.EV_SYN, Code: evdev.SYN_REPORT, Value: 0})
	}
}
