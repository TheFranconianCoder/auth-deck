package entities

type FlowType string

const (
	FlowAuthorizationCode FlowType = "authorization_code"
	FlowClientCredentials FlowType = "client_credentials"
)

const (
	DefaultTokenPath     = "access_token"
	DefaultTokenTypePath = "token_type"
	DefaultExpiresPath   = "expires_in"
)

type Provider struct {
	Name          string
	ClientID      string
	ClientSecret  string
	AuthURL       string
	TokenURL      string
	RedirectURI   string
	Scopes        []string
	BaseURL       string
	Flow          FlowType
	Resource      string
	TokenPath     string
	TokenTypePath string
	ExpiresPath   string
	PKCE          bool
	Prompt        string
}
