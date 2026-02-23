package middleware

import (
	"strings"

	"bg-go/internal/lib/jwt"
	"bg-go/internal/lib/response"

	"github.com/gofiber/fiber/v2"
)

// AuthGuard protects routes requiring authentication
func AuthGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		
		if authHeader == "" {
			return response.Unauthorized(c, "No token provided")
		}
		
		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid token format")
		}
		
		tokenString := parts[1]
		
		claims, err := jwt.VerifyAccessToken(tokenString)
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired token")
		}
		
		// Store claims in locals for later use
		c.Locals("user", claims)
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)
		
		return c.Next()
	}
}

// RoleGuard protects routes by role
func RoleGuard(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := c.Locals("role")
		
		if role == nil {
			return response.Unauthorized(c, "No role found")
		}
		
		roleStr := role.(string)
		
		for _, allowedRole := range allowedRoles {
			if roleStr == allowedRole {
				return c.Next()
			}
		}
		
		return response.Error(c, 403, "Insufficient permissions")
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *fiber.Ctx) string {
	if userID := c.Locals("user_id"); userID != nil {
		return userID.(string)
	}
	return ""
}

// GetUserRole extracts user role from context
func GetUserRole(c *fiber.Ctx) string {
	if role := c.Locals("role"); role != nil {
		return role.(string)
	}
	return ""
}

// GetClaims extracts all claims from context
func GetClaims(c *fiber.Ctx) *jwt.Claims {
	if user := c.Locals("user"); user != nil {
		return user.(*jwt.Claims)
	}
	return nil
}
