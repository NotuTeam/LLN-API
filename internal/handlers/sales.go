package handlers

import (
	"context"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SalesHandler handles sales routes
type SalesHandler struct{}

// NewSalesHandler creates a new sales handler
func NewSalesHandler() *SalesHandler {
	return &SalesHandler{}
}

// List returns all sales with pagination
func (h *SalesHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	search := c.Query("search")
	activeOnly := c.QueryBool("active_only", false)

	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter = bson.M{
			"$or": []bson.M{
				{"name": bson.M{"$regex": search, "$options": "i"}},
				{"phone": bson.M{"$regex": search, "$options": "i"}},
				{"email": bson.M{"$regex": search, "$options": "i"}},
			},
		}
	}
	if activeOnly {
		filter["is_active"] = true
	}

	collection := database.GetMongoCollection("sales")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch sales")
	}
	defer cursor.Close(ctx)

	var sales []models.Sales
	cursor.All(ctx, &sales)

	return response.SuccessWithPagination(c, 200, sales, response.CalculatePagination(int64(page), int64(limit), total))
}

// Detail returns a single sales by ID
func (h *SalesHandler) Detail(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sales := &models.Sales{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(sales)
	if err != nil {
		return response.NotFound(c, "Sales not found")
	}

	return response.Success(c, 200, sales)
}

// Create creates a new sales
func (h *SalesHandler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Email   string `json:"email,omitempty"`
		Address string `json:"address,omitempty"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Name == "" || req.Phone == "" {
		return response.BadRequest(c, "Name and phone are required")
	}

	sales := models.NewSales()
	sales.Name = req.Name
	sales.Phone = req.Phone
	sales.Email = req.Email
	sales.Address = req.Address

	collection := database.GetMongoCollection("sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, sales)
	if err != nil {
		return response.Error(c, 500, "Failed to create sales")
	}

	return response.Success(c, 201, sales)
}

// Update updates a sales
func (h *SalesHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	type UpdateRequest struct {
		Name     string `json:"name,omitempty"`
		Phone    string `json:"phone,omitempty"`
		Email    string `json:"email,omitempty"`
		Address  string `json:"address,omitempty"`
		IsActive *bool  `json:"is_active,omitempty"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	update := bson.M{"updated_at": time.Now()}
	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Phone != "" {
		update["phone"] = req.Phone
	}
	if req.Email != "" {
		update["email"] = req.Email
	}
	if req.Address != "" {
		update["address"] = req.Address
	}
	if req.IsActive != nil {
		update["is_active"] = *req.IsActive
	}

	collection := database.GetMongoCollection("sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update sales")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Sales not found")
	}

	return response.SuccessWithMessage(c, 200, "Successfully updated")
}

// Delete deletes a sales (soft delete by setting is_active to false)
func (h *SalesHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return response.Error(c, 500, "Failed to delete sales")
	}
	if result.DeletedCount == 0 {
		return response.NotFound(c, "Sales not found")
	}

	return response.SuccessWithMessage(c, 200, "Successfully deleted")
}
