package cli

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// Input related constants
const (
	enterKey     = 13
	backspaceKey = 127
	deleteKey    = 8
	ctrlC        = 3
)

// InputState manages the current input state
type inputState struct {
	active bool
	prompt string
	buffer string
	mu     sync.Mutex
}

// Global input state
var input = &inputState{}

// EnableInput displays an input prompt and handles user input
func PromptInput(prompt string) string {
	input.mu.Lock()
	input.active = true
	input.prompt = fmt.Sprintf("%s: ", prompt)
	input.buffer = ""
	input.mu.Unlock()

	display.refresh()
	return handleRawInput()
}

// handleRawInput manages raw terminal input
func handleRawInput() string {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var inputBytes []byte
	buffer := make([]byte, 1)

	for {
		if _, err := os.Stdin.Read(buffer); err != nil {
			panic(err)
		}

		switch buffer[0] {
		case enterKey:
			cleanupInput()
			return string(inputBytes)
		case backspaceKey, deleteKey:
			inputBytes = handleBackspace(inputBytes)
		case ctrlC:
			cleanupInput()
			term.Restore(int(os.Stdin.Fd()), oldState)
			os.Exit(0)
		default:
			inputBytes = handleCharacter(inputBytes, buffer[0])
		}
	}
}

// handleBackspace processes backspace/delete key press
func handleBackspace(input []byte) []byte {
	if len(input) > 0 {
		input = input[:len(input)-1]
		updateInputBuffer(string(input))
	}
	return input
}

// handleCharacter processes printable characters
func handleCharacter(input []byte, char byte) []byte {
	if char >= 32 && char <= 126 {
		input = append(input, char)
		updateInputBuffer(string(input))
	}
	return input
}

// updateInputBuffer updates the current input buffer and refreshes the display
func updateInputBuffer(content string) {
	input.mu.Lock()
	input.buffer = content
	input.mu.Unlock()
	display.refresh()
}

// cleanupInput resets the input state
func cleanupInput() {
	input.mu.Lock()
	input.active = false
	input.prompt = ""
	input.buffer = ""
	input.mu.Unlock()

	display.maxLines = len(display.lines)
	display.refresh()
}
