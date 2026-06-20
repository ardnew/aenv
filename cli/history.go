package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// history records evaluated inputs and recalls them by navigation.
//
// index points one past the newest entry while the user edits a draft; prev and
// next move it. An empty path keeps history in memory only.
type history struct {
	path    string
	entries []string
	index   int
	draft   string
}

// loadHistory reads entries from path. A missing or unreadable file yields an
// empty history. An empty path keeps history in memory only.
func loadHistory(path string) history {
	h := history{path: path}
	if path == "" {
		return h
	}
	file, err := os.Open(path)
	if err != nil {
		return h
	}
	defer func() { _ = file.Close() }()

	sin := bufio.NewScanner(file)
	sin.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sin.Scan() {
		line := sin.Text()
		if line == "" {
			continue
		}
		entry, err := strconv.Unquote(line)
		if err != nil {
			continue
		}
		h.entries = append(h.entries, entry)
	}
	_ = sin.Err()
	h.index = len(h.entries)
	return h
}

// record stores input unless it is blank or repeats the newest entry. It resets
// navigation and persists accepted entries.
func (h *history) record(input string) {
	h.index = len(h.entries)
	h.draft = ""
	if strings.TrimSpace(input) == "" {
		return
	}
	if n := len(h.entries); n > 0 && h.entries[n-1] == input {
		return
	}
	h.entries = append(h.entries, input)
	h.index = len(h.entries)
	h.persist(input)
}

// prev returns the previous entry and true, saving current as the draft when
// leaving it. It returns false at the oldest entry.
func (h *history) prev(current string) (string, bool) {
	if h.index == 0 {
		return "", false
	}
	if h.index == len(h.entries) {
		h.draft = current
	}
	h.index--
	return h.entries[h.index], true
}

// next returns the next entry and true, or the saved draft at the newest entry.
// It returns false once the draft is reached.
func (h *history) next() (string, bool) {
	if h.index >= len(h.entries) {
		return "", false
	}
	h.index++
	if h.index == len(h.entries) {
		return h.draft, true
	}
	return h.entries[h.index], true
}

func (h *history) persist(entry string) {
	if h.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString(strconv.Quote(entry) + "\n")
}
