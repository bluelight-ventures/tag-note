package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/mcpoauth"
	"github.com/runminglu/tag-note/internal/mcpserver"
	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
)

// Build-time variables set via -ldflags.
var (
	Version = "dev"
)

func main() {
	log.SetOutput(os.Stderr)

	cfg := mcpserver.ConfigFromEnv()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.StringVar(&cfg.DBPath, "db", cfg.DBPath, "SQLite database path")
	flag.StringVar(&cfg.UploadsDir, "uploads", cfg.UploadsDir, "uploads directory")
	flag.StringVar(&cfg.PublicURL, "public-url", cfg.PublicURL, "public MCP origin, for example https://mcp.tag-note.com")
	flag.StringVar(&cfg.ResourcePath, "resource-path", cfg.ResourcePath, "MCP HTTP resource path")
	flag.BoolVar(&cfg.ReadOnly, "read-only", cfg.ReadOnly, "register only read tools")
	flag.BoolVar(&cfg.AllowDelete, "allow-delete", cfg.AllowDelete, "register soft-delete tools")
	flag.IntVar(&cfg.MaxNotes, "max-notes", cfg.MaxNotes, "maximum notes returned by one MCP call")
	flag.IntVar(&cfg.MaxContentBytes, "max-content-bytes", cfg.MaxContentBytes, "maximum note content bytes returned by one MCP call")
	flag.Parse()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("validate config: %v", err)
	}

	store, err := repo.NewSQLiteRepo(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	svc := service.New(store)
	emailSvc := service.NewEmailService()
	authSvc, err := service.NewAuth(store, emailSvc, cfg.UploadsDir)
	if err != nil {
		log.Fatalf("configure auth: %v", err)
	}
	if os.Getenv("TAGNOTE_TEST_MODE") == "1" {
		if err := authSvc.EnsureTestUser(context.Background()); err != nil {
			log.Fatalf("ensure test user: %v", err)
		}
	}

	mcpServer, err := mcpserver.New(cfg, svc, Version)
	if err != nil {
		log.Fatalf("configure MCP server: %v", err)
	}

	oauthServer, err := mcpoauth.NewServer(mcpoauth.Config{
		Issuer:              cfg.PublicURL,
		Resource:            cfg.ResourceURL(),
		ResourceMetadataURL: cfg.ResourceMetadataURL(),
	}, mcpoauth.NewStore(store.DB()), authSvc)
	if err != nil {
		log.Fatalf("configure MCP OAuth: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	oauthServer.RegisterRoutes(mux)
	streamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 30 * time.Minute,
	})
	mux.Handle(cfg.ResourcePath, oauthServer.BearerMiddleware(streamHandler))

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("tagnote-mcp listening on %s resource=%s", cfg.Addr, cfg.ResourceURL())
		errCh <- httpServer.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		log.Printf("shutting down after %s", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("run tagnote-mcp: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown tagnote-mcp: %v", err)
	}
}
