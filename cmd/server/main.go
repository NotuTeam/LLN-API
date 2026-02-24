package main

import (
	"log"
	"os"

	"bg-go/internal/config"
	"bg-go/internal/database"
	"bg-go/internal/lib/cloudinary"
	"bg-go/internal/lib/whatsapp"
	"bg-go/internal/middleware"
	"bg-go/internal/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file (ignore error in production)
	godotenv.Load()
}

func main() {
	// Load configuration
	cfg := config.Load()

	log.Printf("Starting %s...", cfg.App.Name)
	log.Printf("Environment: %s", cfg.App.Env)
	log.Printf("Port: %s", cfg.App.Port)
	log.Printf("Database Driver: %s", cfg.Database.Driver)

	// Initialize CDN (optional)
	if err := cloudinary.Init(); err != nil {
		log.Printf("Warning: Failed to initialize CDN: %v", err)
	}

	// Connect to database (non-fatal for health check to work)
	if _, err := database.Connect(&cfg.Database); err != nil {
		log.Printf("ERROR: Failed to connect to database: %v", err)
		// Don't crash - let health check return error status
	} else {
		log.Printf("Database connected successfully")
	}

	// Initialize WhatsApp (optional)
	if err := whatsapp.Init(); err != nil {
		log.Printf("Warning: Failed to initialize WhatsApp: %v", err)
	} else {
		log.Println("WhatsApp initialized. Use /api/v1/whatsapp/connect to connect.")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ServerHeader: cfg.App.Name,
		BodyLimit:    int(cfg.Upload.MaxFileSize),
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// CORS middleware
	app.Use(middleware.CustomCORS())
	app.Use(middleware.SetupCORS())

	// Setup routes
	routes.SetupRoutes(app)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(fiber.Map{
			"status":  404,
			"message": "Endpoint not found",
		})
	})

	// Start server
	log.Printf("Server listening on port %s...", cfg.App.Port)

	// Use Render/Railway PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.App.Port
	}

	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
