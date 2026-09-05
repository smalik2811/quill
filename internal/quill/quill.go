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
	// 1. GetActualKeyboard
	// 2. Grab it to get its inputs.
	// 3. Clone it for virtual keyboard.
	// 4. Redirect the input of actualKeyboard to virtual keyboard.

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

		if err := quillKeyboard.WriteOne(inputEvent); err != nil {
			fmt.Printf("failed to write input event to the quill keyboard, got: %v\n", err)
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
