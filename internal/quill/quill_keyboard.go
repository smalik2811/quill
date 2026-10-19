package quill

import (
	"fmt"

	"github.com/holoplot/go-evdev"
)

type quillKeyboard struct {
	virtualDevice
}

func createQuillKeyboard(sourceKeyboard *evdev.InputDevice) (*quillKeyboard, error) {
	keyboardName := "Quill Keyboard"
	sKbName, _ := sourceKeyboard.Name()

	qKeyboard, err := evdev.CloneDevice(keyboardName, sourceKeyboard)
	if err != nil {
		return nil, fmt.Errorf("failed to clone the user keyboard %s", sKbName)
	}

	return &quillKeyboard{
		virtualDevice: virtualDevice{
			eventWriter: qKeyboard,
			heldKeys:    []evdev.EvCode{},
			closeFn: func() error {
				return evdev.DestroyDevice(qKeyboard)
			},
		},
	}, nil
}

func (k *quillKeyboard) handleInputEvent(iEvent *evdev.InputEvent) {
	k.updateHeldKeys(iEvent)
	k.WriteOne(iEvent)
}
