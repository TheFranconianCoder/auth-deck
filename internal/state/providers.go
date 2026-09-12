package state

import "github.com/TheFranconianCoder/auth-deck/internal/core/entities"

// ProviderCatalog is an immutable, ordered view over the configured providers.
type ProviderCatalog struct {
	order     []string
	providers map[string]*entities.Provider
}

func NewProviderCatalog(order []string, providers map[string]*entities.Provider) *ProviderCatalog {
	names := make([]string, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, name := range order {
		if _, ok := providers[name]; ok && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for name := range providers {
		if !seen[name] {
			names = append(names, name)
		}
	}
	return &ProviderCatalog{order: names, providers: providers}
}

func (c *ProviderCatalog) Get(name string) (*entities.Provider, bool) {
	p, ok := c.providers[name]
	return p, ok
}

// Names returns provider names in configuration order.
func (c *ProviderCatalog) Names() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}
