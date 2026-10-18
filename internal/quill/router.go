package quill

import (
	"github.com/holoplot/go-evdev"
)

//go:generate stringer -type=routeTarget
type routeTarget int

const (
	targetKeyboard routeTarget = iota
	targetPointer
)

type router struct {
	pointerDevice  *quillPointer
	keyboardDevice *quillKeyboard
	target         routeTarget
}

func createRouter(pointerDevice *quillPointer, keyboardDevice *quillKeyboard) *router {
	return &router{
		pointerDevice:  pointerDevice,
		keyboardDevice: keyboardDevice,
		target:         targetKeyboard,
	}
}

func (r *router) route(iEvent *evdev.InputEvent) {
	if iEvent.Type == evdev.EV_KEY && iEvent.Code == evdev.KEY_CAPSLOCK {
		if iEvent.Value != evRelease {
			return
		}

		switch r.target {
		case targetKeyboard:
			r.keyboardDevice.flush()
			r.target = targetPointer
		case targetPointer:
			r.pointerDevice.flush()
			r.target = targetKeyboard
		}
		return
	}

	switch r.target {
	case targetKeyboard:
		r.keyboardDevice.handleInputEvent(iEvent)
	case targetPointer:
		r.pointerDevice.handleInputEvent(iEvent)
	}
}
