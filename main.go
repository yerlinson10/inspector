package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"inspector/internal/config"
	"inspector/internal/handlers"
	"inspector/internal/middleware"
	"inspector/internal/storage"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load config
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init database
	if err := storage.Init(cfg.Database.Path); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	authHandler := handlers.NewAuthHandler(cfg.Auth.Username, cfg.Auth.Password)

	r.HTMLRender = buildRenderer()

	// Public routes (no auth) - Receiver endpoints
	public := r.Group("/in")
	{
		public.Any("/:slug", handlers.ReceiveRequest)
		public.GET("/:slug/ws", handlers.ReceiveWebSocket)
	}
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authHandler.HandleLogin)
	r.GET("/logout", authHandler.Logout)

	// Authenticated routes
	auth := r.Group("/", middleware.SessionAuth(authHandler.SessionValue))
	{
		auth.GET("/", func(c *gin.Context) {
			c.Redirect(302, "/dashboard")
		})
		auth.GET("/dashboard", handlers.Dashboard)

		// Requests
		auth.GET("/requests", handlers.ListRequests)
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

		// SSE
		auth.GET("/events", handlers.SSEStream)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Inspector running on http://%s", addr)
	log.Printf("Auth: %s / %s", cfg.Auth.Username, cfg.Auth.Password)
	log.Printf("Public endpoints: http://%s/in/<slug>", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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
