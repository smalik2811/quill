package quill

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/holoplot/go-evdev"
	"github.com/samber/lo"
)

const debugEvents = false

func Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	systemKeyboard, err := getSystemKeyboard()
	if err != nil {
		return fmt.Errorf("failed to read your keyboard, got: %w", err)
	}

	var (
		cleanupKeyboardOnce sync.Once
		isGrabbed           bool
		grabMu              sync.Mutex
	)
	cleanupKeyboard := func() {
		cleanupKeyboardOnce.Do(func() {
			grabMu.Lock()
			grabbed := isGrabbed
			grabMu.Unlock()
			if grabbed {
				if err := systemKeyboard.Ungrab(); err != nil && !errors.Is(err, syscall.EBADF) && !strings.Contains(err.Error(), "bad file descriptor") {
					fmt.Printf("failed to ungrab your keyboard: %v\n", err)
				}
			}
			if err := systemKeyboard.Close(); err != nil && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EBADF) {
				fmt.Printf("failed to close the system keyboard resource: %v\n", err)
			}
		})
	}
	defer cleanupKeyboard()

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
	grabMu.Lock()
	isGrabbed = true
	grabMu.Unlock()

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
			cleanupKeyboard()
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

		if debugEvents {
			fmt.Printf("input event received from system keyboard: %+v\n", inputEvent)
		}

		r.route(inputEvent)
	}

	return nil
}

func getSystemKeyboard() (*evdev.InputDevice, error) {
	const keyboardDeviceIdentifier = "keyboard"

	iPaths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices, got: %w", err)
	}

	if len(iPaths) == 0 && os.Geteuid() != 0 {
		return nil, fmt.Errorf("no input devices accessible; please run with sudo or add your user to the 'input' group")
	}

	kbInput, ok := lo.Find(iPaths, func(item evdev.InputPath) bool {
		return strings.Contains(strings.ToLower(item.Name), keyboardDeviceIdentifier)
	})
	if !ok {
		return nil, fmt.Errorf("no input device found with name matching: %s", keyboardDeviceIdentifier)
	}

	kbDevice, err := evdev.Open(kbInput.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open the keyboard device %s at %s, got: %v", kbInput.Name, kbInput.Path, err)
	}
	return kbDevice, nil
}
