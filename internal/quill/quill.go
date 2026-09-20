package quill

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
)

func Run(ctx context.Context) error {
	// inMovementMode decides whether the key input should be directed
	// to the virtual keyboard or the virtual pointing device.
	var inMovementMode bool

	systemKeyboard, err := getSystemKeyboard()
	if err != nil {
		return fmt.Errorf("failed to read your keyboard, got: %w", err)
	}
	defer func() {
		if err := systemKeyboard.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			fmt.Printf("failed to close the system keyboard resource: %v\n", err)
		}
	}()

	quillKeyboard, err := evdev.CloneDevice("Quill_Keyboard", systemKeyboard)
	if err != nil {
		return fmt.Errorf("failed to clone your keyboard, got: %w", err)
	}
	defer func() {
		if err := evdev.DestroyDevice(quillKeyboard); err != nil {
			fmt.Printf("failed to destroy the quill keyboard resource: %v\n", err)
		}
	}()

	quillPointer, err := NewQuillPonter()
	if err != nil {
		return fmt.Errorf("failed to create quill pointer, got: %w", err)
	}
	defer func() {
		if err := quillKeyboard.Close(); err != nil {
			fmt.Printf("failed to close the quill pointer: %v\n", err)
			return
		}

		if err := evdev.DestroyDevice(quillPointer.InputDevice); err != nil {
			fmt.Printf("failed to destroy the quill pointer resource: %v\n", err)
		}
	}()

	if err := systemKeyboard.Grab(); err != nil {
		return fmt.Errorf("failed to grab you keyboard, got: %w", err)
	}
	defer func() {
		if err := systemKeyboard.Ungrab(); err != nil && !errors.Is(err, syscall.EBADF) && !strings.Contains(err.Error(), "bad file descriptor") {
			fmt.Printf("failed to ungrab your keyboard: %v\n", err)
		}
	}()

	if err := systemKeyboard.NonBlock(); err != nil {
		return fmt.Errorf("failed to set your keyboard to non block: %w", err)
	}

	stopWatcher := make(chan struct{}, 1)
	defer func() {
		close(stopWatcher)
	}()

	go func() {
		select {
		case <-ctx.Done():
			systemKeyboard.Close()
		case <-stopWatcher:
		}
	}()

	for {
		if err := ctx.Err(); err != nil {
			break
		}

		inputEvent, err := systemKeyboard.ReadOne()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Printf("failed to read the input event from system keyboard, got: %v\n", err)
			continue
		}

		fmt.Printf("input event received from system keyboard: %+v\n", inputEvent)

		if inputEvent.Type == evdev.EV_KEY && inputEvent.Code == evdev.KEY_CAPSLOCK && inputEvent.Value == 0 {
			inMovementMode = !inMovementMode
			fmt.Printf("~~~ MODE CHANGED ~~~")

			inputPipedTo := "keyboard"
			if inMovementMode {
				inputPipedTo = "mouse"
			}
			fmt.Printf("Input being directed to %v", inputPipedTo)

			continue
		}

		if inMovementMode {
			quillPointer.HandleInput(inputEvent)
		} else {
			if err := quillKeyboard.WriteOne(inputEvent); err != nil {
				fmt.Printf("failed to write input event to the quill keyboard, got: %v\n", err)
			}
		}
	}

	return nil
}

func getSystemKeyboard() (*evdev.InputDevice, error) {
	// TODO: Handle cleanup
	const KeyboardDeviceIdentifier = "keyboard"

	iPaths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices, got: %w", err)
	}

	kbInput, ok := lo.Find(iPaths, func(item evdev.InputPath) bool {
		return strings.Contains(item.Name, KeyboardDeviceIdentifier)
	})
	if !ok {
		return nil, fmt.Errorf("no input device found with name matching: %s", KeyboardDeviceIdentifier)
	}

	if kbDevice, err := evdev.Open(kbInput.Path); err != nil {
		return nil, fmt.Errorf("failed to open the keyboard device %s at %s, got: %v", kbInput.Name, kbInput.Path, err)
	} else {
		return kbDevice, nil
	}
}
