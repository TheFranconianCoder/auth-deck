package state

import "sync"

type authEntry struct {
	provider string
	codeCh   chan string
}

// AuthStates correlates an in-flight authorization_code flow (by OAuth state)
// with the channel that receives the authorization code.
type AuthStates struct {
	mu     sync.Mutex
	states map[string]*authEntry
}

func NewAuthStates() *AuthStates {
	return &AuthStates{states: make(map[string]*authEntry)}
}

func (a *AuthStates) Register(state, provider string) chan string {
	ch := make(chan string, 1)
	a.mu.Lock()
	a.states[state] = &authEntry{provider: provider, codeCh: ch}
	a.mu.Unlock()
	return ch
}

func (a *AuthStates) Unregister(state string) {
	a.mu.Lock()
	delete(a.states, state)
	a.mu.Unlock()
}

// Deliver routes an authorization code to a waiting flow. It reports whether a
// matching flow was found.
func (a *AuthStates) Deliver(state, code string) bool {
	a.mu.Lock()
	entry, ok := a.states[state]
	a.mu.Unlock()
	if !ok {
		return false
	}
	entry.codeCh <- code
	return true
}
