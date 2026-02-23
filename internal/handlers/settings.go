package handlers

import (
	"context"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

// SettingsHandler handles settings routes
type SettingsHandler struct{}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// Get returns company settings
func (h *SettingsHandler) Get(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("company_settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settings := &models.CompanySettings{}
	err := collection.FindOne(ctx, bson.M{}).Decode(settings)
	if err != nil {
		// Return default empty settings if not found
		return response.Success(c, 200, &models.CompanySettings{
			Name: "LabaLaba Nusantara",
		})
	}

	return response.Success(c, 200, settings)
}

// Update updates company settings
func (h *SettingsHandler) Update(c *fiber.Ctx) error {
	type UpdateRequest struct {
		Name           string `json:"name"`
		Address        string `json:"address"`
		Phone          string `json:"phone"`
		Email          string `json:"email"`
		BankName       string `json:"bank_name"`
		BankAccount    string `json:"bank_account"`
		BankHolder     string `json:"bank_holder"`
		BankName2      string `json:"bank_name_2"`
		BankAccount2   string `json:"bank_account_2"`
		BankHolder2    string `json:"bank_holder_2"`
		WhatsAppNumber string `json:"whatsapp_number"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	collection := database.GetMongoCollection("company_settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if settings exists
	existing := &models.CompanySettings{}
	err := collection.FindOne(ctx, bson.M{}).Decode(existing)

	now := time.Now()

	if err != nil {
		// Create new settings
		settings := models.NewCompanySettings()
		settings.Name = req.Name
		settings.Address = req.Address
		settings.Phone = req.Phone
		settings.Email = req.Email
		settings.BankName = req.BankName
		settings.BankAccount = req.BankAccount
		settings.BankHolder = req.BankHolder
		settings.BankName2 = req.BankName2
		settings.BankAccount2 = req.BankAccount2
		settings.BankHolder2 = req.BankHolder2
		settings.WhatsAppNumber = req.WhatsAppNumber

		_, err = collection.InsertOne(ctx, settings)
		if err != nil {
			return response.Error(c, 500, "Failed to create settings")
		}

		return response.Success(c, 200, settings)
	}

	// Update existing settings
	update := bson.M{
		"name":            req.Name,
		"address":         req.Address,
		"phone":           req.Phone,
		"email":           req.Email,
		"bank_name":       req.BankName,
		"bank_account":    req.BankAccount,
		"bank_holder":     req.BankHolder,
		"bank_name_2":     req.BankName2,
		"bank_account_2":  req.BankAccount2,
		"bank_holder_2":   req.BankHolder2,
		"whatsapp_number": req.WhatsAppNumber,
		"updated_at":      now,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"_id": existing.ID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update settings")
	}

	// Get updated settings
	collection.FindOne(ctx, bson.M{"_id": existing.ID}).Decode(existing)

	return response.Success(c, 200, existing)
}

// GetPublic returns public company settings (for client)
func (h *SettingsHandler) GetPublic(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("company_settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settings := &models.CompanySettings{}
	err := collection.FindOne(ctx, bson.M{}).Decode(settings)
	if err != nil {
		return response.Success(c, 200, &models.CompanySettings{
			Name: "LabaLaba Nusantara",
		})
	}

	// Return only public info
	return response.Success(c, 200, fiber.Map{
		"name":           settings.Name,
		"bank_name":      settings.BankName,
		"bank_account":   settings.BankAccount,
		"bank_holder":    settings.BankHolder,
		"bank_name_2":    settings.BankName2,
		"bank_account_2": settings.BankAccount2,
		"bank_holder_2":  settings.BankHolder2,
		"whatsapp_number": settings.WhatsAppNumber,
	})
}
