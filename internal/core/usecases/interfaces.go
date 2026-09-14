package usecases

import (
	"context"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

// Ports implemented by adapters.

type ProviderCatalog interface {
	Get(name string) (*entities.Provider, bool)
	Names() []string
}

type TokenStore interface {
	Get(provider string) (*entities.Token, bool)
	Set(provider string, token *entities.Token)
	All() map[string]*entities.Token
}

type TokenClient interface {
	ClientCredentials(ctx context.Context, provider *entities.Provider) (*entities.Token, error)
	Refresh(ctx context.Context, provider *entities.Provider, refreshToken string) (*entities.Token, error)
	ExchangeCode(ctx context.Context, provider *entities.Provider, code, codeVerifier string) (*entities.Token, error)
}

type Browser interface {
	Open(url string) error
}

type AuthStateStore interface {
	Register(state, provider string) chan string
	Unregister(state string)
	Deliver(state, code string) bool
}

type RequestQueue interface {
	Add(req *state.PendingRequest)
	Remove(id string)
	Pending() []*state.PendingRequest
	AddLog(entry state.LogEntry)
	Logs() []state.LogEntry
}

type Forwarder interface {
	Do(ctx context.Context, req ForwardRequest) (ForwardResponse, error)
}

// Notifier receives UI events from use cases.
type Notifier interface {
	Notify(event any)
}

// BrowserFunc adapts a plain function to the Browser port.
type BrowserFunc func(url string) error

func (f BrowserFunc) Open(url string) error { return f(url) }

// NotifierFunc adapts a plain function to the Notifier port.
type NotifierFunc func(event any)

func (f NotifierFunc) Notify(event any) { f(event) }

// DTOs.

type SelectInput struct {
	Type      string
	Method    string
	Path      string
	GrantType string
}

type SelectOutput struct {
	Provider string
}

type ForwardInput struct {
	Provider string
	Method   string
	Path     string
	RawQuery string
	Headers  map[string][]string
	Body     []byte
}

type ForwardOutput struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type ForwardRequest struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    []byte
	Token   *entities.Token
}

type ForwardResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}
