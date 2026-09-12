package usecases

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

const (
	defaultRedirectURI = "http://127.0.0.1:9090/callback"
	refreshThreshold   = 5 * time.Minute
	renewCooldown      = time.Minute
	reauthCooldown     = 5 * time.Minute
)

// TokenService resolves access tokens: cache first, then refresh, then the
// provider's configured flow (client credentials or interactive authorization).
type TokenService struct {
	catalog    ProviderCatalog
	store      TokenStore
	client     TokenClient
	browser    Browser
	authStates AuthStateStore
	notifier   Notifier

	mu       sync.Mutex
	cooldown map[string]time.Time
}

func NewTokenService(
	catalog ProviderCatalog,
	store TokenStore,
	client TokenClient,
	browser Browser,
	authStates AuthStateStore,
	notifier Notifier,
) *TokenService {
	return &TokenService{
		catalog:    catalog,
		store:      store,
		client:     client,
		browser:    browser,
		authStates: authStates,
		notifier:   notifier,
		cooldown:   make(map[string]time.Time),
	}
}

// Obtain returns a valid token for the provider, reusing the cache and
// refreshing when possible. Authorization-code providers fall back to an
// interactive browser flow.
func (s *TokenService) Obtain(ctx context.Context, provider string) (*entities.Token, error) {
	p, ok := s.catalog.Get(provider)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrProviderNotFound, provider)
	}

	if token, ok := s.store.Get(provider); ok {
		if token.IsValid() {
			return token, nil
		}
		if refreshed, err := s.tryRefresh(ctx, p, token); err == nil {
			return refreshed, nil
		}
	}

	if p.Flow == entities.FlowAuthorizationCode {
		return s.authorizeInteractive(ctx, p)
	}

	token, err := s.client.ClientCredentials(ctx, p)
	if err != nil {
		s.notify(state.TokenUpdated{Provider: provider, Err: err.Error()})
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	s.storeAndNotify(provider, token)
	return token, nil
}

// RefreshNow forces a fresh token for a user-initiated fetch, ignoring the
// cache: it refreshes via an existing refresh token, re-mints client
// credentials, or starts an interactive login. The resolved token is always
// reported to the UI.
func (s *TokenService) RefreshNow(ctx context.Context, provider string) (*entities.Token, error) {
	p, ok := s.catalog.Get(provider)
	if !ok {
		err := fmt.Errorf("%w: %q", ErrProviderNotFound, provider)
		s.notify(state.TokenUpdated{Provider: provider, Err: err.Error()})
		return nil, err
	}

	token, err := s.forceRefresh(ctx, p)
	if err != nil {
		s.notify(state.TokenUpdated{Provider: provider, Err: err.Error()})
		return nil, err
	}
	return token, nil
}

func (s *TokenService) forceRefresh(ctx context.Context, p *entities.Provider) (*entities.Token, error) {
	if current, ok := s.store.Get(p.Name); ok && current.RefreshToken != "" {
		if refreshed, err := s.tryRefresh(ctx, p, current); err == nil {
			return refreshed, nil
		}
	}

	switch p.Flow {
	case entities.FlowClientCredentials:
		token, err := s.client.ClientCredentials(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
		}
		s.storeAndNotify(p.Name, token)
		return token, nil
	case entities.FlowAuthorizationCode:
		return s.authorizeInteractive(ctx, p)
	default:
		return nil, fmt.Errorf("%w: unsupported flow %q for provider %q", ErrProviderConfig, p.Flow, p.Name)
	}
}

// Exchange swaps an authorization code for a token. codeVerifier is the PKCE
// verifier when PKCE was used, otherwise empty.
func (s *TokenService) Exchange(ctx context.Context, provider, code, codeVerifier string) (*entities.Token, error) {
	p, ok := s.catalog.Get(provider)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrProviderNotFound, provider)
	}
	token, err := s.client.ExchangeCode(ctx, p, code, codeVerifier)
	if err != nil {
		s.notify(state.TokenUpdated{Provider: provider, Err: err.Error()})
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	s.storeAndNotify(provider, token)
	return token, nil
}

// DeliverCode routes an authorization code from the OAuth callback to the
// waiting interactive flow.
func (s *TokenService) DeliverCode(stateID, code string) bool {
	return s.authStates.Deliver(stateID, code)
}

// AutoRefresh periodically renews cached tokens that are close to expiry.
// It never opens a browser: client-credentials tokens are re-minted, tokens
// with a refresh token are refreshed, and expiring interactive tokens without
// a usable refresh token trigger a re-login notification instead.
func (s *TokenService) AutoRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run an immediate pass so persisted tokens are renewed right after start.
	s.refreshAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshAll(ctx)
		}
	}
}

