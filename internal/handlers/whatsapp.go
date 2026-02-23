package handlers

import (
	"bg-go/internal/lib/response"
	"bg-go/internal/lib/whatsapp"

	"github.com/gofiber/fiber/v2"
)

// WhatsAppHandler handles WhatsApp routes
type WhatsAppHandler struct{}

// NewWhatsAppHandler creates a new WhatsApp handler
func NewWhatsAppHandler() *WhatsAppHandler {
	return &WhatsAppHandler{}
}

// GetStatus returns WhatsApp connection status
func (h *WhatsAppHandler) GetStatus(c *fiber.Ctx) error {
	if whatsapp.WhatsApp == nil {
		return response.Success(c, 200, fiber.Map{
			"connected":     false,
			"logged_in":     false,
			"qr_code":       "",
			"qr_code_image": "",
			"message":       "WhatsApp not initialized",
		})
	}

	status := whatsapp.WhatsApp.GetStatus()
	return response.Success(c, 200, status)
}

// Connect starts WhatsApp connection
func (h *WhatsAppHandler) Connect(c *fiber.Ctx) error {
	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	err := whatsapp.WhatsApp.Connect()
	if err != nil {
		return response.Error(c, 500, "Failed to connect: "+err.Error())
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Connecting...",
	})
}

// Disconnect disconnects WhatsApp
func (h *WhatsAppHandler) Disconnect(c *fiber.Ctx) error {
	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	whatsapp.WhatsApp.Disconnect()
	return response.Success(c, 200, fiber.Map{
		"message": "Disconnected",
	})
}

// Logout logs out and clears session
func (h *WhatsAppHandler) Logout(c *fiber.Ctx) error {
	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	err := whatsapp.WhatsApp.Logout()
	if err != nil {
		return response.Error(c, 500, "Failed to logout: "+err.Error())
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Logged out successfully",
	})
}

// Restart restarts WhatsApp connection
func (h *WhatsAppHandler) Restart(c *fiber.Ctx) error {
	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	err := whatsapp.WhatsApp.Restart()
	if err != nil {
		return response.Error(c, 500, "Failed to restart: "+err.Error())
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Restarting...",
	})
}

// SendMessage sends a WhatsApp message
func (h *WhatsAppHandler) SendMessage(c *fiber.Ctx) error {
	type SendRequest struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}

	var req SendRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Phone == "" || req.Message == "" {
		return response.BadRequest(c, "Phone and message are required")
	}

	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	if !whatsapp.WhatsApp.IsLoggedIn() {
		return response.Error(c, 500, "WhatsApp not logged in")
	}

	err := whatsapp.WhatsApp.SendMessage(req.Phone, req.Message)
	if err != nil {
		return response.Error(c, 500, "Failed to send message: "+err.Error())
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Message sent successfully",
		"phone":   req.Phone,
	})
}

// SendTestMessage sends a test message (for testing)
func (h *WhatsAppHandler) SendTestMessage(c *fiber.Ctx) error {
	phone := c.Query("phone")
	if phone == "" {
		return response.BadRequest(c, "Phone parameter is required")
	}

	if whatsapp.WhatsApp == nil {
		return response.Error(c, 500, "WhatsApp not initialized")
	}

	if !whatsapp.WhatsApp.IsLoggedIn() {
		return response.Error(c, 500, "WhatsApp not logged in")
	}

	message := "Test message from LabaLaba Nusantara - " + `{{date}}`
	err := whatsapp.WhatsApp.SendMessage(phone, message)
	if err != nil {
		return response.Error(c, 500, "Failed to send test message: "+err.Error())
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Test message sent successfully",
		"phone":   phone,
	})
}
