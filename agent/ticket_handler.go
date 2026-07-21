package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smartcs/go-impl/tracing"
)

// TicketHandlerAgent 工单处理Agent — 工单CRUD与流转。
type TicketHandlerAgent struct {
	mu       sync.RWMutex
	tickets  map[string]*Ticket
	counter  int
	filePath string
}

type Ticket struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Summary   string `json:"summary"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func NewTicketHandlerAgent(filePath string) *TicketHandlerAgent {
	a := &TicketHandlerAgent{
		tickets:  make(map[string]*Ticket),
		filePath: filePath,
	}

	a.loadFromDisk()
	return a
}

func (a *TicketHandlerAgent) Process(state *State) *State {
	return tracing.TraceFunc("ticket_handler", "process", func() *State {
		ticket := a.createTicket(state.UserID, state.UserMessage)

		result := fmt.Sprintf(
			"工单已创建成功！\n\n"+
				"工单号: %s\n"+
				"状态: 已创建\n"+
				"优先级: 中等\n"+
				"创建时间: %s\n\n"+
				"我们将尽快处理您的请求，请保存好工单号以便后续查询。",
			ticket.ID, ticket.CreatedAt,
		)

		state.SubResults["ticket_handler"] = result
		return state
	})
}

func (a *TicketHandlerAgent) createTicket(userID, summary string) *Ticket {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.counter++
	now := time.Now()
	ticketID := fmt.Sprintf("TK-%s-%04d", now.Format("20060102"), a.counter)

	ticket := &Ticket{
		ID:        ticketID,
		UserID:    userID,
		Summary:   summary,
		Priority:  "medium",
		Status:    "created",
		CreatedAt: now.Format("2006-01-02 15:04:05"),
	}

	a.tickets[ticketID] = ticket
	a.persistLocked()
	return ticket
}

func (a *TicketHandlerAgent) loadFromDisk() {
	if a.filePath == "" {
		return
	}

	raw, err := os.ReadFile(a.filePath)
	if err != nil {
		return
	}

	var snapshot struct {
		Counter int                `json:"counter"`
		Tickets map[string]*Ticket `json:"tickets"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if snapshot.Counter > 0 {
		a.counter = snapshot.Counter
	}
	if snapshot.Tickets != nil {
		a.tickets = snapshot.Tickets
	}
}

func (a *TicketHandlerAgent) persistLocked() {
	if a.filePath == "" {
		return
	}

	snapshot := struct {
		Counter int                `json:"counter"`
		Tickets map[string]*Ticket `json:"tickets"`
	}{
		Counter: a.counter,
		Tickets: a.tickets,
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(a.filePath), 0755); err != nil {
		return
	}

	if err := os.WriteFile(a.filePath, payload, 0644); err != nil {
		return
	}
}

func (a *TicketHandlerAgent) Name() string {
	return "ticket_handler"
}
