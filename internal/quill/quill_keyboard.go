package quill

import (
	"fmt"

	"github.com/holoplot/go-evdev"
)

type quillKeyboard = virtualDevice

func createQuillKeyboard(sourceKeyboard *evdev.InputDevice) (*virtualDevice, error) {
	keyboardName := "Quill Keyboard"
	sKbName, _ := sourceKeyboard.Name()

	qKeyboard, err := evdev.CloneDevice(keyboardName, sourceKeyboard)
	if err != nil {
		return nil, fmt.Errorf("failed to clone the user keyboard %s", sKbName)
	}

	return &virtualDevice{
		InputDevice: qKeyboard,
		heldKeys:    []evdev.EvCode{},
	}, nil
}

func (v *virtualDevice) handleInputEvent(iEvent *evdev.InputEvent) {
	v.InputDevice.WriteOne(iEvent)
	v.InputDevice.WriteOne(syncEvent)
}
