package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"inspector/internal/config"
	"inspector/internal/handlers"
	"inspector/internal/middleware"
	"inspector/internal/storage"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) > 1 && strings.EqualFold(strings.TrimSpace(os.Args[1]), "healthcheck") {
		os.Exit(runHealthcheckCLI(os.Args[2:]))
	}

	// Load config
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if isDefaultCredentials(cfg) {
		if isProductionEnvironment() && os.Getenv("INSPECTOR_ALLOW_DEFAULT_AUTH") != "1" {
			log.Fatalf("Refusing to start with default credentials in production. Update auth.username and auth.password in config.")
		}
		log.Printf("WARNING: running with default credentials. Change auth.username/auth.password before exposing publicly.")
	}

	// Init database
	if err := storage.Init(cfg.Database.Path); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	handlers.ConfigureRuntime(handlers.RuntimeConfig{
		MaxRequests:          cfg.Settings.MaxRequests,
		MaxRequestBodyBytes:  cfg.Settings.MaxRequestBodyBytes,
		MaxResponseBodyBytes: cfg.Settings.MaxResponseBodyBytes,
		AllowedWSOrigins:     cfg.Settings.AllowedWSOrigins,
		RedactionEnabled:     cfg.Settings.RedactionEnabled,
		RedactionHeaders:     cfg.Settings.RedactionHeaders,
		RedactionFields:      cfg.Settings.RedactionFields,
		AlertWebhookURL:      cfg.Settings.AlertWebhookURL,
		AlertMinSentStatus:   cfg.Settings.AlertMinSentStatus,
		AlertOnSentError:     cfg.Settings.AlertOnSentError,
	})

	cleanupInterval := time.Duration(cfg.Settings.CleanupIntervalSeconds) * time.Second
	if cleanupInterval <= 0 {
		cleanupInterval = 30 * time.Second
	}
	storage.Cleanup(cfg.Settings.MaxRequests)
	stopCleanup := storage.StartCleanupWorker(cfg.Settings.MaxRequests, cleanupInterval)
	defer stopCleanup()

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(middleware.SecurityHeaders())

	sessionTTL := time.Duration(cfg.Settings.SessionTTLHours) * time.Hour
	authHandler := handlers.NewAuthHandler(cfg.Auth.Username, cfg.Auth.Password, sessionTTL)

	r.HTMLRender = buildRenderer()

	// Public routes (no auth) - Receiver endpoints
	public := r.Group("/in")
	{
		public.Any("/:slug", handlers.ReceiveRequest)
		public.GET("/:slug/ws", handlers.ReceiveWebSocket)
	}
	r.GET("/healthz", handlers.Healthz)
	r.GET("/readyz", handlers.Readyz)
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authHandler.HandleLogin)
	r.GET("/logout", authHandler.Logout)

	// Authenticated routes
	auth := r.Group("/", middleware.SessionAuth(authHandler.ValidateSession), middleware.CSRFProtection())
	{
		auth.GET("/", func(c *gin.Context) {
			c.Redirect(302, "/dashboard")
		})
		auth.GET("/dashboard", handlers.Dashboard)

		// Requests
		auth.GET("/requests", handlers.ListRequests)
		auth.GET("/requests/diff", handlers.RequestDiff)
		auth.GET("/requests/:id", handlers.RequestDetail)

		// Endpoints CRUD
		auth.GET("/endpoints", handlers.ListEndpoints)
		auth.POST("/endpoints", handlers.CreateEndpoint)
		auth.PUT("/endpoints/:id", handlers.UpdateEndpoint)
		auth.POST("/endpoints/:id", handlers.UpdateEndpoint) // HTML form fallback
		auth.DELETE("/endpoints/:id", handlers.DeleteEndpoint)
		auth.POST("/endpoints/:id/clear", handlers.ClearEndpointLogs)

		// Sender
		auth.GET("/send", handlers.SenderPage)
		auth.POST("/send/http", handlers.SendHTTP)
		auth.GET("/send/history", handlers.SentHistory)
		auth.GET("/send/history/:id", handlers.SentDetail)
		auth.GET("/send/ws-client", handlers.WSClientPage)
		auth.GET("/send/ws-proxy", handlers.WSProxy)
		auth.GET("/docs", handlers.DocsPage)

		// SSE + WS live updates (WS preferred behind tunnels/proxies)
		auth.GET("/events", handlers.SSEStream)
		auth.GET("/events/ws", handlers.EventsWebSocket)
		auth.GET("/events/poll", handlers.EventsPoll)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Inspector running on http://%s", addr)
	log.Printf("Auth user: %s", cfg.Auth.Username)
	log.Printf("Public endpoints: http://%s/in/<slug>", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("Shutting down server due to signal: %s", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}
}

func runHealthcheckCLI(args []string) int {
	target := "http://127.0.0.1:9090/readyz"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		target = strings.TrimSpace(args[0])
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		log.Printf("healthcheck failed: %v", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("healthcheck returned status %d", resp.StatusCode)
		return 1
	}

	return 0
}

func isDefaultCredentials(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.Auth.Username) == "admin" && strings.TrimSpace(cfg.Auth.Password) == "inspector123"
}

func isProductionEnvironment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("INSPECTOR_ENV")), "production")
}

func buildRenderer() multitemplate.Renderer {
	r := multitemplate.NewRenderer()
	funcMap := template.FuncMap{
		"add":      func(a, b int) int { return a + b },
		"subtract": func(a, b int) int { return a - b },
	}
	pages := []string{
		"dashboard.html",
		"endpoints.html",
		"requests.html",
		"request_diff.html",
		"request_detail.html",
		"sender.html",
		"ws_client.html",
		"sent_history.html",
		"sent_detail.html",
		"docs.html",
	}
	layoutFile := filepath.Join("web", "templates", "layout.html")
	for _, page := range pages {
		pageFile := filepath.Join("web", "templates", page)
		tmpl := template.Must(template.New("layout.html").Funcs(funcMap).ParseFiles(layoutFile, pageFile))
		r.Add(page, tmpl)
	}
	r.AddFromFiles("login.html", filepath.Join("web", "templates", "login.html"))
	return r
}
