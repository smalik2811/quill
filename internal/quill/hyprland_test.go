package quill

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/holoplot/go-evdev"
)

type mockHyprlandServer struct {
	listener  net.Listener
	mu        sync.Mutex
	monitors  []hyprlandMonitor
	cursor    hyprlandCursorPos
	badJSON   bool
	lastCmds  []string
	closeChan chan struct{}
}

func startMockHyprlandServer(t *testing.T) (*mockHyprlandServer, string) {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "hypr_test.sock")

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}

	server := &mockHyprlandServer{
		listener:  l,
		closeChan: make(chan struct{}),
		monitors: []hyprlandMonitor{
			{
				ID:      0,
				X:       0,
				Y:       0,
				Width:   1920,
				Height:  1080,
				Scale:   1.0,
				Focused: false,
			},
			{
				ID:      1,
				X:       1920,
				Y:       0,
				Width:   3840,
				Height:  2160,
				Scale:   2.0, // Logical: 1920x1080 (extends from 1920 to 3840)
				Focused: true,
			},
		},
		cursor: hyprlandCursorPos{X: 500, Y: 300},
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go server.handleConn(conn)
		}
	}()

	t.Cleanup(func() {
		close(server.closeChan)
		_ = l.Close()
	})

	return server, sockPath
}

func (s *mockHyprlandServer) handleConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return
	}
	cmd := string(buf[:n])

	s.mu.Lock()
	s.lastCmds = append(s.lastCmds, cmd)
	badJSON := s.badJSON
	mons := s.monitors
	cur := s.cursor
	s.mu.Unlock()

	if badJSON {
		_, _ = conn.Write([]byte("invalid json{"))
		return
	}

	switch cmd {
	case "j/monitors":
		data, _ := json.Marshal(mons)
		_, _ = conn.Write(data)
	case "j/cursorpos":
		data, _ := json.Marshal(cur)
		_, _ = conn.Write(data)
	default:
		_, _ = conn.Write([]byte("ok"))
	}
}

func TestHyprland_FindSocket(t *testing.T) {
	tmpDir := t.TempDir()
	sig := "test-instance-123"
	sockDir := filepath.Join(tmpDir, "hypr", sig)
	if err := os.MkdirAll(sockDir, 0755); err != nil {
		t.Fatal(err)
	}
	sockFile := filepath.Join(sockDir, ".socket.sock")
	if err := os.WriteFile(sockFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig)
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	got := findHyprlandSocket()
	if got != sockFile {
		t.Errorf("findHyprlandSocket() = %q, want %q", got, sockFile)
	}
}

func TestHyprland_NewHyprlandClient_Lifecycle(t *testing.T) {
	client := NewHyprlandClient()
	if client != nil {
		sock := findHyprlandSocket()
		if sock == "" {
			t.Error("client is non-nil but findHyprlandSocket is empty")
		}
	}
}

func TestHyprland_SendCommand_Errors(t *testing.T) {
	var nilClient *HyprlandClient
	if _, err := nilClient.sendCommand("test"); err == nil {
		t.Error("sendCommand on nil client expected error, got nil")
	}

	emptyClient := &HyprlandClient{}
	if _, err := emptyClient.sendCommand("test"); err == nil {
		t.Error("sendCommand on empty client expected error, got nil")
	}

	badClient := &HyprlandClient{socketPath: "/nonexistent/socket/path.sock"}
	if _, err := badClient.sendCommand("test"); err == nil {
		t.Error("sendCommand on bad socket path expected dial error, got nil")
	}
}

func TestHyprland_GetMonitorsAndCaching(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	mons, err := client.getMonitors()
	if err != nil {
		t.Fatalf("getMonitors() error: %v", err)
	}
	if len(mons) != 2 {
		t.Fatalf("getMonitors() returned %d monitors, want 2", len(mons))
	}
	if mons[0].ID != 0 || mons[1].ID != 1 {
		t.Errorf("monitor IDs = [%d, %d], want [0, 1]", mons[0].ID, mons[1].ID)
	}

	// Second call should return cached monitors
	server.mu.Lock()
	server.monitors = []hyprlandMonitor{} // empty server state
	server.mu.Unlock()

	cachedMons, err := client.getMonitors()
	if err != nil {
		t.Fatalf("cached getMonitors() error: %v", err)
	}
	if len(cachedMons) != 2 {
		t.Errorf("expected 2 cached monitors, got %d", len(cachedMons))
	}

	// Invalidate cache by setting lastMon in the past
	client.mu.Lock()
	client.lastMon = time.Now().Add(-5 * time.Second)
	client.mu.Unlock()

	freshMons, err := client.getMonitors()
	if err != nil {
		t.Fatalf("fresh getMonitors() error: %v", err)
	}
	if len(freshMons) != 0 {
		t.Errorf("expected fresh getMonitors() to return 0 monitors from updated server, got %d", len(freshMons))
	}
}