func (s *TokenService) refreshAll(ctx context.Context) {
	for _, name := range s.catalog.Names() {
		s.renew(ctx, name)
	}
}

// renew handles a single provider in the background. Only providers that have
// already produced a token are touched, keeping unused providers lazy.
func (s *TokenService) renew(ctx context.Context, name string) {
	token, ok := s.store.Get(name)
	if !ok {
		return
	}
	if time.Until(token.ExpiresAt) > refreshThreshold {
		return
	}
	if s.inCooldown(name) {
		return
	}

	p, ok := s.catalog.Get(name)
	if !ok {
		return
	}

	// Prefer a refresh token when the provider issued one.
	if token.RefreshToken != "" {
		if _, err := s.tryRefresh(ctx, p, token); err == nil {
			return
		}
	}

	switch p.Flow {
	case entities.FlowClientCredentials:
		refreshed, err := s.client.ClientCredentials(ctx, p)
		if err != nil {
			s.setCooldown(name, renewCooldown)
			s.notify(state.TokenUpdated{Provider: name, Err: err.Error()})
			return
		}
		s.storeAndNotify(name, refreshed)
	case entities.FlowAuthorizationCode:
		// Cannot renew silently; surface a re-login request.
		s.setCooldown(name, reauthCooldown)
		s.notify(state.ReauthRequired{Provider: name})
	}
}

// tryRefresh exchanges an existing refresh token for a new token.
func (s *TokenService) tryRefresh(ctx context.Context, p *entities.Provider, token *entities.Token) (*entities.Token, error) {
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token for %q", p.Name)
	}
	refreshed, err := s.client.Refresh(ctx, p, token.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	s.storeAndNotify(p.Name, refreshed)
	return refreshed, nil
}

func (s *TokenService) storeAndNotify(provider string, token *entities.Token) {
	s.store.Set(provider, token)
	s.notify(state.TokenUpdated{Provider: provider, Token: token})
}

func (s *TokenService) inCooldown(provider string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.cooldown[provider]
	return ok && time.Now().Before(until)
}

func (s *TokenService) setCooldown(provider string, d time.Duration) {
	s.mu.Lock()
	s.cooldown[provider] = time.Now().Add(d)
	s.mu.Unlock()
}

func (s *TokenService) authorizeInteractive(ctx context.Context, p *entities.Provider) (*entities.Token, error) {
	if p.AuthURL == "" {
		return nil, fmt.Errorf("%w: auth_url not configured for provider %q", ErrProviderConfig, p.Name)
	}

	stateID := state.NewID()
	redirectURI := p.RedirectURI
	if redirectURI == "" {
		redirectURI = defaultRedirectURI
	}

	codeVerifier := ""
	codeChallenge := ""
	if p.PKCE {
		verifier, challenge, err := newPKCE()
		if err != nil {
			return nil, fmt.Errorf("generate PKCE challenge: %w", err)
		}
		codeVerifier = verifier
		codeChallenge = challenge
	}
	authURL := buildAuthURL(p, stateID, redirectURI, codeChallenge)

	codeCh := s.authStates.Register(stateID, p.Name)
	defer func() {
		s.authStates.Unregister(stateID)
		s.notify(state.BrowserClosed{})
	}()

	s.notify(state.BrowserOpen{URL: authURL, Provider: p.Name})
	if err := s.browser.Open(authURL); err != nil {
		// Non-fatal: the URL is shown in the TUI for manual opening.
		s.notify(state.TokenUpdated{Provider: p.Name, Err: "browser open failed: " + err.Error()})
	}

	select {
	case code := <-codeCh:
		return s.Exchange(ctx, p.Name, code, codeVerifier)
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("%w: waiting for auth callback", ErrAuthTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *TokenService) notify(event any) {
	if s.notifier != nil {
		s.notifier.Notify(event)
	}
}

// buildAuthURL assembles the authorization request, including PKCE parameters
// when a code challenge is provided.
func buildAuthURL(p *entities.Provider, stateID, redirectURI, codeChallenge string) string {
	q := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(p.Scopes, " ")},
		"state":         {stateID},
	}
	if p.Resource != "" {
		q.Set("resource", p.Resource)
	}
	if p.Prompt != "" {
		q.Set("prompt", p.Prompt)
	}
	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}
	return p.AuthURL + "?" + q.Encode()
}
