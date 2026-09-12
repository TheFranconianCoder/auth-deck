package state

import (
	"fmt"
	"sync/atomic"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
)

// Events are notifications emitted by use cases and consumed by the TUI.
// They are intentionally transport-agnostic plain structs.

type RequestAdded struct {
	Request *PendingRequest
}

type RequestDone struct {
	ID       string
	Provider string
}

type BrowserOpen struct {
	URL      string
	Provider string
}

type BrowserClosed struct{}

type TokenUpdated struct {
	Provider string
	Token    *entities.Token
	Err      string
}

// ReauthRequired signals that a provider's token expired and cannot be renewed
// silently, so an interactive login is needed.
type ReauthRequired struct {
	Provider string
}

var idCounter atomic.Int64

// NewID returns a process-unique identifier.
func NewID() string {
	return fmt.Sprintf("%d", idCounter.Add(1))
}
