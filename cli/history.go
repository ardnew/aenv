package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ardnew/aenv/log"
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
	log.Trace(log.Attrs(
		"index", h.index,
		"count", len(h.entries),
		"len", len(input),
	), "history record")

	h.index = len(h.entries)
	h.draft = ""
	if strings.TrimSpace(input) == "" {
		log.Trace(log.Attrs("reason", "blank"), "history skip")
		return
	}
	if n := len(h.entries); n > 0 && h.entries[n-1] == input {
		log.Trace(log.Attrs("reason", "duplicate"), "history skip")
		return
	}
	h.entries = append(h.entries, input)
	h.index = len(h.entries)
	log.Trace(log.Attrs(
		"index", h.index,
		"count", len(h.entries),
	), "history stored")
	h.persist(input)
}

// prev returns the previous entry and true, saving current as the draft when
// leaving it. It returns false at the oldest entry.
func (h *history) prev(current string) (string, bool) {
	if h.index == 0 {
		log.Trace(log.Attrs("index", h.index, "count", len(h.entries)), "history prev boundary")
		return "", false
	}
	usedDraft := h.index == len(h.entries)
	if h.index == len(h.entries) {
		h.draft = current
	}
	h.index--
	log.Trace(log.Attrs(
		"index", h.index,
		"count", len(h.entries),
		"capture-draft", usedDraft,
	), "history prev")
	return h.entries[h.index], true
}

// next returns the next entry and true, or the saved draft at the newest entry.
// It returns false once the draft is reached.
func (h *history) next() (string, bool) {
	if h.index >= len(h.entries) {
		log.Trace(log.Attrs("index", h.index, "count", len(h.entries)), "history next boundary")
		return "", false
	}
	h.index++
	log.Trace(log.Attrs("index", h.index, "count", len(h.entries)), "history next")
	if h.index == len(h.entries) {
		return h.draft, true
	}
	return h.entries[h.index], true
}

func (h *history) persist(entry string) {
	if h.path == "" {
		log.Trace(log.Attrs("reason", "memory-only"), "history persist skip")
		return
	}
	log.Trace(log.Attrs("path", h.path, "len", len(entry)), "history persist")
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
