package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

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

var (
	outputLines []string
	outputMu    sync.Mutex
	maxLines    int
)

var spinnerFrames = []string{
	"⠈⠁",
	"⠈⠑",
	"⠈⠱",
	"⠈⡱",
	"⢀⡱",
	"⢄⡱",
	"⢄⡱",
	"⢆⡱",
	"⢎⡱",
	"⢎⡰",
	"⢎⡠",
	"⢎⡀",
	"⢎⠁",
	"⠎⠁",
	"⠊⠁",
}

// Start creates and returns a new Status with an optional parent
func Start(message string, parent *Status) *Status {
	status := &Status{
		message:  message,
		active:   true,
		stopChan: make(chan struct{}),
	}

	if parent != nil {
		parent.mu.Lock()
		status.parent = parent
		status.indentLevel = parent.indentLevel + 1
		parent.children = append(parent.children, status)
		parent.mu.Unlock()
	}

	status.lineIndex = len(outputLines)
	status.render(spinnerFrames[0])
	go status.animate()

	return status
}

// Success marks the status as successful and stops the spinner
func (s *Status) Success(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		s.stopChan <- struct{}{}
		s.active = false
		s.message = message // Update the message before rendering
		s.render("✔️")
	}
}

// Fail marks the status as failed and stops the spinner
func (s *Status) Fail(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		s.stopChan <- struct{}{}
		s.active = false
		s.message = message // Update the message before rendering
		s.render("✗")
	}
}

// animate handles the spinner animation
func (s *Status) animate() {
	frameIndex := 0
	for {
		select {
		case <-s.stopChan:
			return
		default:
			s.mu.Lock()
			if s.active {
				s.render(spinnerFrames[frameIndex])
			}
			s.mu.Unlock()
			frameIndex = (frameIndex + 1) % len(spinnerFrames)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// render updates the status line with the current state
func (s *Status) render(symbol string) {
	content := fmt.Sprintf("%s%s %s",
		getIndentation(s.indentLevel),
		symbol,
		s.message)

	updateLine(s.lineIndex, content)
	refreshScreen()
}

// Helper functions for screen management
func refreshScreen() {
	outputMu.Lock()
	defer outputMu.Unlock()

	if maxLines > 0 {
		fmt.Printf("\033[%dA", maxLines)
	}

	for _, line := range outputLines {
		fmt.Print("\033[2K\r")
		fmt.Println(line)
	}

	for i := len(outputLines); i < maxLines; i++ {
		fmt.Print("\033[2K\r\n")
	}
}

func updateLine(index int, content string) {
	outputMu.Lock()
	defer outputMu.Unlock()

	for len(outputLines) <= index {
		outputLines = append(outputLines, "")
	}
	outputLines[index] = content

	if len(outputLines) > maxLines {
		maxLines = len(outputLines)
	}
}

func getIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat("\t", level) + "└──  "
}
