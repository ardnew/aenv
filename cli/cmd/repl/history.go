package repl

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

const baseHistory = "history.utf8"

// HistoryEntry represents a single history entry with its mode.
type HistoryEntry struct {
	Line string
	Mode inputMode
}

// History manages command history with file persistence.
// It implements the github.com/lmorg/readline.History interface.
type History struct {
	path    string
	entries []HistoryEntry
	mu      sync.RWMutex
}

// NewHistory creates a new History instance with the given file path.
func NewHistory(path string) *History {
	return &History{path: path}
}

// Load reads history entries from the history file.
func (h *History) Load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	file, err := os.Open(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	h.entries = nil

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse mode prefix (E: for eval, C: for ctrl)
		var (
			mode    inputMode
			content string
		)

		if s, ok := strings.CutPrefix(line, "E:"); ok {
			mode = modeEval
			content = s
		} else if s, ok := strings.CutPrefix(line, "C:"); ok {
			mode = modeCtrl
			content = s
		} else {
			// Legacy format without mode prefix - assume eval mode
			mode = modeEval
			content = line
		}

		h.entries = append(h.entries, HistoryEntry{
			Line: content,
			Mode: mode,
		})
	}

	return scanner.Err()
}

// Write appends a new entry to the history with its mode.
// Implements readline.History interface.
func (h *History) Write(entry string) (int, error) {
	return h.WriteWithMode(entry, modeEval)
}

// WriteWithMode appends a new entry to the history with the specified mode.
// If a duplicate entry exists (same line and mode), it removes the old one.
func (h *History) WriteWithMode(entry string, mode inputMode) (int, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return 0, nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Skip if same as last entry (both line and mode)
	if len(h.entries) > 0 {
		last := h.entries[len(h.entries)-1]
		if last.Line == entry && last.Mode == mode {
			return len(entry), nil
		}
	}

	// Remove any existing duplicate entry (same line and mode)
	needsRewrite := false

	for i := 0; i < len(h.entries); i++ {
		if h.entries[i].Line == entry && h.entries[i].Mode == mode {
			// Remove this entry
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			needsRewrite = true

			break
		}
	}

	// Add new entry
	h.entries = append(h.entries, HistoryEntry{
		Line: entry,
		Mode: mode,
	})

	// If we removed a duplicate, rewrite the entire file
	// Otherwise, just append
	if needsRewrite {
		return h.rewriteFile()
	}

	// Append to file with mode prefix
	file, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var prefix string
	if mode == modeCtrl {
		prefix = "C:"
	} else {
		prefix = "E:"
	}

	n, err := file.WriteString(prefix + entry + "\n")

	return n, err
}

// GetLine retrieves a historic line by index.
// Index 0 is the oldest entry.
// Implements readline.History interface.
func (h *History) GetLine(i int) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if i < 0 || i >= len(h.entries) {
		return "", ErrOutOfBounds
	}

	return h.entries[i].Line, nil
}

// GetEntry retrieves a historic entry (line and mode) by index.
// Index 0 is the oldest entry.
func (h *History) GetEntry(i int) (HistoryEntry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if i < 0 || i >= len(h.entries) {
		return HistoryEntry{}, ErrOutOfBounds
	}

	return h.entries[i], nil
}

// Len returns the number of history entries.
// Implements readline.History interface.
func (h *History) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.entries)
}

// Dump exports the entire history.
// Implements readline.History interface.
func (h *History) Dump() any {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]string, len(h.entries))
	for i, entry := range h.entries {
		result[i] = entry.Line
	}

	return result
}

// Entries returns all history entries.
func (h *History) Entries() []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]HistoryEntry, len(h.entries))
	copy(result, h.entries)

	return result
}

// rewriteFile rewrites the entire history file with current entries.
// Must be called with h.mu held.
func (h *History) rewriteFile() (int, error) {
	file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	totalBytes := 0

	for _, entry := range h.entries {
		var prefix string
		if entry.Mode == modeCtrl {
			prefix = "C:"
		} else {
			prefix = "E:"
		}

		n, err := file.WriteString(prefix + entry.Line + "\n")
		if err != nil {
			return totalBytes, err
		}

		totalBytes += n
	}

	return totalBytes, nil
}
