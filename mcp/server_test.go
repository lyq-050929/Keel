package mcp

import "testing"

func TestDefaultToolsRegistered(t *testing.T) {
	server := NewMCPToolServer()
	tools := server.ListTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 default tools, got %d", len(tools))
	}
}
