package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"github.com/runminglu/tag-note/internal/admin"
	"github.com/runminglu/tag-note/internal/handler"
	"github.com/runminglu/tag-note/internal/middleware"
	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
	"github.com/runminglu/tag-note/web"
)

// Build-time variables set via -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var startTime = time.Now()

func main() {
	addr := flag.String("addr", ":3000", "listen address")
	dbPath := flag.String("db", "data/tagnote.db", "path to SQLite database")
	uploadDir := flag.String("uploads", "data/uploads", "path to image uploads directory")
	flag.Parse()

	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	r, err := repo.NewSQLiteRepo(*dbPath)
	if err != nil {
		log.Fatalf("init repo: %v", err)
	}

	emailSvc := service.NewEmailService()
	svc := service.New(r)
	authSvc, err := service.NewAuth(r, emailSvc, *uploadDir)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	h := handler.New(svc)
	ah := handler.NewAuth(authSvc)
	ih := handler.NewImage(*uploadDir, r)

	// Load admin config
	adminCfg := admin.LoadConfig()
	adminHandler := admin.NewHandler(r, authSvc, adminCfg, r.DB())

	// Create test user if TAGNOTE_TEST_MODE=1
	if os.Getenv("TAGNOTE_TEST_MODE") == "1" {
		if err := authSvc.EnsureTestUser(context.Background()); err != nil {
			log.Printf("warning: could not create test user: %v", err)
		} else {
			log.Println("test user ensured (test@test.com / testpass123)")
		}
	}

	app := fiber.New(fiber.Config{
		AppName:   "TagNote",
		BodyLimit: 10 * 1024 * 1024, // 10MB to allow overhead beyond 5MB image uploads
	})
	app.Use(logger.New())
	app.Use(cors.New())

	// Metrics middleware — must be registered early to capture all requests
	app.Use(admin.MetricsMiddleware())

	operationalAccess := admin.OperationalAccess(adminCfg, authSvc)

	// Metrics endpoint (operational bearer token or admin JWT)
	app.Get("/metrics", operationalAccess, admin.ExposeMetrics)

	app.Get("/healthz", func(c *fiber.Ctx) error {
		dbOK := true
		if err := r.Ping(c.Context()); err != nil {
			dbOK = false
		}

		status := "ok"
		if !dbOK {
			status = "degraded"
		}

		return c.JSON(fiber.Map{
			"status": status,
		})
	})

	app.Get("/status", operationalAccess, func(c *fiber.Ctx) error {
		db := r.DB()

		var userCount, noteCount, tagCount, trashCount int
		db.QueryRowContext(c.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
		db.QueryRowContext(c.Context(), "SELECT COUNT(*) FROM subnotes WHERE deleted_at IS NULL").Scan(&noteCount)
		db.QueryRowContext(c.Context(), "SELECT COUNT(*) FROM tags").Scan(&tagCount)
		db.QueryRowContext(c.Context(), "SELECT COUNT(*) FROM subnotes WHERE deleted_at IS NOT NULL").Scan(&trashCount)

		var pageCount, pageSize int
		db.QueryRowContext(c.Context(), "PRAGMA page_count").Scan(&pageCount)
		db.QueryRowContext(c.Context(), "PRAGMA page_size").Scan(&pageSize)
		dbSizeBytes := pageCount * pageSize

		return c.JSON(fiber.Map{
			"version":       Version,
			"build_time":    BuildTime,
			"git_commit":    GitCommit,
			"uptime":        time.Since(startTime).Truncate(time.Second).String(),
			"uptime_sec":    int(time.Since(startTime).Seconds()),
			"users":         userCount,
			"notes":         noteCount,
			"tags":          tagCount,
			"trash":         trashCount,
			"db_size_bytes": dbSizeBytes,
			"db_size_mb":    fmt.Sprintf("%.2f", float64(dbSizeBytes)/(1024*1024)),
		})
	})

	h.Register(app, ah, ih, authSvc, admin.AuditLog(r))

	// Admin API routes (JWT + AdminOnly protected)
	adminAPI := app.Group("/api/v1/admin", middleware.JWTAuth(authSvc), admin.AdminOnly(adminCfg, authSvc))
	adminAPI.Get("/overview", adminHandler.Overview)
	adminAPI.Get("/users", adminHandler.Users)
	adminAPI.Get("/logs", adminHandler.Logs)

	// Admin dashboard page
	app.Get("/admin", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("admin.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("admin page not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	// Serve uploaded images
	uploadStatic := fiber.Static{Browse: false}
	if os.Getenv("TAGNOTE_TEST_MODE") == "1" {
		// fasthttp keeps an open-file-handle cache (default 10s) that keeps
		// serving a file even after it is unlinked on account deletion.
		// Production keeps that cache for performance; the test environment
		// shortens it so e2e assertions can observe the deletion promptly.
		uploadStatic.CacheDuration = time.Second
	}
	app.Static("/uploads", *uploadDir, uploadStatic)

	// Landing page at exact "/"
	app.Get("/", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("landing.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("landing page not found")
		}
		html := strings.ReplaceAll(string(file), "{{BASE_URL}}", publicBaseURL())
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(html)
	})

	app.Get("/robots.txt", func(c *fiber.Ctx) error {
		baseURL := publicBaseURL()
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString("User-agent: *\n" +
			"Allow: /\n" +
			"Allow: /privacy\n" +
			"Allow: /terms\n" +
			"Allow: /support\n\n" +
			"Disallow: /app\n" +
			"Disallow: /app/\n" +
			"Disallow: /api/\n" +
			"Disallow: /healthz\n" +
			"Disallow: /status\n" +
			"Disallow: /metrics\n" +
			"Disallow: /uploads/\n\n" +
			"Sitemap: " + baseURL + "/sitemap.xml\n")
	})

	app.Get("/sitemap.xml", func(c *fiber.Ctx) error {
		baseURL := publicBaseURL()
		c.Set("Content-Type", "application/xml; charset=utf-8")
		return c.SendString(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>` + baseURL + `/</loc>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>` + baseURL + `/privacy</loc>
    <changefreq>monthly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + baseURL + `/terms</loc>
    <changefreq>monthly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + baseURL + `/support</loc>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>`)
	})

	// Privacy policy
	app.Get("/privacy", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("privacy.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("page not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	// Terms of service
	app.Get("/terms", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("terms.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("page not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	// Support / help (also the App Store Connect Support URL)
	app.Get("/support", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("support.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("page not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	// App SPA: serve index.html for /app and /app/* (not /app.js or /app.css)
	app.Use("/app", func(c *fiber.Ctx) error {
		path := c.Path()
		// Only serve SPA for /app or /app/ or /app/... (not /app.js, /app.css, etc.)
		if path != "/app" && !strings.HasPrefix(path, "/app/") {
			return c.Next()
		}
		file, err := web.Assets.ReadFile("index.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("app not found")
		}

		// Inject Google Client ID into the HTML if configured.
		googleClientID := webGoogleClientID(os.Getenv("GOOGLE_CLIENT_ID"))
		html := string(file)
		if googleClientID != "" {
			// Inject script tag before </head> to set GOOGLE_CLIENT_ID
			configScript := fmt.Sprintf(`<script>window.GOOGLE_CLIENT_ID="%s";</script>`, googleClientID)
			// Also add Google Identity Services library
			gsiScript := `<script src="https://accounts.google.com/gsi/client" async defer></script>`
			html = strings.Replace(html, "</head>", configScript+gsiScript+"</head>", 1)
		}

		// Inject Sign in with Apple (web) config if a Services ID is configured.
		appleClientID := strings.TrimSpace(os.Getenv("APPLE_WEB_CLIENT_ID"))
		if appleClientID != "" {
			appleRedirect := strings.TrimSpace(os.Getenv("APPLE_WEB_REDIRECT_URI"))
			if appleRedirect == "" {
				appleRedirect = publicBaseURL()
			}
			appleConfig := fmt.Sprintf(`<script>window.APPLE_CLIENT_ID="%s";window.APPLE_REDIRECT_URI="%s";</script>`, appleClientID, appleRedirect)
			appleSDK := `<script src="https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js" async defer></script>`
			html = strings.Replace(html, "</head>", appleConfig+appleSDK+"</head>", 1)
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(html)
	})

	// Serve embedded static assets (CSS, JS, images, etc.)
	webRoot, err := fs.Sub(web.Assets, ".")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}
	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(webRoot),
	}))

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", *addr, err)
	}

	printStartupLinks(*addr)

	// Start server in a goroutine
	go func() {
		if err := app.Listener(listener); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	if err := r.Close(); err != nil {
		log.Printf("db close error: %v", err)
	}

	fmt.Println("Server stopped")
}

// webGoogleClientID returns the client ID used to initialize Google Sign-In in
// the browser. GOOGLE_CLIENT_ID may be a comma-separated list of accepted token
// audiences (web, iOS, ...) for backend verification; the browser must use a
// single, valid client ID — the web client, which is the first entry. Injecting
// the whole list makes Google reject it with "invalid_client".
func webGoogleClientID(raw string) string {
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(raw)
}

func publicBaseURL() string {
	if baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/"); baseURL != "" {
		return baseURL
	}
	if domain := strings.TrimSpace(os.Getenv("TAGNOTE_DOMAIN")); domain != "" {
		return "https://" + strings.TrimRight(domain, "/")
	}
	return "http://localhost:3000"
}

func printStartupLinks(addr string) {
	baseURL := devURL("TAGNOTE_DEV_BASE_URL", addr)
	proxyURL := strings.TrimRight(os.Getenv("TAGNOTE_DEV_PROXY_URL"), "/")
	grafanaURL := strings.TrimSpace(os.Getenv("TAGNOTE_GRAFANA_URL"))
	operationalToken := strings.TrimSpace(os.Getenv("OPERATIONAL_BEARER_TOKEN"))

	if grafanaURL == "" && proxyURL != "" {
		grafanaURL = proxyURL + "/grafana/"
	}
	if operationalToken == "" {
		operationalToken = "(set OPERATIONAL_BEARER_TOKEN)"
	}

	fmt.Printf("TagNote %s server ready on %s\n", Version, addr)
	fmt.Println()
	fmt.Println("Useful links")
	fmt.Printf("  Landing: %s/\n", baseURL)
	fmt.Printf("  App:     %s/app\n", baseURL)
	fmt.Printf("  Admin:   %s/admin\n", baseURL)
	fmt.Printf("  Health:  %s/healthz\n", baseURL)
	fmt.Printf("  Status:  %s/status\n", baseURL)
	fmt.Printf("  Metrics: %s/metrics\n", baseURL)
	if proxyURL != "" {
		fmt.Printf("  Proxy:   %s/\n", proxyURL)
	}
	if grafanaURL != "" {
		fmt.Printf("  Grafana: %s\n", grafanaURL)
	}
	fmt.Println()
	fmt.Printf("Operational endpoints require: Authorization: Bearer %s\n", operationalToken)
	fmt.Println()
}

func devURL(envKey, addr string) string {
	if value := strings.TrimRight(os.Getenv(envKey), "/"); value != "" {
		return value
	}
	if baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/"); baseURL != "" {
		return baseURL
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return publicBaseURL()
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
