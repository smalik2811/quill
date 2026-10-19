# Quill 🪶

> **Never lift your fingers from the home row again.**  
> A blazing-fast, kernel-level input daemon for Linux power users, tiling window manager enthusiasts, and developers who refuse to reach for a mouse.

---

## ⚡ The Philosophy

Reaching for a mouse or trackpad disrupts your flow state. If you live inside tiling window managers like **Hyprland**, **Sway**, or **i3**, and edit text with **Neovim** or **Emacs**, every moment your hands leave the home row is wasted time and ergonomic strain.

**Quill** eliminates the mouse entirely. By sitting directly on the Linux input subsystem (`evdev` / `uinput`), Quill intercepts hardware keyboard events and transforms your keyboard into a precision virtual mouse with **zero lag**, **zero GUI overhead**, and **pure muscle memory**.

---

## ✨ Features at a Glance

- **Vim Navigation on the Home Row (`H`, `J`, `K`, `L`)**: Full 8-direction cursor movement directly beneath your fingertips.
- **Physical Acceleration S-Curve**: Starts with near-zero acceleration and gentle velocity for single-pixel precision, then ramps smoothly into rapid screen traversal.
- **Dynamic Screen-Edge Friction (Hyprland IPC)**: Real-time proximity awareness slows cursor velocity as you approach screen boundaries, preventing cursor overshooting.
- **Smart 10-Second Auto-Restore**: If you finish a quick mouse task and pause, Quill automatically returns to standard keyboard mode after 10 seconds of inactivity.
- **Zero-Friction Window Manager Combos**: Keys like `Super`/`Command`, `Alt`/`Option`, `Ctrl`, `Shift`, `Fn`, and `Esc` immediately bypass pointer interception—shortcuts like `Super + F` (fullscreen), `Ctrl + C` (copy), or `Super + [1-9]` (workspace switch) work effortlessly in all modes.
- **Dedicated Home-Row Precision Mode (`A`)**: Hold `A` (Aim / Accurate) with your left pinky to lock speed into a steady, non-accelerated crawl ($35\,\text{units/s}$) while moving with `HJKL`.
- **Cross-Platform Linux Hardware Support**: First-class support for generic PC keyboards, ThinkPads, and Apple MacBooks running Arch Linux (including Command, Option, and Globe keys).
- **Embedded Web Curve Visualizer**: Built-in terminal and interactive browser charting utility (`make plot`) to inspect and visualize the motion curves.

---

## 🎮 Keybindings & Controls

Quill switches between **Keyboard Mode** (passthrough) and **Pointer Mode** using `CapsLock`.

```
                  ┌────────────────────────────────────────┐
                  │          CapsLock (on release)         │
                  └──────────────┬───────────────────▲─────┘
                                 │                   │
                                 ▼                   │
                  ┌────────────────────────┐         │
                  │      Pointer Mode      │─────────┘
                  │ (10s Inactivity Return)│
                  └────────────────────────┘
```

### 1. Pointer Mode Toggle
| Key | Action | Description |
|:---|:---|:---|
| <kbd>CapsLock</kbd> | **Toggle Mode** | Toggles into Pointer Mode on key release. When returning to Keyboard Mode, all active keys are flushed cleanly in reverse press order. |
| *Auto-Timeout* | **Auto-Restore (10s)** | Automatically returns to Keyboard Mode after 10 seconds of inactivity. Pressing <kbd>CapsLock</kbd> jumps right back in. |

---

### 2. Cursor Movement (Normal & Precise)
| Key | Direction | Function |
|:---|:---:|:---|
| <kbd>H</kbd> | Left ($\leftarrow$) | Translate cursor left |
| <kbd>J</kbd> | Down ($\downarrow$) | Translate cursor down |
| <kbd>K</kbd> | Up ($\uparrow$) | Translate cursor up |
| <kbd>L</kbd> | Right ($\rightarrow$) | Translate cursor right |
| *Diagonal* | Combinations | Pressing <kbd>H</kbd>+<kbd>K</kbd>, <kbd>L</kbd>+<kbd>J</kbd>, etc., moves diagonally with synchronized reporting. |
| <kbd>A</kbd> *(Hold)* | **Precision Mode (Aim / Accurate)** | Drops acceleration and locks speed to a steady $35\,\text{units/s}$ for micro-positioning, drawing, and UI targeting. Left <kbd>Ctrl</kbd> remains completely free for your regular shortcuts. |

---

### 3. Mouse Clicks & Compound Actions
Quill provides mirrored click buttons for both hands:

