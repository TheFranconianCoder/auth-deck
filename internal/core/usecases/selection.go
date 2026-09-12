package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

// ProviderSelector enqueues a request for the TUI and blocks until a provider
// is chosen (or the request times out / is cancelled).
type ProviderSelector struct {
	queue    RequestQueue
	notifier Notifier
}

func NewProviderSelector(queue RequestQueue, notifier Notifier) *ProviderSelector {
	return &ProviderSelector{queue: queue, notifier: notifier}
}

func (s *ProviderSelector) Select(ctx context.Context, in SelectInput) (SelectOutput, error) {
	req := &state.PendingRequest{
		ID:         state.NewID(),
		Type:       in.Type,
		Method:     in.Method,
		Path:       in.Path,
		GrantType:  in.GrantType,
		ProviderCh: make(chan string, 1),
		CreatedAt:  time.Now(),
	}

	s.queue.Add(req)
	s.notify(state.RequestAdded{Request: req})

	select {
	case provider := <-req.ProviderCh:
		s.queue.Remove(req.ID)
		s.notify(state.RequestDone{ID: req.ID, Provider: provider})
		return SelectOutput{Provider: provider}, nil
	case <-time.After(2 * time.Minute):
		s.queue.Remove(req.ID)
		s.notify(state.RequestDone{ID: req.ID, Provider: "timeout"})
		return SelectOutput{}, fmt.Errorf("timeout waiting for provider selection")
	case <-ctx.Done():
		s.queue.Remove(req.ID)
		s.notify(state.RequestDone{ID: req.ID, Provider: "cancelled"})
		return SelectOutput{}, ctx.Err()
	}
}

func (s *ProviderSelector) notify(event any) {
	if s.notifier != nil {
		s.notifier.Notify(event)
	}
}
