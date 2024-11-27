package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Terminal control sequences
const (
    clearLine    = "\033[2K\r"
    moveCursorUp = "\033[%dA"
)

// DisplayConfig holds configuration for visual elements
type DisplayConfig struct {
    IndentString  string
    SpinnerFrames []string
}

// Default display configuration
var defaultDisplay = DisplayConfig{
    IndentString: "└──  ",
    SpinnerFrames: []string{
        "⠈⠁", "⠈⠑", "⠈⠱", "⠈⡱", "⢀⡱",
        "⢄⡱", "⢄⡱", "⢆⡱", "⢎⡱", "⢎⡰",
        "⢎⡠", "⢎⡀", "⢎⠁", "⠎⠁", "⠊⠁",
    },
}

// Status represents a progress indicator with optional child statuses
type Status struct {
    message     string
    indentLevel int
    lineIndex   int
    active      bool
    stopChan    chan struct{}
    mu          sync.Mutex
    children    []*Status
    parent      *Status
}

// Display state management
type displayState struct {
    lines    []string
    maxLines int
    mu       sync.Mutex
}

// Global display state
var display = &displayState{}

// Display methods
func (d *displayState) updateLine(index int, content string) {
    d.mu.Lock()
    defer d.mu.Unlock()

    for len(d.lines) <= index {
        d.lines = append(d.lines, "")
    }
    d.lines[index] = content

    if len(d.lines) > d.maxLines {
        d.maxLines = len(d.lines)
    }
}

func (d *displayState) refresh() {
    d.mu.Lock()
    defer d.mu.Unlock()

    if d.maxLines > 0 {
        fmt.Printf(moveCursorUp, d.maxLines)
    }

    // Print status lines
    for _, line := range d.lines {
        fmt.Print(clearLine)
        fmt.Println(line)
    }

    // Fill empty lines
    for i := len(d.lines); i < d.maxLines; i++ {
        fmt.Print(clearLine + "\n")
    }

    // Handle input line
    fmt.Print(clearLine)
    if input.active {
        fmt.Printf("%s%s", input.prompt, input.buffer)
    }
}

// Status methods
func Start(message string, parent *Status) *Status {
    status := &Status{
        message:  message,
        active:   true,
        stopChan: make(chan struct{}),
    }

    if parent != nil {
        status.attachToParent(parent)
    }

    status.lineIndex = len(display.lines)
    status.render(defaultDisplay.SpinnerFrames[0])
    go status.animate()

    return status
}

func (s *Status) attachToParent(parent *Status) {
    parent.mu.Lock()
    defer parent.mu.Unlock()

    s.parent = parent
    s.indentLevel = parent.indentLevel + 1
    parent.children = append(parent.children, s)
}

func (s *Status) Success(message string) {
    s.updateStatus(message, "✔️")
}

func (s *Status) Fail(message string) {
    s.updateStatus(message, "✗")
}

func (s *Status) updateStatus(message, symbol string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.active {
        s.stopChan <- struct{}{}
        s.active = false
        s.message = message
        s.render(symbol)
    }
}

func (s *Status) animate() {
    frameIndex := 0
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-s.stopChan:
            return
        case <-ticker.C:
            s.mu.Lock()
            if s.active {
                s.render(defaultDisplay.SpinnerFrames[frameIndex])
                frameIndex = (frameIndex + 1) % len(defaultDisplay.SpinnerFrames)
            }
            s.mu.Unlock()
        }
    }
}

func (s *Status) render(symbol string) {
    content := fmt.Sprintf("%s%s %s",
        getIndentation(s.indentLevel),
        symbol,
        s.message)

    display.updateLine(s.lineIndex, content)
    display.refresh()
}

// Utility functions
func getIndentation(level int) string {
    if level <= 0 {
        return ""
    }
    return strings.Repeat("\t", level) + defaultDisplay.IndentString
}