| Key | Mouse Button | Typical Use |
|:---|:---|:---|
| <kbd>N</kbd> or <kbd>E</kbd> | **Left Click** (`BTN_LEFT`) | Primary click, focus, double-click, drag & drop. |
| <kbd>M</kbd> or <kbd>R</kbd> | **Right Click** (`BTN_RIGHT`) | Secondary click, context menus. |
| <kbd>Space</kbd> | **Middle Click** (`BTN_MIDDLE`) | Open links in new tab, paste primary clipboard, auto-scroll. |

> **Selection / Drag-and-Drop**: Hold <kbd>N</kbd> (Left Click) with your index finger while moving with <kbd>H</kbd><kbd>J</kbd><kbd>K</kbd><kbd>L</kbd> to highlight text, select multiple files, or drag windows.

---

### 4. Scrolling Mode (<kbd>S</kbd> + Direction)
Hold <kbd>S</kbd> to engage high-resolution scrolling:

| Key Combination | Action |
|:---|:---|
| <kbd>S</kbd> + <kbd>J</kbd> | Scroll Down (`REL_WHEEL`) |
| <kbd>S</kbd> + <kbd>K</kbd> | Scroll Up (`REL_WHEEL`) |
| <kbd>S</kbd> + <kbd>H</kbd> | Scroll Left (`REL_HWHEEL`) |
| <kbd>S</kbd> + <kbd>L</kbd> | Scroll Right (`REL_HWHEEL`) |

---

### 5. Zooming Mode (<kbd>Z</kbd> + Direction)
Hold <kbd>Z</kbd> to zoom in or out in browsers, document viewers, and graphic tools:

| Key Combination | Action |
|:---|:---|
| <kbd>Z</kbd> + <kbd>J</kbd> | Zoom In (`Ctrl + Wheel Up`) |
| <kbd>Z</kbd> + <kbd>K</kbd> | Zoom Out (`Ctrl + Wheel Down`) |

---

### 6. Browser & File History Navigation
Standard mouse side buttons are wired directly to the top row for instant page navigation in Zen Browser, Firefox, Chrome, and file managers:

| Key | Mouse Button | Browser Action |
|:---|:---|:---|
| <kbd>Q</kbd> | `BTN_SIDE` (Button 8) | **Navigate Backward** |
| <kbd>W</kbd> | `BTN_EXTRA` (Button 9) | **Navigate Forward** |

---

### 7. Transparent Window Manager & Modifier Passthrough
In Pointer Mode, only letter keys (<kbd>A</kbd>–<kbd>Z</kbd>) are captured for pointer functions. **All modifier keys and shortcuts pass directly through to your system keyboard:**

- **Control**: <kbd>Ctrl</kbd>+<kbd>C</kbd>, <kbd>Ctrl</kbd>+<kbd>V</kbd>, <kbd>Ctrl</kbd>+<kbd>R</kbd>, terminal shortcuts work as standard without leaving pointer mode.
- **Super / Command**: <kbd>Super</kbd>+<kbd>F</kbd> (toggle fullscreen), <kbd>Super</kbd>+<kbd>Enter</kbd> (terminal), <kbd>Super</kbd>+<kbd>1-9</kbd> (workspace switch) work instantly.
- **Alt / Option**: <kbd>Alt</kbd>+<kbd>Tab</kbd>, application hotkeys.
- **Shift**: <kbd>Shift</kbd>+selection, modifier actions.
- **Mac Keys on Arch Linux**: <kbd>Cmd</kbd>, <kbd>Option</kbd>, and <kbd>Globe/Fn</kbd> keys are recognized and routed cleanly.

---

## 📈 The Velocity Curve & Edge-Friction Physics

Quill models pointer velocity using a cubic S-curve ($p=3.0$, $\tau=650\,\text{ms}$) governed by:

$$v(t) = v_0 + (v_{\max} - v_0) \cdot \left(1 - e^{-(t/\tau)^p}\right)$$

```
Speed (u/s)
  2000 ┼─────────────────────────────────────────────╭─────── (Max Traversal Speed)
       │                                      ╭──────╯
       │                               ╭──────╯
  1000 ┼                        ╭──────╯                      (Midpoint: ~500ms)
       │                 ╭──────╯
       │          ╭──────╯
   300 ┼    ╭─────╯
    60 ┼────╯ (Flat initial micro-control: 0-150ms)
     0 ┼──────┬───────┬───────┬───────┬───────┬───────┬───────
       0ms  150ms   300ms   450ms   600ms   750ms   1000ms    Hold Duration (t)
```

