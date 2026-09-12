package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
)

// FileStore is an in-memory token store that persists to a JSON file so tokens
// survive a restart.
type FileStore struct {
	mu     sync.RWMutex
	path   string
	tokens map[string]*entities.Token
}

type persistedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope"`
}

func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path, tokens: make(map[string]*entities.Token)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) Get(provider string) (*entities.Token, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[provider]
	return token, ok
}

// All returns a snapshot of every stored token, keyed by provider.
func (s *FileStore) All() map[string]*entities.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*entities.Token, len(s.tokens))
	for provider, token := range s.tokens {
		out[provider] = token
	}
	return out
}

func (s *FileStore) Set(provider string, token *entities.Token) {
	s.mu.Lock()
	s.tokens[provider] = token
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		log.Printf("token store: failed to persist %s: %v", s.path, err)
	}
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read token store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var raw map[string]persistedToken
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse token store: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for provider, t := range raw {
		s.tokens[provider] = &entities.Token{
			AccessToken:  t.AccessToken,
			RefreshToken: t.RefreshToken,
			TokenType:    t.TokenType,
			ExpiresAt:    t.ExpiresAt,
			Scope:        t.Scope,
		}
	}
	return nil
}

func (s *FileStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create token store dir: %w", err)
	}

	raw := make(map[string]persistedToken, len(s.tokens))
	for provider, t := range s.tokens {
		raw[provider] = persistedToken{
			AccessToken:  t.AccessToken,
			RefreshToken: t.RefreshToken,
			TokenType:    t.TokenType,
			ExpiresAt:    t.ExpiresAt,
			Scope:        t.Scope,
		}
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode token store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write token store: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace token store: %w", err)
	}
	return nil
}
