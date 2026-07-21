package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/smartcs/go-impl/agent"
	"github.com/smartcs/go-impl/config"
	"github.com/smartcs/go-impl/mcp"
	"github.com/smartcs/go-impl/memory"
)

func TestServerChatFlow(t *testing.T) {
	server := newTestServer(t)

	health := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	server.engine.ServeHTTP(health, req)
	if health.Code != http.StatusOK {
		t.Fatalf("expected health 200, got %d", health.Code)
	}

	body := bytes.NewBufferString(`{"message":"我想退款","user_id":"u1"}`)
	chat := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/chat", body)
	req.Header.Set("Content-Type", "application/json")
	server.engine.ServeHTTP(chat, req)
	if chat.Code != http.StatusOK {
		t.Fatalf("expected chat 200, got %d: %s", chat.Code, chat.Body.String())
	}

	var chatResp struct {
		Intent    string `json:"intent"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(chat.Body.Bytes(), &chatResp); err != nil {
		t.Fatal(err)
	}
	if chatResp.Intent != "ticket_handler" {
		t.Fatalf("expected ticket_handler intent, got %s", chatResp.Intent)
	}
	if chatResp.SessionID == "" {
		t.Fatal("expected generated session id")
	}

	history := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/history/"+chatResp.SessionID, nil)
	server.engine.ServeHTTP(history, req)
	if history.Code != http.StatusOK {
		t.Fatalf("expected history 200, got %d", history.Code)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	dataDir := t.TempDir()
	workingMem := memory.NewWorkingMemory(filepath.Join(dataDir, "working.json"))
	shortTermMem := memory.NewShortTermMemory("", filepath.Join(dataDir, "short.json"))
	longTermMem := memory.NewLongTermMemory()
	longTermMem.AddDocument("退款政策：用户在购买后7天内可申请无理由退款。", "refund_policy.md")

	intentRouter := agent.NewIntentRouterAgent()
	knowledgeAgent := agent.NewKnowledgeRAGAgent(longTermMem)
	ticketAgent := agent.NewTicketHandlerAgent(filepath.Join(dataDir, "tickets.json"))
	complianceAgent := agent.NewComplianceCheckerAgent()
	supervisor := agent.NewSupervisorAgent(intentRouter, knowledgeAgent, ticketAgent, complianceAgent, workingMem)

	cfg := config.Config{
		Port:           "0",
		DataDir:        dataDir,
		RequestTimeout: 30 * time.Second,
	}

	return NewServer(supervisor, shortTermMem, longTermMem, mcp.NewMCPToolServer(), cfg)
}
