package quill

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/holoplot/go-evdev"
)

// in builds an EV_KEY event as it would arrive from the physical keyboard.
func in(code evdev.EvCode, val inputVal) *evdev.InputEvent {
	return &evdev.InputEvent{Type: evdev.EV_KEY, Code: code, Value: val}
}

// inAt builds an EV_KEY event with a timestamp.
func inAt(code evdev.EvCode, val inputVal, t time.Duration) *evdev.InputEvent {
	return &evdev.InputEvent{
		Time:  syscall.NsecToTimeval(t.Nanoseconds()),
		Type:  evdev.EV_KEY,
		Code:  code,
		Value: val,
	}
}

// ev builds an expected output event.
func ev(typ evdev.EvType, code evdev.EvCode, val int32) *evdev.InputEvent {
	return &evdev.InputEvent{Type: typ, Code: code, Value: val}
}

type pipeDevice struct {
	file *os.File
}

func (p *pipeDevice) WriteOne(event *evdev.InputEvent) error {
	return binary.Write(p.file, binary.LittleEndian, event)
}

func newPipeDevice() (*pipeDevice, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	return &pipeDevice{file: w}, r
}

func readDeviceEvents(r *os.File) []*evdev.InputEvent {
	var n int
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, r.Fd(), syscall.TIOCINQ, uintptr(unsafe.Pointer(&n)))
	if errno != 0 || n == 0 {
		return nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil
	}
	reader := bytes.NewReader(buf)
	var events []*evdev.InputEvent
	for reader.Len() >= 24 {
		var ev evdev.InputEvent
		if err := binary.Read(reader, binary.LittleEndian, &ev); err != nil {
			break
		}
		events = append(events, &ev)
	}
	return events
}

type testRouter struct {
	*router
	inMovementMode bool
	kbReader       *os.File
	ptrReader      *os.File
}

func newTestPointer(mode pointerMode) *quillPointer {
	return &quillPointer{pointerMode: mode}
}

func newTestRouter() *testRouter {
	kbDev, kbReader := newPipeDevice()
	ptrDev, ptrReader := newPipeDevice()

	kb := &quillKeyboard{
		virtualDevice: virtualDevice{
			eventWriter: kbDev,
			heldKeys:    []evdev.EvCode{},
		},
	}
	ptr := &quillPointer{
		virtualDevice: virtualDevice{
			eventWriter: ptrDev,
			heldKeys:    []evdev.EvCode{},
		},
		pointerMode: modeNormal,
	}

	r := createRouter(ptr, kb)
	return &testRouter{
		router:         r,
		inMovementMode: r.target == targetPointer,
		kbReader:       kbReader,
		ptrReader:      ptrReader,
	}
}

func (tr *testRouter) route(e *evdev.InputEvent) ([]*evdev.InputEvent, []*evdev.InputEvent, bool) {
	oldTarget := tr.target
	tr.router.route(e)
	changed := tr.target != oldTarget
	tr.inMovementMode = tr.target == targetPointer
	kb := readDeviceEvents(tr.kbReader)
	ptr := readDeviceEvents(tr.ptrReader)
	return kb, ptr, changed
}

// payload strips the SYN_REPORT markers so tests can assert on the real events.
func payload(evs []*evdev.InputEvent) []*evdev.InputEvent {
	var out []*evdev.InputEvent
	for _, e := range evs {
		if e.Type != evdev.EV_SYN {
			out = append(out, e)
		}
	}
	return out
}

func describe(e *evdev.InputEvent) string {
	return fmt.Sprintf("%s/%s=%d", evdev.EVToString[e.Type], e.CodeName(), e.Value)
}

func describeAll(evs []*evdev.InputEvent) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, describe(e))
	}
	return out
}

// assertPayload compares type, code and value of the non-SYN events, in order.
func assertPayload(t *testing.T, got []*evdev.InputEvent, want ...*evdev.InputEvent) {
	t.Helper()
	g := payload(got)
	if len(g) != len(want) {
		t.Fatalf("got %v, want %v", describeAll(g), describeAll(want))
	}
	for i := range want {
		if g[i].Type != want[i].Type || g[i].Code != want[i].Code || g[i].Value != want[i].Value {
			t.Fatalf("event %d: got %v, want %v", i, describeAll(g), describeAll(want))
		}
	}
}

func isDeclaredByPointer(e *evdev.InputEvent) bool {
	return slices.Contains(pointerCapabilities[e.Type], e.Code)
}

// deviceState models the kernel's per-device key state: press sets, release clears.
type deviceState map[evdev.EvCode]bool

func (d deviceState) apply(evs []*evdev.InputEvent) {
	for _, e := range evs {
		if e.Type != evdev.EV_KEY {
			continue
		}
		switch e.Value {
		case evPress:
			d[e.Code] = true
		case evRelease:
			delete(d, e.Code)
		}
	}
}

// String prints the held keys by name, sorted, so failures read as
// "keyboard=[KEY_CAPSLOCK]" instead of raw numbers.
func (d deviceState) String() string {
	names := make([]string, 0, len(d))
	for code := range d {
		name, ok := evdev.KEYToString[code]
		if !ok {
			name = fmt.Sprintf("code(%d)", code)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return "[" + strings.Join(names, " ") + "]"
}

var allValues = []inputVal{evRelease, evPress, evRepeat}

var allModes = []pointerMode{modeNormal, modeScroll, modeZoom, modePrecise}

var directionKeys = []evdev.EvCode{evdev.KEY_H, evdev.KEY_J, evdev.KEY_K, evdev.KEY_L}

// every key Quill gives meaning to, plus keys it must ignore.
var allTestKeys = []evdev.EvCode{
	evdev.KEY_H, evdev.KEY_J, evdev.KEY_K, evdev.KEY_L,
	evdev.KEY_N, evdev.KEY_E, evdev.KEY_M, evdev.KEY_R, evdev.KEY_SPACE,
	evdev.KEY_Q, evdev.KEY_W,
	evdev.KEY_S, evdev.KEY_Z, evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL,
	evdev.KEY_A, evdev.KEY_ESC,
}
