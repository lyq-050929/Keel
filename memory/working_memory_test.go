package memory

import (
	"path/filepath"
	"testing"
)

func TestWorkingMemoryPersistsToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "working_memory.json")

	mem := NewWorkingMemory(path)
	mem.Update("sess-1", map[string]interface{}{"intent": "knowledge_rag"})

	mem2 := NewWorkingMemory(path)
	ctx := mem2.GetContext("sess-1")
	if ctx["intent"] != "knowledge_rag" {
		t.Fatalf("expected persisted intent knowledge_rag, got %#v", ctx["intent"])
	}
}
