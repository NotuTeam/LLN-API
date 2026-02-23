package middleware

import (
	"bg-go/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// SetupCORS configures CORS middleware
func SetupCORS() fiber.Handler {
	cfg := config.Cfg
	
	return cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     cfg.CORS.AllowedMethods,
		AllowHeaders:     cfg.CORS.AllowedHeaders,
		AllowCredentials: true,
		MaxAge:           86400,
	})
}

// CustomCORS adds custom CORS headers (for preflight handling)
func CustomCORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		
		cfg := config.Cfg
		
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Methods", cfg.CORS.AllowedMethods)
		c.Set("Access-Control-Allow-Headers", cfg.CORS.AllowedHeaders)
		c.Set("Access-Control-Allow-Credentials", "true")
		
		if c.Method() == "OPTIONS" {
			c.Set("Access-Control-Max-Age", "86400")
			return c.SendStatus(fiber.StatusNoContent)
		}
		
		return c.Next()
	}
}
