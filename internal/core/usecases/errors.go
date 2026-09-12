package usecases

import "errors"

// Sentinel errors let the transport adapter map failures onto the right HTTP
// status without inspecting error strings.
var (
	// ErrProviderNotFound means the caller referenced a provider that is not
	// configured. This is a client/usage error.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrProviderConfig means a configured provider is missing required
	// settings. This is a server-side configuration error.
	ErrProviderConfig = errors.New("provider misconfigured")
	// ErrUpstream means an upstream token or API request failed.
	ErrUpstream = errors.New("upstream request failed")
	// ErrAuthTimeout means an interactive authorization did not complete in time.
	ErrAuthTimeout = errors.New("authorization timed out")
)
