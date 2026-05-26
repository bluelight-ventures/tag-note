package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"tagnote/internal/handler"
	"tagnote/internal/repo"
	"tagnote/internal/service"
	"tagnote/web"
)

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
	authSvc := service.NewAuth(r, emailSvc)
	h := handler.New(svc)
	ah := handler.NewAuth(authSvc)
	ih := handler.NewImage(*uploadDir)

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

	h.Register(app, ah, ih, authSvc)

	// Serve uploaded images
	app.Static("/uploads", *uploadDir, fiber.Static{
		Browse: false,
	})

	// Landing page at exact "/"
	app.Get("/", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("landing.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("landing page not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	// Serve embedded static assets (CSS, JS, images, etc.)
	webRoot, err := fs.Sub(web.Assets, ".")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}
	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(webRoot),
	}))

	// App SPA catch-all: /app and /app/* serve index.html
	app.Use("/app", func(c *fiber.Ctx) error {
		file, err := web.Assets.ReadFile("index.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("app not found")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(file)
	})

	fmt.Printf("TagNote server listening on %s\n", *addr)
	log.Fatal(app.Listen(*addr))
}
