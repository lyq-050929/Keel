package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WorkingMemory 工作记忆 — 进程内存储，零延迟。
// 按SessionID隔离，维护当前对话的中间推理状态。
type WorkingMemory struct {
	mu       sync.RWMutex
	context  map[string]map[string]interface{}
	history  map[string][]MemoryEntry
	filePath string
}

type MemoryEntry struct {
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func NewWorkingMemory(filePath string) *WorkingMemory {
	m := &WorkingMemory{
		context:  make(map[string]map[string]interface{}),
		history:  make(map[string][]MemoryEntry),
		filePath: filePath,
	}

	m.loadFromDisk()
	return m
}

func (m *WorkingMemory) Update(sessionID string, data map[string]interface{}) {
	m.mu.Lock()
	if m.context[sessionID] == nil {
		m.context[sessionID] = make(map[string]interface{})
	}
	for k, v := range data {
		m.context[sessionID][k] = v
	}

	snapshotData := make(map[string]interface{}, len(data))
	for k, v := range data {
		snapshotData[k] = v
	}

	m.history[sessionID] = append(m.history[sessionID], MemoryEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      snapshotData,
	})

	if len(m.history[sessionID]) > 50 {
		m.history[sessionID] = m.history[sessionID][len(m.history[sessionID])-50:]
	}
	m.persistLocked()
	m.mu.Unlock()
}

func (m *WorkingMemory) GetContext(sessionID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx := m.context[sessionID]
	if ctx == nil {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{}, len(ctx))
	for k, v := range ctx {
		result[k] = v
	}
	return result
}

func (m *WorkingMemory) Clear(sessionID string) {
	m.mu.Lock()
	delete(m.context, sessionID)
	delete(m.history, sessionID)
	m.persistLocked()
	m.mu.Unlock()
}

func (m *WorkingMemory) loadFromDisk() {
	if m.filePath == "" {
		return
	}

	raw, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}

	var snapshot struct {
		Context map[string]map[string]interface{} `json:"context"`
		History map[string][]MemoryEntry          `json:"history"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return
	}

	if snapshot.Context != nil {
		m.context = snapshot.Context
	}
	if snapshot.History != nil {
		m.history = snapshot.History
	}
}

func (m *WorkingMemory) persistLocked() {
	if m.filePath == "" {
		return
	}

	snapshot := struct {
		Context map[string]map[string]interface{} `json:"context"`
		History map[string][]MemoryEntry          `json:"history"`
	}{
		Context: m.context,
		History: m.history,
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return
	}

	if err := os.WriteFile(m.filePath, payload, 0644); err != nil {
		return
	}
}
