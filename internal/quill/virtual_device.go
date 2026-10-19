package quill

import (
	"fmt"
	"slices"

	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
)

type eventWriter interface {
	WriteOne(event *evdev.InputEvent) error
}

type virtualDevice struct {
	eventWriter
	heldKeys []evdev.EvCode
	closeFn  func() error
}

func (v *virtualDevice) close() error {
	v.flush()

	if v.closeFn != nil {
		if err := v.closeFn(); err != nil {
			return fmt.Errorf("failed to cleanup device resources, got: %w", err)
		}
	}

	return nil
}

func (v *virtualDevice) updateHeldKeys(iEvent *evdev.InputEvent) {
	if iEvent.Type == evdev.EV_KEY {
		switch iEvent.Value {
		case evPress:
			v.press(iEvent.Code)
		case evRelease:
			v.release(iEvent.Code)
		}
	}
}

func (v *virtualDevice) flush() {
	if len(v.heldKeys) == 0 {
		return
	}

	for i := range slices.Backward(v.heldKeys) {
		v.WriteOne(&evdev.InputEvent{
			Type:  evdev.EV_KEY,
			Code:  v.heldKeys[i],
			Value: evRelease,
		})
		v.WriteOne(syncEvent)
	}

	v.heldKeys = []evdev.EvCode{}
}

func (v *virtualDevice) press(code evdev.EvCode) {
	if !lo.Contains(v.heldKeys, code) {
		v.heldKeys = append(v.heldKeys, code)
	}
}

func (v *virtualDevice) release(code evdev.EvCode) {
	v.heldKeys = lo.Without(v.heldKeys, code)
}
