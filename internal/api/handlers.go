package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/core/usecases"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
	"github.com/TheFranconianCoder/auth-deck/internal/types"
)

// respondSelectionError maps a failed TUI provider selection onto an HTTP
// response: an explicit rejection is forbidden, anything else unauthorized.
func respondSelectionError(w http.ResponseWriter, err error) {
	if errors.Is(err, usecases.ErrRejected) {
		respondError(w, types.NewForbiddenError(err.Error()))
		return
	}
	respondError(w, types.NewUnauthorizedError(err.Error()))
}

// respondUsecaseError maps typed use-case failures onto HTTP statuses:
// unknown provider is a usage error (400), a misconfigured provider is a
// server error (500), upstream failures are 502, and auth timeouts are 504.
func respondUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecases.ErrProviderNotFound):
		respondError(w, types.NewBadRequestError(err.Error()))
	case errors.Is(err, usecases.ErrProviderConfig):
		respondError(w, types.NewInternalError(err.Error()))
	case errors.Is(err, usecases.ErrUpstream):
		respondError(w, types.NewBadGatewayError(err.Error()))
	case errors.Is(err, usecases.ErrAuthTimeout):
		respondError(w, types.NewGatewayTimeoutError(err.Error()))
	default:
		respondError(w, types.NewInternalError(err.Error()))
	}
}

func (r *Router) handleToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		respondError(w, types.NewMethodNotAllowedError("POST required"))
		return
	}

	if err := req.ParseForm(); err != nil {
		respondError(w, types.NewBadRequestError("invalid request body"))
		return
	}
	if grantType := req.FormValue("grant_type"); grantType != "" && grantType != "client_credentials" {
		respondError(w, types.NewBadRequestError("only the client_credentials grant is supported"))
		return
	}

	selection, err := r.selector.Select(req.Context(), usecases.SelectInput{
		Type:      "TOKEN",
		Method:    "TOKEN",
		Path:      "/token",
		GrantType: "client_credentials",
	})
	if err != nil {
		respondSelectionError(w, err)
		return
	}
	r.writeProviderToken(w, req, selection.Provider)
}

func (r *Router) handleTokenDirect(w http.ResponseWriter, req *http.Request) {
	provider := strings.TrimPrefix(req.URL.Path, "/token/")
	if provider == "" {
		respondError(w, types.NewBadRequestError("provider name required"))
		return
	}
	r.writeProviderToken(w, req, provider)
}

// writeProviderToken resolves a token for a specific provider without asking
// the TUI.
func (r *Router) writeProviderToken(w http.ResponseWriter, req *http.Request, provider string) {
	path := req.URL.Path

	if _, ok := r.catalog.Get(provider); !ok {
		r.queue.AddLog(state.LogEntry{Time: time.Now(), Method: "TOKEN", Path: path, Provider: provider, Err: "provider not found"})
		respondError(w, types.NewBadRequestError(fmt.Sprintf("unknown provider %q", provider)))
		return
	}

	start := time.Now()
	token, err := r.tokens.Obtain(req.Context(), provider)
	if err != nil {
		r.queue.AddLog(state.LogEntry{Time: time.Now(), Method: "TOKEN", Path: path, Provider: provider, Err: err.Error()})
		respondUsecaseError(w, err)
		return
	}
	r.queue.AddLog(state.LogEntry{
		Time:     time.Now(),
		Method:   "TOKEN",
		Path:     path,
		Provider: provider,
		Status:   http.StatusOK,
		Err:      fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
	})
	writeToken(w, token)
}

func (r *Router) handleCallback(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	code := query.Get("code")
	stateID := query.Get("state")

	if errParam := query.Get("error"); errParam != "" {
		respondError(w, types.NewBadRequestError(fmt.Sprintf("%s: %s", errParam, query.Get("error_description"))))
		return
	}
	if code == "" {
		respondError(w, types.NewBadRequestError("no authorization code received"))
		return
	}

	// The callback only serves AuthDeck's own interactive upstream flows.
	if !r.tokens.DeliverCode(stateID, code) {
		respondError(w, types.NewBadRequestError("unknown or expired authorization state"))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<html><body><h1>Authorization complete!</h1><p>You can close this window.</p></body></html>`)
}

// handleProxy forwards an ad-hoc request whose provider is chosen in the TUI.
func (r *Router) handleProxy(w http.ResponseWriter, req *http.Request) {
	path := "/" + strings.TrimPrefix(req.URL.Path, "/proxy/")

	selection, err := r.selector.Select(req.Context(), usecases.SelectInput{
		Type:      "PROXY",
		Method:    req.Method,
		Path:      path,
		GrantType: "client_credentials",
	})
	if err != nil {
		respondSelectionError(w, err)
		return
	}
	r.forward(w, req, selection.Provider, path)
}

// handleDirectProxy forwards a request whose provider is pinned in the path,
// so clients can point their API base URL at /direct/{provider} without any
// AuthDeck-specific header.
func (r *Router) handleDirectProxy(w http.ResponseWriter, req *http.Request) {
	provider, path, ok := splitProvider(strings.TrimPrefix(req.URL.Path, "/direct/"))
	if !ok {
		respondError(w, types.NewBadRequestError("provider name required"))
		return
	}
	if _, ok := r.catalog.Get(provider); !ok {
		respondError(w, types.NewBadRequestError(fmt.Sprintf("unknown provider %q", provider)))
		return
	}
	r.forward(w, req, provider, path)
}

// forward reads the request body and proxies it to the provider's upstream API.
func (r *Router) forward(w http.ResponseWriter, req *http.Request, provider, path string) {
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()

	out, err := r.proxy.Forward(req.Context(), usecases.ForwardInput{
		Provider: provider,
		Method:   req.Method,
		Path:     path,
		RawQuery: req.URL.RawQuery,
		Headers:  req.Header,
		Body:     body,
	})
	if err != nil {
		respondUsecaseError(w, err)
		return
	}

	for key, values := range out.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(out.StatusCode)
	w.Write(out.Body)
}

// splitProvider splits "name/rest" into the provider name and the upstream
// path ("/rest"). It reports false when no provider segment is present.
func splitProvider(rest string) (provider, path string, ok bool) {
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return "", "", false
	}
	provider, tail, found := strings.Cut(rest, "/")
	if provider == "" {
		return "", "", false
	}
	if !found {
		return provider, "/", true
	}
	return provider, "/" + tail, true
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<html><body><h1>AuthDeck</h1><p>OAuth 2.0 Local Token Proxy</p></body></html>`)
}
