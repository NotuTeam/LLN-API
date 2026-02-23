package main

import (
	"log"

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
	// Load .env file
	godotenv.Load()
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Log environment
	if cfg.IsDevelopment() {
		log.Println("Development mode - Hot reload enabled (use 'air')")
	}

	// Initialize CDN
	if err := cloudinary.Init(); err != nil {
		log.Printf("Warning: Failed to initialize CDN: %v", err)
	}

	// Connect to database
	if _, err := database.Connect(&cfg.Database); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize WhatsApp (optional - won't fail if error)
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
	log.Printf("Server running on port %s...", cfg.App.Port)
	log.Printf("Database: %s", cfg.Database.Driver)

	if err := app.Listen(":" + cfg.App.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