func TestHyprland_GetCursorPos(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 123.5, Y: 456.75}
	server.mu.Unlock()

	pos, err := client.getCursorPos()
	if err != nil {
		t.Fatalf("getCursorPos() error: %v", err)
	}
	if pos.X != 123.5 || pos.Y != 456.75 {
		t.Errorf("getCursorPos() = %+v, want {X: 123.5, Y: 456.75}", pos)
	}
}

func TestHyprland_BadJSONHandling(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	server.mu.Lock()
	server.badJSON = true
	server.mu.Unlock()

	if _, err := client.getMonitors(); err == nil {
		t.Error("getMonitors() expected unmarshal error, got nil")
	}
	if _, err := client.getCursorPos(); err == nil {
		t.Error("getCursorPos() expected unmarshal error, got nil")
	}
}

func TestHyprland_GetActiveMonitor(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	// 1. Cursor on Monitor 0: (500, 300)
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 500, Y: 300}
	server.mu.Unlock()

	mon, err := client.getActiveMonitor()
	if err != nil {
		t.Fatalf("getActiveMonitor() error: %v", err)
	}
	if mon.ID != 0 {
		t.Errorf("active monitor ID = %d, want 0", mon.ID)
	}

	// 2. Cursor on Monitor 1: (2500, 400).
	// Monitor 1 starts at X=1920 and has logical width 3840 / 2.0 = 1920 (extends from 1920 to 3840).
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 2500, Y: 400}
	server.mu.Unlock()

	mon, err = client.getActiveMonitor()
	if err != nil {
		t.Fatalf("getActiveMonitor() error: %v", err)
	}
	if mon.ID != 1 {
		t.Errorf("active monitor ID = %d, want 1", mon.ID)
	}

	// 3. Cursor outside all monitors: (5000, 5000) -> should fall back to focused monitor (Monitor 1)
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 5000, Y: 5000}
	server.mu.Unlock()

	mon, err = client.getActiveMonitor()
	if err != nil {
		t.Fatalf("getActiveMonitor() error: %v", err)
	}
	if mon.ID != 1 {
		t.Errorf("fallback to focused monitor ID = %d, want 1", mon.ID)
	}

	// 4. Cursor outside all monitors and neither focused -> falls back to first monitor
	server.mu.Lock()
	server.monitors[0].Focused = false
	server.monitors[1].Focused = false
	server.mu.Unlock()
	client.mu.Lock()
	client.monitors = nil // invalidate cache
	client.mu.Unlock()

	mon, err = client.getActiveMonitor()
	if err != nil {
		t.Fatalf("getActiveMonitor() error: %v", err)
	}
	if mon.ID != 0 {
		t.Errorf("fallback to first monitor ID = %d, want 0", mon.ID)
	}
}

func TestHyprland_DistanceToEdge(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	// Cursor at (500, 300) on Monitor 0 (X:0..1920, Y:0..1080)
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 500, Y: 300}
	server.mu.Unlock()

	// KEY_L: moving right -> distance = 1920 - 500 = 1420
	dist, dim, ok := client.DistanceToEdge(evdev.KEY_L)
	if !ok || dist != 1420 || dim != 1920 {
		t.Errorf("KEY_L: dist=%v, dim=%v, ok=%v; want 1420, 1920, true", dist, dim, ok)
	}

	// KEY_H: moving left -> distance = 500 - 0 = 500
	dist, dim, ok = client.DistanceToEdge(evdev.KEY_H)
	if !ok || dist != 500 || dim != 1920 {
		t.Errorf("KEY_H: dist=%v, dim=%v, ok=%v; want 500, 1920, true", dist, dim, ok)
	}

	// KEY_J: moving down -> distance = 1080 - 300 = 780
	dist, dim, ok = client.DistanceToEdge(evdev.KEY_J)
	if !ok || dist != 780 || dim != 1080 {
		t.Errorf("KEY_J: dist=%v, dim=%v, ok=%v; want 780, 1080, true", dist, dim, ok)
	}

	// KEY_K: moving up -> distance = 300 - 0 = 300
	dist, dim, ok = client.DistanceToEdge(evdev.KEY_K)
	if !ok || dist != 300 || dim != 1080 {
		t.Errorf("KEY_K: dist=%v, dim=%v, ok=%v; want 300, 1080, true", dist, dim, ok)
	}

	// Non-movement key returns ok = false
	if _, _, ok := client.DistanceToEdge(evdev.KEY_A); ok {
		t.Error("KEY_A: expected ok=false, got true")
	}

	// Nil client returns ok = false
	var nilClient *HyprlandClient
	if _, _, ok := nilClient.DistanceToEdge(evdev.KEY_L); ok {
		t.Error("nil client expected ok=false, got true")
	}

	// Out of bounds clamping (cursor beyond monitor 1 maxX = 3840)
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 4500, Y: 300}
	server.mu.Unlock()
	dist, _, ok = client.DistanceToEdge(evdev.KEY_L)
	if !ok || dist != 0 {
		t.Errorf("KEY_L out of bounds: dist=%v, want 0 clamped", dist)
	}
}

