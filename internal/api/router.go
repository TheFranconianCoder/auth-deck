package api

import (
	"net/http"

	"github.com/TheFranconianCoder/auth-deck/internal/core/usecases"
)

type Router struct {
	mux      *http.ServeMux
	selector *usecases.ProviderSelector
	tokens   *usecases.TokenService
	proxy    *usecases.ProxyService
	catalog  usecases.ProviderCatalog
	queue    usecases.RequestQueue
}

func NewRouter(
	selector *usecases.ProviderSelector,
	tokens *usecases.TokenService,
	proxy *usecases.ProxyService,
	catalog usecases.ProviderCatalog,
	queue usecases.RequestQueue,
) *Router {
	r := &Router{
		mux:      http.NewServeMux(),
		selector: selector,
		tokens:   tokens,
		proxy:    proxy,
		catalog:  catalog,
		queue:    queue,
	}
	r.routes()
	return r
}

func (r *Router) routes() {
	r.mux.HandleFunc("/token", r.handleToken)
	r.mux.HandleFunc("/token/", r.handleTokenDirect)
	r.mux.HandleFunc("/callback", r.handleCallback)
	r.mux.HandleFunc("/proxy/", r.handleProxy)
	r.mux.HandleFunc("/direct/", r.handleDirectProxy)
	r.mux.HandleFunc("/health", r.handleHealth)
	r.mux.HandleFunc("/", r.handleIndex)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
