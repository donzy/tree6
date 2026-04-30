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
	hotkeyMu      sync.Mutex
}

func NewRecorder() *Recorder {
	return &Recorder{
		isRecording:  false,
		events:       make([]RecordEvent, 0),
		outputDir:    "./recordings",
		ctrlPressed:  false,
		shiftPressed: false,
	}
}

func (r *Recorder) Start() {
	_ = os.MkdirAll(r.outputDir, 0755)

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
}

func (r *Recorder) handleKeyDown(ev hook.Event) {
	r.hotkeyMu.Lock()
	defer r.hotkeyMu.Unlock()

	isCtrl := isCtrlKey(ev.Rawcode)
	isShift := isShiftKey(ev.Rawcode)
	isB := ev.Rawcode == 11
	isE := ev.Rawcode == 14

	if isCtrl {
		r.ctrlPressed = true
	}
	if isShift {
		r.shiftPressed = true
	}

	if r.ctrlPressed && r.shiftPressed {
		if isB {
			r.StartRecording()
			return
		} else if isE {
			r.StopRecording()
			return
		}
	}

	if r.IsRecording() && !isCtrl && !isShift {
		keyChar := string(rune(ev.Keychar))
		if keyChar == "" {
			keyChar = fmt.Sprintf("KeyCode:%d", ev.Rawcode)
		}

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

	if isCtrl {
		r.ctrlPressed = false
	}
	if isShift {
		r.shiftPressed = false
	}
}

func isCtrlKey(rawcode uint16) bool {
	return rawcode == 59 || rawcode == 62
}

func isShiftKey(rawcode uint16) bool {
	return rawcode == 56 || rawcode == 60
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

	fmt.Println("Recording started. Press Ctrl+Shift+E to stop.")
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

	fmt.Println("Recording stopped.")
	r.saveEvents()
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
	set frontWindow to front window of frontApp
	set windowName to name of frontWindow
	set windowPos to position of frontWindow
	set windowSize to size of frontWindow
	set windowX to item 1 of windowPos
	set windowY to item 2 of windowPos
	set windowWidth to item 1 of windowSize
	set windowHeight to item 2 of windowSize
	return appName & ", " & windowName & ", " & windowX & ", " & windowY & ", " & windowWidth & ", " & windowHeight
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
			}`, event.WindowInfo.AppName, event.WindowInfo.WindowName,
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
			event.Key, event.MouseButton, event.MouseX, event.MouseY,
			windowInfoStr, screenshotStr, comma)
	}
	fmt.Fprintf(file, "]\n")

	fmt.Printf("Events saved to: %s\n", filename)
}
