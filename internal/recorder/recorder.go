package recorder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	hook "github.com/robotn/gohook"
)

type RecordEvent struct {
	Timestamp   time.Time   `json:"timestamp"`
	Type        string      `json:"type"` // "keyboard", "mouse", "start", "stop"
	Key         string      `json:"key,omitempty"`
	MouseButton string      `json:"mouse_button,omitempty"`
	MouseX      int         `json:"mouse_x,omitempty"`
	MouseY      int         `json:"mouse_y,omitempty"`
	WindowInfo  *WindowInfo `json:"window_info,omitempty"`
	Screenshot  string      `json:"screenshot,omitempty"` // file path to screenshot
}

type WindowInfo struct {
	AppName    string `json:"app_name"`
	WindowName string `json:"window_name"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type Recorder struct {
	isRecording   bool
	isRecordingMu sync.Mutex
	events        []RecordEvent
	outputDir     string
	ctrlPressed   bool
	shiftPressed  bool
	altPressed    bool
	hotkeyMu      sync.Mutex
}

func NewRecorder() *Recorder {
	return &Recorder{
		isRecording:  false,
		events:       make([]RecordEvent, 0),
		outputDir:    "./recordings",
		ctrlPressed:  false,
		shiftPressed: false,
		altPressed:   false,
	}
}

func (r *Recorder) Start() {
	_ = os.MkdirAll(r.outputDir, 0755)

	fmt.Println("Global hook starting...")
	fmt.Println("Listening for keyboard events...")
	fmt.Println("Hotkeys:")
	fmt.Println("  Ctrl+Alt+B = Start recording")
	fmt.Println("  Ctrl+Alt+E = Stop recording")
	fmt.Println("  Ctrl+C = Exit program")
	fmt.Println()

	go r.setupGlobalHook()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	r.StopRecording()
	hook.End()
}

func (r *Recorder) setupGlobalHook() {
	evChan := hook.Start()
	defer hook.End()

	fmt.Println("Global hook started, waiting for events...")

	for ev := range evChan {
		switch ev.Kind {
		case hook.KeyDown:
			r.handleKeyDown(ev)
		case hook.KeyUp:
			r.handleKeyUp(ev)
		case hook.MouseDown:
			r.handleMouseEvent(ev)
		}
	}

	fmt.Println("Global hook channel closed")
}

func (r *Recorder) handleKeyDown(ev hook.Event) {
	r.hotkeyMu.Lock()
	defer r.hotkeyMu.Unlock()

	keyChar := string(rune(ev.Keychar))
	if keyChar == "" {
		keyChar = fmt.Sprintf("KeyCode:%d", ev.Rawcode)
	}
	fmt.Printf("[DEBUG] KeyDown: rawcode=%d, keychar=%c, key=%s\n", ev.Rawcode, ev.Keychar, keyChar)

	isCtrl := isCtrlKey(ev.Rawcode)
	isShift := isShiftKey(ev.Rawcode)
	isAlt := isAltKey(ev.Rawcode)
	isB := ev.Rawcode == 11
	isE := ev.Rawcode == 14

	if isCtrl {
		r.ctrlPressed = true
		fmt.Println("[DEBUG] Ctrl pressed")
	}
	if isShift {
		r.shiftPressed = true
		fmt.Println("[DEBUG] Shift pressed")
	}
	if isAlt {
		r.altPressed = true
		fmt.Println("[DEBUG] Alt pressed")
	}

	fmt.Printf("[DEBUG] State: ctrl=%v, alt=%v, shift=%v\n", r.ctrlPressed, r.altPressed, r.shiftPressed)

	if r.ctrlPressed && r.altPressed {
		if isB {
			fmt.Println("[DEBUG] Ctrl+Alt+B detected - Starting recording")
			r.StartRecording()
			return
		} else if isE {
			fmt.Println("[DEBUG] Ctrl+Alt+E detected - Stopping recording")
			r.StopRecording()
			return
		}
	}

	if r.IsRecording() && !isCtrl && !isShift && !isAlt {
		event := RecordEvent{
			Timestamp: time.Now(),
			Type:      "keyboard",
			Key:       keyChar,
		}

		windowInfo, err := r.getActiveWindowInfo()
		if err == nil {
			event.WindowInfo = windowInfo
		}

		screenshotPath, err := r.captureScreenshot()
		if err == nil {
			event.Screenshot = screenshotPath
		}

		r.addEvent(event)
		fmt.Printf("Key pressed: %s\n", keyChar)
	}
}

func (r *Recorder) handleKeyUp(ev hook.Event) {
	r.hotkeyMu.Lock()
	defer r.hotkeyMu.Unlock()

	isCtrl := isCtrlKey(ev.Rawcode)
	isShift := isShiftKey(ev.Rawcode)
	isAlt := isAltKey(ev.Rawcode)

	if isCtrl {
		r.ctrlPressed = false
		fmt.Println("[DEBUG] Ctrl released")
	}
	if isShift {
		r.shiftPressed = false
		fmt.Println("[DEBUG] Shift released")
	}
	if isAlt {
		r.altPressed = false
		fmt.Println("[DEBUG] Alt released")
	}
}

func isCtrlKey(rawcode uint16) bool {
	return rawcode == 59 || rawcode == 62
}

func isShiftKey(rawcode uint16) bool {
	return rawcode == 56 || rawcode == 60
}

func isAltKey(rawcode uint16) bool {
	return rawcode == 58 || rawcode == 61
}

func (r *Recorder) handleMouseEvent(ev hook.Event) {
	if !r.IsRecording() {
		return
	}

	button := "left"
	switch ev.Button {
	case 2:
		button = "right"
	case 3:
		button = "middle"
	}

	x, y := int(ev.X), int(ev.Y)

	event := RecordEvent{
		Timestamp:   time.Now(),
		Type:        "mouse",
		MouseButton: button,
		MouseX:      x,
		MouseY:      y,
	}

	windowInfo, err := r.getActiveWindowInfo()
	if err == nil {
		event.WindowInfo = windowInfo
	}

	screenshotPath, err := r.captureScreenshot()
	if err == nil {
		event.Screenshot = screenshotPath
	}

	r.addEvent(event)
	fmt.Printf("Mouse clicked: %s at (%d, %d)\n", button, x, y)
}

func (r *Recorder) IsRecording() bool {
	r.isRecordingMu.Lock()
	defer r.isRecordingMu.Unlock()
	return r.isRecording
}

func (r *Recorder) StartRecording() {
	r.isRecordingMu.Lock()
	defer r.isRecordingMu.Unlock()

	if r.isRecording {
		fmt.Println("Recording is already in progress.")
		return
	}

	r.isRecording = true
	r.events = make([]RecordEvent, 0)

	startEvent := RecordEvent{
		Timestamp: time.Now(),
		Type:      "start",
	}
	r.events = append(r.events, startEvent)

	fmt.Println("========================================")
	fmt.Println("Recording started.")
	fmt.Println("All keyboard and mouse events will be recorded.")
	fmt.Println("Press Ctrl+Alt+E to stop recording.")
	fmt.Println("========================================")
}

func (r *Recorder) StopRecording() {
	r.isRecordingMu.Lock()
	defer r.isRecordingMu.Unlock()

	if !r.isRecording {
		return
	}

	r.isRecording = false

	stopEvent := RecordEvent{
		Timestamp: time.Now(),
		Type:      "stop",
	}
	r.events = append(r.events, stopEvent)

	fmt.Println("========================================")
	fmt.Println("Recording stopped.")
	fmt.Printf("Total events recorded: %d\n", len(r.events))
	r.saveEvents()
	fmt.Println("========================================")
}

func (r *Recorder) addEvent(event RecordEvent) {
	r.isRecordingMu.Lock()
	defer r.isRecordingMu.Unlock()
	r.events = append(r.events, event)
}

func (r *Recorder) getActiveWindowInfo() (*WindowInfo, error) {
	script := `
tell application "System Events"
	set frontApp to first application process whose frontmost is true
	set appName to name of frontApp
	try
		set frontWindow to front window of frontApp
		set windowName to name of frontWindow
		set windowPos to position of frontWindow
		set windowSize to size of frontWindow
		set windowX to item 1 of windowPos
		set windowY to item 2 of windowPos
		set windowWidth to item 1 of windowSize
		set windowHeight to item 2 of windowSize
		return appName & ", " & windowName & ", " & windowX & ", " & windowY & ", " & windowWidth & ", " & windowHeight
	on error
		return appName & ", , 0, 0, 0, 0"
	end try
end tell
`

	cmd := exec.Command("osascript", "-e", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	result := strings.TrimSpace(out.String())
	parts := strings.Split(result, ", ")
	if len(parts) < 6 {
		return nil, fmt.Errorf("unexpected AppleScript output: %s", result)
	}

	appName := strings.TrimSpace(parts[0])
	windowName := strings.TrimSpace(parts[1])
	x, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
	y, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
	width, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
	height, _ := strconv.Atoi(strings.TrimSpace(parts[5]))

	return &WindowInfo{
		AppName:    appName,
		WindowName: windowName,
		X:          x,
		Y:          y,
		Width:      width,
		Height:     height,
	}, nil
}

func (r *Recorder) captureScreenshot() (string, error) {
	timestamp := time.Now().Format("20060102-150405.000000")
	filename := fmt.Sprintf("%s/screenshot_%s.png", r.outputDir, timestamp)

	cmd := exec.Command("screencapture", "-x", filename)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return filename, nil
}

func (r *Recorder) saveEvents() {
	filename := fmt.Sprintf("%s/events_%s.json", r.outputDir, time.Now().Format("20060102-150405"))

	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating events file: %v\n", err)
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "[\n")
	for i, event := range r.events {
		windowInfoStr := "null"
		if event.WindowInfo != nil {
			windowInfoStr = fmt.Sprintf(`{
				"app_name": "%s",
				"window_name": "%s",
				"x": %d,
				"y": %d,
				"width": %d,
				"height": %d
			}`, escapeJSON(event.WindowInfo.AppName), escapeJSON(event.WindowInfo.WindowName),
				event.WindowInfo.X, event.WindowInfo.Y,
				event.WindowInfo.Width, event.WindowInfo.Height)
		}

		screenshotStr := "null"
		if event.Screenshot != "" {
			screenshotStr = fmt.Sprintf(`"%s"`, event.Screenshot)
		}

		comma := ","
		if i == len(r.events)-1 {
			comma = ""
		}

		fmt.Fprintf(file, `{
			"timestamp": "%s",
			"type": "%s",
			"key": "%s",
			"mouse_button": "%s",
			"mouse_x": %d,
			"mouse_y": %d,
			"window_info": %s,
			"screenshot": %s
		}%s
		`, event.Timestamp.Format(time.RFC3339Nano), event.Type,
			escapeJSON(event.Key), escapeJSON(event.MouseButton), event.MouseX, event.MouseY,
			windowInfoStr, screenshotStr, comma)
	}
	fmt.Fprintf(file, "]\n")

	fmt.Printf("Events saved to: %s\n", filename)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
