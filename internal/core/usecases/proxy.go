package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

// ProxyService forwards an incoming request to the selected provider's upstream
// API, injecting a valid bearer token.
type ProxyService struct {
	catalog   ProviderCatalog
	tokens    *TokenService
	forwarder Forwarder
	queue     RequestQueue
}

func NewProxyService(catalog ProviderCatalog, tokens *TokenService, forwarder Forwarder, queue RequestQueue) *ProxyService {
	return &ProxyService{catalog: catalog, tokens: tokens, forwarder: forwarder, queue: queue}
}

func (s *ProxyService) Forward(ctx context.Context, in ForwardInput) (ForwardOutput, error) {
	p, ok := s.catalog.Get(in.Provider)
	if !ok {
		return ForwardOutput{}, fmt.Errorf("provider %q not found", in.Provider)
	}

	token, err := s.tokens.Obtain(ctx, in.Provider)
	if err != nil {
		s.queue.AddLog(state.LogEntry{
			Time:     time.Now(),
			Method:   in.Method,
			Path:     in.Path,
			Provider: in.Provider,
			Err:      err.Error(),
		})
		return ForwardOutput{}, err
	}

	target := strings.TrimSuffix(p.BaseURL, "/") + "/" + strings.TrimPrefix(in.Path, "/")
	if in.RawQuery != "" {
		target += "?" + in.RawQuery
	}

	start := time.Now()
	resp, err := s.forwarder.Do(ctx, ForwardRequest{
		Method:     in.Method,
		URL:        target,
		Headers:    in.Headers,
		Body:       in.Body,
		Token:      token,
		RemoteAddr: in.RemoteAddr,
		Host:       in.Host,
	})
	if err != nil {
		s.queue.AddLog(state.LogEntry{
			Time:     time.Now(),
			Method:   in.Method,
			Path:     in.Path,
			Provider: in.Provider,
			Err:      err.Error(),
		})
		return ForwardOutput{}, err
	}

	s.queue.AddLog(state.LogEntry{
		Time:     time.Now(),
		Method:   in.Method,
		Path:     in.Path,
		Provider: in.Provider,
		Status:   resp.StatusCode,
		Err:      fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
	})

	return ForwardOutput{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
	}, nil
}
