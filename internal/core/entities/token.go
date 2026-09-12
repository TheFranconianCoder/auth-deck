package entities

import "time"

type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
	Scope        string
}

func (t *Token) IsValid() bool {
	return t.AccessToken != "" && time.Now().Before(t.ExpiresAt)
}

func (t *Token) ExpiresIn() int64 {
	remaining := time.Until(t.ExpiresAt).Seconds()
	if remaining < 0 {
		return 0
	}
	return int64(remaining)
}
