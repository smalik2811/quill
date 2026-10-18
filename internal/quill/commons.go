package quill

import "github.com/holoplot/go-evdev"

var syncEvent = &evdev.InputEvent{
	Type:  evdev.EV_SYN,
	Code:  evdev.SYN_REPORT,
	Value: 0,
}
