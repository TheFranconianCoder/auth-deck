package proxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/core/usecases"
)

// hopByHopHeaders must not be forwarded to or from the upstream (RFC 7230).
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// forwardedHeaders are proxy-specific and must not be passed through: AuthDeck
// is a local client, not a reverse proxy in front of the upstream.
var forwardedHeaders = map[string]bool{
	"X-Forwarded-For":   true,
	"X-Forwarded-Host":  true,
	"X-Forwarded-Proto": true,
}

// Forwarder performs the outbound HTTP call to an upstream API.
type Forwarder struct {
	http *http.Client
}

func NewForwarder() *Forwarder {
	return &Forwarder{http: &http.Client{Timeout: 30 * time.Second}}
}

func (f *Forwarder) Do(ctx context.Context, req usecases.ForwardRequest) (usecases.ForwardResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, strings.NewReader(string(req.Body)))
	if err != nil {
		return usecases.ForwardResponse{}, err
	}

	// Forward all client headers except hop-by-hop, proxy-specific, length, and
	// AuthDeck's own authorization header.
	for key, values := range req.Headers {
		canonical := http.CanonicalHeaderKey(key)
		if hopByHopHeaders[canonical] || forwardedHeaders[canonical] ||
			canonical == "Content-Length" || canonical == "Host" ||
			canonical == "Authorization" {
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(canonical, value)
		}
	}

	if req.Token != nil {
		tokenType := req.Token.TokenType
		if tokenType == "" {
			tokenType = "Bearer"
		}
		httpReq.Header.Set("Authorization", tokenType+" "+req.Token.AccessToken)
	}

	resp, err := f.http.Do(httpReq)
	if err != nil {
		return usecases.ForwardResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return usecases.ForwardResponse{}, err
	}

	headers := make(map[string][]string, len(resp.Header))
	for key, values := range resp.Header {
		canonical := http.CanonicalHeaderKey(key)
		if hopByHopHeaders[canonical] || forwardedHeaders[canonical] || canonical == "Content-Length" {
			continue
		}
		headers[key] = values
	}

	return usecases.ForwardResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}
