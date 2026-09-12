package state

import (
	"sync"
	"time"
)

// Decision is the TUI's answer to a pending request: either a chosen provider
// or an explicit rejection.
type Decision struct {
	Provider string
	Rejected bool
}

// PendingRequest represents an incoming proxy/token request awaiting a
// provider decision in the TUI. ProviderCh carries the decision.
type PendingRequest struct {
	ID         string
	Type       string
	Method     string
	Path       string
	GrantType  string
	ProviderCh chan Decision
	CreatedAt  time.Time
}

type LogEntry struct {
	Time     time.Time
	Method   string
	Path     string
	Provider string
	Status   int
	Err      string
}

// RequestQueue holds pending requests and recent log entries for the TUI.
type RequestQueue struct {
	mu      sync.Mutex
	pending []*PendingRequest
	logs    []LogEntry
}

func NewRequestQueue() *RequestQueue {
	return &RequestQueue{}
}

func (q *RequestQueue) Add(req *PendingRequest) {
	q.mu.Lock()
	q.pending = append(q.pending, req)
	q.mu.Unlock()
}

func (q *RequestQueue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, r := range q.pending {
		if r.ID == id {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			return
		}
	}
}

func (q *RequestQueue) Pending() []*PendingRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*PendingRequest, len(q.pending))
	copy(out, q.pending)
	return out
}

func (q *RequestQueue) AddLog(entry LogEntry) {
	q.mu.Lock()
	q.logs = append(q.logs, entry)
	if len(q.logs) > 100 {
		q.logs = q.logs[len(q.logs)-100:]
	}
	q.mu.Unlock()
}

func (q *RequestQueue) Logs() []LogEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]LogEntry, len(q.logs))
	copy(out, q.logs)
	return out
}
