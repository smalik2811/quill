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
	systemKeyboard, err := getSystemKeyboard()
	if err != nil {
		return fmt.Errorf("failed to read your keyboard, got: %w", err)
	}
	defer func() {
		if err := systemKeyboard.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			fmt.Printf("failed to close the system keyboard resource: %v\n", err)
		}
	}()

	qKeyboard, err := createQuillKeyboard(systemKeyboard)
	if err != nil {
		return fmt.Errorf("failed to create quill keyboard, got: %w", err)
	}
	defer func() {
		if err := qKeyboard.close(); err != nil {
			fmt.Printf("failed to close quill keyboard: %v\n", err)
		}
	}()

	qPointer, err := createQuillPointer()
	if err != nil {
		return fmt.Errorf("failed to create quill pointer, got: %w", err)
	}
	defer func() {
		if err := qPointer.close(); err != nil {
			fmt.Printf("failed to close quill pointer: %v\n", err)
		}
	}()

	r := createRouter(qPointer, qKeyboard)

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

		r.route(inputEvent)
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