1. **Initial Micro-Control ($0\text{–}150\,\text{ms}$)**: Speed starts at $60\,\text{units/s}$ with near-zero acceleration ($dv/dt \approx 0$). Tapping a key produces single-unit nudges.
2. **Smooth Acceleration ($200\text{–}600\,\text{ms}$)**: Velocity ramps linearly across the display without sudden jumps.
3. **Edge-Friction Damping**: When running under Hyprland, Quill queries cursor position and applies dynamic friction proportional to proximity and hold duration. When nearing screen edges, velocity drops automatically to prevent overshooting.

### Visualizing the Curve
Quill includes a standalone visualizer tool. Run:

```bash
make plot
```
This generates a terminal ASCII graph and writes [`curve.html`](file:///home/sonu/Projects/Development/quill/curve.html). Open it in your browser:
```bash
xdg-open curve.html
```

---

## 🧪 Platform Support & Tested Environments

Quill is actively developed and tested on:
- **Hyprland on Omarchy (Arch Linux)** — Verified with full edge-friction physics and IPC integration.

> [!NOTE]  
> If you have tested Quill on other supported environments (such as **Sway**, **i3**, **River**, or on different distributions and hardware like MacBooks), please open an issue and share your findings! We will gladly update the verified platforms list.

---

## 🛠️ Building & Running

### Requirements
- Linux kernel with `/dev/uinput` enabled
- Go 1.22 or newer
- Access to `evdev` devices (run with `sudo` or add user to `input` group)

### Build from Source
```bash
git clone https://github.com/smalik2811/quill.git
cd quill
make build
```

The compiled binary will be placed at `./build/quill`.

### Run
```bash
sudo ./build/quill
```

### Running Tests
Quill includes unit and invariant test suites ensuring safety, capabilities, and device flushing:
```bash
go test -v ./...
```

---

## 🏗️ Architecture

```
                 /dev/input/event* (Physical Keyboard)
                               │
                               ▼
                   ┌───────────────────────┐
                   │     Quill Router      │
                   └───────────┬───────────┘
                               │
            ┌──────────────────┴──────────────────┐
            │                                     │
   (Keyboard Mode /                      (Pointer Mode &
    Modifier Combos)                      A-Z Letters)
            │                                     │
            ▼                                     ▼
 ┌──────────────────────┐              ┌──────────────────────┐
 │    Quill Keyboard    │              │    Quill Pointer     │
 │ (Virtual Clone /dev) │              │ (/dev/uinput Device) │
 └──────────────────────┘              └──────────┬───────────┘
                                                  │
                                                  ▼
                                       ┌──────────────────────┐
                                       │    Hyprland IPC      │
                                       │ (Cursor / Edge Calc) │
                                       └──────────────────────┘
```

- **Clean Evdev Cloning**: The virtual keyboard dynamically clones all capabilities of your physical hardware.
- **State Preservation**: Transitions between modes flush held keys in strict reverse press order to prevent stuck keys or missed releases.
- **Hyprland Native**: Direct UNIX domain socket IPC provides lightweight edge distance detection with fallback handling if running outside Hyprland.

---

## ⚠️ Known Limitations

- **Non-Deterministic Keyboard Device Capture**: Currently, Quill automatically grabs the first device it finds on `/dev/input/event*` containing `keyboard` in its device name. If you have multiple keyboards connected (e.g. built-in laptop keyboard plus an external mechanical keyboard or wireless receiver), device selection is non-deterministic. Selecting a specific device via CLI flag or config file is coming soon (see Roadmap).

---

## 🗺️ Roadmap & Planned Features

The project is actively evolving. Key capabilities planned for upcoming releases include:

- [ ] **Custom Input Device Selection**: Currently, Quill automatically discovers and grabs the first input device containing `keyboard` in its name. Future releases will allow users to explicitly specify the exact keyboard device path or device identifier (e.g. via CLI flag or config file) to capture input from when multiple keyboards or peripherals are connected.
- [ ] **Systemd Service Integration**: Ship an out-of-the-box systemd user/system service so Quill launches seamlessly on boot.
- [ ] **Custom Keybinding Configuration**: Flexible configuration file (e.g., TOML/YAML) to let users customize all mode triggers, motion keys, clicks, and modifiers.
- [ ] **Status Bar Mode Indicator**: Lightweight status icon / module integration (Waybar, Polybar, eww) to visually indicate whether Quill is in **Keyboard Mode** or **Pointer Mode**.

---

## 💡 Feedback & Feature Requests

> [!NOTE]  
> **We are currently not accepting code contributions or pull requests.**  
> However, we are actively open to suggestions, ideas, and feature requests! If you have a workflow idea or feature you would love to see in Quill, please feel free to open an issue on GitHub.

---

## 📄 License

This project is licensed under the MIT License.
