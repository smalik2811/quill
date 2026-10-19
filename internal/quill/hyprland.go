package quill

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
)

type EdgeDistanceProvider interface {
	DistanceToEdge(code evdev.EvCode) (distance float64, totalDim float64, ok bool)
}

type hyprlandMonitor struct {
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Scale   float64 `json:"scale"`
	Focused bool    `json:"focused"`
}

type hyprlandCursorPos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type HyprlandClient struct {
	socketPath string
	mu         sync.Mutex
	monitors   []hyprlandMonitor
	lastMon    time.Time
}

func findHyprlandSocket() string {
	// 1. Direct environment variables
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if his != "" && xdg != "" {
		sock := fmt.Sprintf("%s/hypr/%s/.socket.sock", xdg, his)
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}

	// 2. Under sudo, check SUDO_UID
	sudoUID := os.Getenv("SUDO_UID")
	if sudoUID != "" {
		pattern := fmt.Sprintf("/run/user/%s/hypr/*/.socket.sock", sudoUID)
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}

	// 3. Fallback scan under /run/user/
	if matches, err := filepath.Glob("/run/user/*/hypr/*/.socket.sock"); err == nil && len(matches) > 0 {
		return matches[len(matches)-1]
	}

	return ""
}

func NewHyprlandClient() EdgeDistanceProvider {
	sock := findHyprlandSocket()
	if sock == "" {
		return nil
	}
	return &HyprlandClient{
		socketPath: sock,
	}
}

func (h *HyprlandClient) sendCommand(cmd string) ([]byte, error) {
	if h == nil || h.socketPath == "" {
		return nil, fmt.Errorf("hyprland client is nil or unconfigured")
	}

	conn, err := net.DialTimeout("unix", h.socketPath, 20*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(50 * time.Millisecond))

	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}

	return io.ReadAll(conn)
}

func (h *HyprlandClient) getMonitors() ([]hyprlandMonitor, error) {
	if h == nil {
		return nil, fmt.Errorf("hyprland client is nil")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.monitors) > 0 && time.Since(h.lastMon) < 2*time.Second {
		return h.monitors, nil
	}

	data, err := h.sendCommand("j/monitors")
	if err != nil {
		return nil, err
	}

	var mons []hyprlandMonitor
	if err := json.Unmarshal(data, &mons); err != nil {
		return nil, err
	}

	h.monitors = mons
	h.lastMon = time.Now()
	return mons, nil
}

func (h *HyprlandClient) getCursorPos() (hyprlandCursorPos, error) {
	if h == nil {
		return hyprlandCursorPos{}, fmt.Errorf("hyprland client is nil")
	}

	data, err := h.sendCommand("j/cursorpos")
	if err != nil {
		return hyprlandCursorPos{}, err
	}

	var pos hyprlandCursorPos
	if err := json.Unmarshal(data, &pos); err != nil {
		return hyprlandCursorPos{}, err
	}
	return pos, nil
}

func (h *HyprlandClient) DistanceToEdge(code evdev.EvCode) (distance float64, totalDim float64, ok bool) {
	if h == nil {
		return 0, 0, false
	}

	cursor, err := h.getCursorPos()
	if err != nil {
		return 0, 0, false
	}

	mons, err := h.getMonitors()
	if err != nil || len(mons) == 0 {
		return 0, 0, false
	}

	var activeMon *hyprlandMonitor
	for i := range mons {
		m := &mons[i]
		scale := m.Scale
		if scale <= 0 {
			scale = 1
		}
		w := m.Width / scale
		ht := m.Height / scale
		if cursor.X >= m.X && cursor.X < m.X+w && cursor.Y >= m.Y && cursor.Y < m.Y+ht {
			activeMon = m
			break
		}
	}

	if activeMon == nil {
		for i := range mons {
			if mons[i].Focused {
				activeMon = &mons[i]
				break
			}
		}
		if activeMon == nil {
			activeMon = &mons[0]
		}
	}

	scale := activeMon.Scale
	if scale <= 0 {
		scale = 1
	}
	logW := activeMon.Width / scale
	logH := activeMon.Height / scale
	minX := activeMon.X
	maxX := activeMon.X + logW
	minY := activeMon.Y
	maxY := activeMon.Y + logH

	switch code {
	case evdev.KEY_L: // moving right
		distance = maxX - cursor.X
		totalDim = logW
	case evdev.KEY_H: // moving left
		distance = cursor.X - minX
		totalDim = logW
	case evdev.KEY_J: // moving down
		distance = maxY - cursor.Y
		totalDim = logH
	case evdev.KEY_K: // moving up
		distance = cursor.Y - minY
		totalDim = logH
	default:
		return 0, 0, false
	}

	if distance < 0 {
		distance = 0
	}
	return distance, totalDim, true
}
