package quill

import (
	"fmt"

	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
	"github.com/samber/lo/mutable"
)

type virtualDevice struct {
	*evdev.InputDevice
	heldKeys []evdev.EvCode
}

func (v *virtualDevice) close() error {
	v.flush()

	if err := evdev.DestroyDevice(v.InputDevice); err != nil {
		return fmt.Errorf("failed to cleanup quill keyboard and release resources, got: %w", err)
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

	mutable.Reverse(v.heldKeys)

	lo.ForEach(v.heldKeys, func(code evdev.EvCode, _ int) {
		v.InputDevice.WriteOne(&evdev.InputEvent{
			Type:  evdev.EV_KEY,
			Code:  code,
			Value: evRelease,
		})
	})

	v.InputDevice.WriteOne(syncEvent)
}

func (v *virtualDevice) press(code evdev.EvCode) {
	if !lo.Contains(v.heldKeys, code) {
		v.heldKeys = append(v.heldKeys, code)
	}
}

func (v *virtualDevice) release(code evdev.EvCode) {
	v.heldKeys = lo.Without(v.heldKeys, code)
}
