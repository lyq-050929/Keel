package agent

import "testing"

func TestIntentRouterRoutesTicketKeywords(t *testing.T) {
	router := NewIntentRouterAgent()
	state := NewState("u1", "s1", "我要退款并投诉")
	out := router.Process(state)
	if out.Intent != "ticket_handler" {
		t.Fatalf("expected ticket_handler, got %s", out.Intent)
	}
}

func TestIntentRouterDefaultsToKnowledge(t *testing.T) {
	router := NewIntentRouterAgent()
	state := NewState("u1", "s1", "你好")
	out := router.Process(state)
	if out.Intent != "knowledge_rag" {
		t.Fatalf("expected knowledge_rag, got %s", out.Intent)
	}
}
