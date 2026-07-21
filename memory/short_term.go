package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ShortTermMemory stores recent conversation history.
// Redis is used when available, with an in-memory fallback for local/dev runs.
type ShortTermMemory struct {
	mu          sync.RWMutex
	store       map[string][]Message
	maxTurns    int
	redisClient *redis.Client
	redisTTL    time.Duration
	filePath    string
}

type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func NewShortTermMemory(redisURL, filePath string) *ShortTermMemory {
	stm := &ShortTermMemory{
		store:    make(map[string][]Message),
		maxTurns: 20,
		redisTTL: 24 * time.Hour,
		filePath: filePath,
	}

	if client, err := newRedisClient(redisURL); err == nil {
		stm.redisClient = client
	}

	stm.loadFromDisk()

	return stm
}

func (m *ShortTermMemory) AddMessage(sessionID, role, content string) {
	msg := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	m.mu.Lock()
	m.store[sessionID] = append(m.store[sessionID], msg)
	if len(m.store[sessionID]) > m.maxTurns {
		m.store[sessionID] = m.store[sessionID][len(m.store[sessionID])-m.maxTurns:]
	}
	history := make([]Message, len(m.store[sessionID]))
	copy(history, m.store[sessionID])
	snapshot := make(map[string][]Message, len(m.store))
	for k, v := range m.store {
		copySlice := make([]Message, len(v))
		copy(copySlice, v)
		snapshot[k] = copySlice
	}
	m.mu.Unlock()

	m.persistHistory(sessionID, history)
	m.persistSnapshot(snapshot)
}

func (m *ShortTermMemory) GetHistory(sessionID string) []Message {
	m.mu.RLock()
	msgs := m.store[sessionID]
	result := make([]Message, len(msgs))
	copy(result, msgs)
	m.mu.RUnlock()

	if len(result) > 0 {
		return result
	}

	if history, ok := m.loadHistoryFromRedis(sessionID); ok {
		return history
	}

	return result
}

func (m *ShortTermMemory) GetHistoryJSON(sessionID string) string {
	msgs := m.GetHistory(sessionID)
	data, _ := json.Marshal(msgs)
	return string(data)
}

func (m *ShortTermMemory) Clear(sessionID string) {
	key := m.redisKey(sessionID)
	m.mu.Lock()
	delete(m.store, sessionID)
	snapshot := make(map[string][]Message, len(m.store))
	for k, v := range m.store {
		copySlice := make([]Message, len(v))
		copy(copySlice, v)
		snapshot[k] = copySlice
	}
	m.mu.Unlock()

	if m.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = m.redisClient.Del(ctx, key).Err()
	}

	m.persistSnapshot(snapshot)
}

func (m *ShortTermMemory) persistHistory(sessionID string, history []Message) {
	if m.redisClient == nil {
		return
	}

	payload, err := json.Marshal(history)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = m.redisClient.Set(ctx, m.redisKey(sessionID), payload, m.redisTTL).Err()
}

func (m *ShortTermMemory) persistSnapshot(snapshot map[string][]Message) {
	if m.filePath == "" {
		return
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

func (m *ShortTermMemory) loadFromDisk() {
	if m.filePath == "" {
		return
	}

	raw, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}

	var snapshot map[string][]Message
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range snapshot {
		copySlice := make([]Message, len(v))
		copy(copySlice, v)
		m.store[k] = copySlice
	}
}

func (m *ShortTermMemory) loadHistoryFromRedis(sessionID string) ([]Message, bool) {
	if m.redisClient == nil {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	raw, err := m.redisClient.Get(ctx, m.redisKey(sessionID)).Bytes()
	if err != nil {
		return nil, false
	}

	var history []Message
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, false
	}

	return history, true
}

func (m *ShortTermMemory) redisKey(sessionID string) string {
	return "short_term:" + sessionID
}

func newRedisClient(redisURL string) (*redis.Client, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is empty")
	}

	if !strings.Contains(redisURL, "://") {
		redisURL = "redis://" + redisURL
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