func TestHyprland_ScreenScaleFactor(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	// 1. Standard 1080p display (1920 logical width) -> factor 1.0
	server.mu.Lock()
	server.cursor = hyprlandCursorPos{X: 500, Y: 300} // on Mon 0 (1920 logical)
	server.mu.Unlock()

	factor := client.ScreenScaleFactor()
	if factor != 1.0 {
		t.Errorf("ScreenScaleFactor() on 1080p = %v, want 1.0", factor)
	}

	// 2. 4K unscaled monitor (3840 logical width)
	server.mu.Lock()
	server.monitors = []hyprlandMonitor{
		{
			ID:      0,
			X:       0,
			Y:       0,
			Width:   3840,
			Height:  2160,
			Scale:   1.0,
			Focused: true,
		},
	}
	server.cursor = hyprlandCursorPos{X: 1000, Y: 500}
	server.mu.Unlock()
	client.mu.Lock()
	client.monitors = nil
	client.mu.Unlock()

	factor = client.ScreenScaleFactor()
	if factor != 2.0 {
		t.Errorf("ScreenScaleFactor() on 4K unscaled = %v, want 2.0", factor)
	}

	// 3. Ultrawide 3440x1440 monitor
	server.mu.Lock()
	server.monitors = []hyprlandMonitor{
		{
			ID:      0,
			X:       0,
			Y:       0,
			Width:   3440,
			Height:  1440,
			Scale:   1.0,
			Focused: true,
		},
	}
	server.mu.Unlock()
	client.mu.Lock()
	client.monitors = nil
	client.mu.Unlock()

	wantFactor := 3440.0 / 1920.0
	factor = client.ScreenScaleFactor()
	if fmt.Sprintf("%.4f", factor) != fmt.Sprintf("%.4f", wantFactor) {
		t.Errorf("ScreenScaleFactor() on Ultrawide = %v, want %v", factor, wantFactor)
	}

	// 4. Clamping bounds: ultra-small resolution (< 0.5 clamped to 0.5)
	server.mu.Lock()
	server.monitors[0].Width = 640
	server.mu.Unlock()
	client.mu.Lock()
	client.monitors = nil
	client.mu.Unlock()

	factor = client.ScreenScaleFactor()
	if factor != 0.5 {
		t.Errorf("ScreenScaleFactor() small screen clamping = %v, want 0.5", factor)
	}

	// 5. Clamping bounds: ultra-wide (> 3.0 clamped to 3.0)
	server.mu.Lock()
	server.monitors[0].Width = 7680
	server.mu.Unlock()
	client.mu.Lock()
	client.monitors = nil
	client.mu.Unlock()

	factor = client.ScreenScaleFactor()
	if factor != 3.0 {
		t.Errorf("ScreenScaleFactor() ultra-large screen clamping = %v, want 3.0", factor)
	}

	// 6. Nil client returns default 1.0
	var nilClient *HyprlandClient
	if f := nilClient.ScreenScaleFactor(); f != 1.0 {
		t.Errorf("nilClient.ScreenScaleFactor() = %v, want 1.0", f)
	}
}

func TestHyprland_ConfigureCompositor(t *testing.T) {
	server, sockPath := startMockHyprlandServer(t)
	client := &HyprlandClient{socketPath: sockPath}

	client.ConfigureCompositor()

	server.mu.Lock()
	cmds := server.lastCmds
	server.mu.Unlock()

	if len(cmds) < 5 {
		t.Errorf("ConfigureCompositor() sent %d commands, want >= 5", len(cmds))
	}

	// Test nil client
	var nilClient *HyprlandClient
	nilClient.ConfigureCompositor() // should not panic

	// Test AutoConfigureCompositor
	AutoConfigureCompositor(client)
	AutoConfigureCompositor(nil)
}
