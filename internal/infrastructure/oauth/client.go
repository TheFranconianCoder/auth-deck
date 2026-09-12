package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
)

// Client talks to OAuth 2.0 token endpoints using the standard form-encoded
// grant requests, honoring provider-specific parameters and response paths.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) ClientCredentials(ctx context.Context, p *entities.Provider) (*entities.Token, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}
	if len(p.Scopes) > 0 {
		data.Set("scope", strings.Join(p.Scopes, " "))
	}
	if p.Resource != "" {
		data.Set("resource", p.Resource)
	}
	return c.tokenRequest(ctx, p, data)
}

func (c *Client) Refresh(ctx context.Context, p *entities.Provider, refreshToken string) (*entities.Token, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}
	if p.Resource != "" {
		data.Set("resource", p.Resource)
	}
	return c.tokenRequest(ctx, p, data)
}

func (c *Client) ExchangeCode(ctx context.Context, p *entities.Provider, code, codeVerifier string) (*entities.Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.RedirectURI},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}
	if p.Resource != "" {
		data.Set("resource", p.Resource)
	}
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}
	return c.tokenRequest(ctx, p, data)
}

func (c *Client) tokenRequest(ctx context.Context, p *entities.Provider, data url.Values) (*entities.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if errCode := stringAt(payload, "error"); errCode != "" {
		return nil, fmt.Errorf("token error: %s - %s", errCode, stringAt(payload, "error_description"))
	}

	accessToken, ok := stringPath(payload, p.TokenPath)
	if !ok || accessToken == "" {
		return nil, fmt.Errorf("access token not found at path %q in token response", p.TokenPath)
	}

	tokenType := stringAt(payload, p.TokenTypePath)
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return &entities.Token{
		AccessToken:  accessToken,
		RefreshToken: stringAt(payload, "refresh_token"),
		TokenType:    tokenType,
		ExpiresAt:    time.Now().Add(time.Duration(int64At(payload, p.ExpiresPath)) * time.Second),
		Scope:        stringAt(payload, "scope"),
	}, nil
}

// resolvePath walks a dotted path through nested JSON objects.
func resolvePath(data map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	var cur any = data
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func stringPath(data map[string]any, path string) (string, bool) {
	v, ok := resolvePath(data, path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func stringAt(data map[string]any, path string) string {
	s, _ := stringPath(data, path)
	return s
}

func int64At(data map[string]any, path string) int64 {
	v, ok := resolvePath(data, path)
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}
