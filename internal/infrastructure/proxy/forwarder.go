package proxy

import (
	"context"
	"io"
	"net"
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

	// Forward all client headers except hop-by-hop, length, and AuthDeck's own
	// authorization header.
	for key, values := range req.Headers {
		canonical := http.CanonicalHeaderKey(key)
		if hopByHopHeaders[canonical] || canonical == "Content-Length" || canonical == "Host" ||
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

	if clientIP := clientIP(req.RemoteAddr); clientIP != "" {
		httpReq.Header.Set("X-Forwarded-For", clientIP)
	}
	if req.Host != "" {
		httpReq.Header.Set("X-Forwarded-Host", req.Host)
	}
	httpReq.Header.Set("X-Forwarded-Proto", "http")

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
		if hopByHopHeaders[canonical] || canonical == "Content-Length" {
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

func clientIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
