package cli

import (
	"path/filepath"
	"testing"
)

func TestHistory_RecordSkipsBlankAndDuplicates(t *testing.T) {
	h := loadHistory("")
	h.record("first")
	h.record("first")
	h.record("   ")
	h.record("")
	h.record("second")

	want := []string{"first", "second"}
	if len(h.entries) != len(want) {
		t.Fatalf("entries = %v, want %v", h.entries, want)
	}
	for i, entry := range want {
		if h.entries[i] != entry {
			t.Fatalf("entries[%d] = %q, want %q", i, h.entries[i], entry)
		}
	}
}

func TestHistory_PrevNextWithDraft(t *testing.T) {
	h := loadHistory("")
	h.record("one")
	h.record("two")

	if v, ok := h.prev("draft"); !ok || v != "two" {
		t.Fatalf("prev() = %q,%v, want \"two\",true", v, ok)
	}
	if v, ok := h.prev("draft"); !ok || v != "one" {
		t.Fatalf("prev() = %q,%v, want \"one\",true", v, ok)
	}
	if v, ok := h.prev("draft"); ok {
		t.Fatalf("prev() at oldest = %q,%v, want \"\",false", v, ok)
	}
	if v, ok := h.next(); !ok || v != "two" {
		t.Fatalf("next() = %q,%v, want \"two\",true", v, ok)
	}
	if v, ok := h.next(); !ok || v != "draft" {
		t.Fatalf("next() = %q,%v, want \"draft\",true", v, ok)
	}
	if v, ok := h.next(); ok {
		t.Fatalf("next() at draft = %q,%v, want \"\",false", v, ok)
	}
}

func TestHistory_PersistRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "history")

	h := loadHistory(path)
	h.record("plain")
	h.record("multi\nline")

	reloaded := loadHistory(path)
	want := []string{"plain", "multi\nline"}
	if len(reloaded.entries) != len(want) {
		t.Fatalf("reloaded entries = %v, want %v", reloaded.entries, want)
	}
	for i, entry := range want {
		if reloaded.entries[i] != entry {
			t.Fatalf("reloaded entries[%d] = %q, want %q", i, reloaded.entries[i], entry)
		}
	}
	if reloaded.index != len(want) {
		t.Fatalf("reloaded index = %d, want %d", reloaded.index, len(want))
	}
}
