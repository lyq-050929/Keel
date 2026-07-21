package memory

import (
	"path/filepath"
	"testing"
)

func TestShortTermMemoryPersistsToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short_term.json")

	mem := NewShortTermMemory("", path)
	mem.AddMessage("sess-1", "user", "hello")

	mem2 := NewShortTermMemory("", path)
	history := mem2.GetHistory("sess-1")
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Content != "hello" {
		t.Fatalf("expected persisted message content hello, got %q", history[0].Content)
	}
}
