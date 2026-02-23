package response

import (
	"github.com/gofiber/fiber/v2"
)

// Pagination holds pagination info
type Pagination struct {
	CurrentPage int   `json:"current_page"`
	TotalPages  int   `json:"total_pages"`
	TotalItems  int64 `json:"total_items"`
	PerPage     int   `json:"per_page"`
	HasNext     bool  `json:"has_next"`
	HasPrev     bool  `json:"has_prev"`
}

// Standard response structures

// Success sends a success response with data wrapped
func Success(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"message": "success",
		"data":    data,
	})
}

// SuccessWithMessage sends a success response with custom message
func SuccessWithMessage(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"message": message,
	})
}

// SuccessWithData sends data at root level (for auth endpoints)
func SuccessWithData(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(data)
}

// SuccessWithPagination sends a paginated response
func SuccessWithPagination(c *fiber.Ctx, status int, data interface{}, pagination *Pagination) error {
	return c.Status(status).JSON(fiber.Map{
		"status":     status,
		"message":    "success",
		"data":       data,
		"pagination": pagination,
	})
}

// Error sends an error response
func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"message": message,
	})
}

// ErrorWithData sends an error response with additional data
func ErrorWithData(c *fiber.Ctx, status int, message string, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"message": message,
		"data":    data,
	})
}

// NotFound sends a 404 response
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return c.Status(200).JSON(fiber.Map{
		"status":  200,
		"message": message,
		"data":    nil,
	})
}

// Unauthorized sends a 401 response
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Unauthorized"
	}
	return c.Status(401).JSON(fiber.Map{
		"status":  401,
		"message": message,
	})
}

// BadRequest sends a 400 response
func BadRequest(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Bad request"
	}
	return c.Status(400).JSON(fiber.Map{
		"status":  400,
		"message": message,
	})
}

// InternalError sends a 500 response
func InternalError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Internal server error"
	}
	return c.Status(500).JSON(fiber.Map{
		"status":  500,
		"message": message,
	})
}

// CalculatePagination creates pagination info
func CalculatePagination(currentPage, perPage, totalItems int64) *Pagination {
	if perPage <= 0 {
		perPage = 10
	}
	
	totalPages := int(totalItems) / int(perPage)
	if int(totalItems)%int(perPage) > 0 {
		totalPages++
	}
	
	return &Pagination{
		CurrentPage: int(currentPage),
		TotalPages:  totalPages,
		TotalItems:  totalItems,
		PerPage:     int(perPage),
		HasNext:     currentPage < int64(totalPages),
		HasPrev:     currentPage > 1,
	}
}
