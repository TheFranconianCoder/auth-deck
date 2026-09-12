package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/TheFranconianCoder/auth-deck/internal/api"
	"github.com/TheFranconianCoder/auth-deck/internal/core/usecases"
	"github.com/TheFranconianCoder/auth-deck/internal/infrastructure/browser"
	"github.com/TheFranconianCoder/auth-deck/internal/infrastructure/config"
	"github.com/TheFranconianCoder/auth-deck/internal/infrastructure/oauth"
	infraproxy "github.com/TheFranconianCoder/auth-deck/internal/infrastructure/proxy"
	filestore "github.com/TheFranconianCoder/auth-deck/internal/infrastructure/store"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
	"github.com/TheFranconianCoder/auth-deck/internal/tui"
)

func main() {
	configPath := flag.String("config", "", "path to config (default: OS config directory)")
	flag.Parse()

	path := *configPath
	if path == "" {
		path = config.DefaultConfigPath()
	}

	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	queue := state.NewRequestQueue()
	authStates := state.NewAuthStates()

	tokenStore, err := filestore.NewFileStore(cfg.TokenStorePath)
	if err != nil {
		log.Fatalf("token store error: %v", err)
	}

	var program *tea.Program
	notifier := usecases.NotifierFunc(func(event any) {
		if program != nil {
			program.Send(event)
		}
	})

	tokens := usecases.NewTokenService(
		cfg.Catalog,
		tokenStore,
		oauth.NewClient(),
		usecases.BrowserFunc(browser.Open),
		authStates,
		notifier,
	)
	selector := usecases.NewProviderSelector(queue, notifier)
	proxy := usecases.NewProxyService(cfg.Catalog, tokens, infraproxy.NewForwarder(), queue)

	router := api.NewRouter(selector, tokens, proxy, cfg.Catalog, queue)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetch := func(provider string) {
		go func() { _, _ = tokens.RefreshNow(ctx, provider) }()
	}

	program = tea.NewProgram(
		tui.NewModel(cfg.Catalog, queue, tokenStore.All(), fetch),
		tea.WithAltScreen(),
	)

	go tokens.AutoRefresh(ctx, 30*time.Second)

	server := &http.Server{Addr: cfg.Addr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP error: %v\n", err)
		}
	}()

	fmt.Printf("AuthDeck listening on %s\n", cfg.Addr)

	if err := program.Start(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	os.Exit(0)
}
