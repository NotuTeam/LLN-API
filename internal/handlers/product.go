package handlers

import (
	"context"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/file"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProductHandler handles product routes
type ProductHandler struct{}

// NewProductHandler creates a new product handler
func NewProductHandler() *ProductHandler {
	return &ProductHandler{}
}

// List returns all products with pagination
func (h *ProductHandler) List(c *fiber.Ctx) error {
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
				{"description": bson.M{"$regex": search, "$options": "i"}},
			},
		}
	}
	if activeOnly {
		filter["is_active"] = true
	}

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch products")
	}
	defer cursor.Close(ctx)

	var products []models.Product
	cursor.All(ctx, &products)

	return response.SuccessWithPagination(c, 200, products, response.CalculatePagination(int64(page), int64(limit), total))
}

// Detail returns a single product by ID
func (h *ProductHandler) Detail(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	product := &models.Product{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(product)
	if err != nil {
		return response.NotFound(c, "Product not found")
	}

	return response.Success(c, 200, product)
}

// Create creates a new product
func (h *ProductHandler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name        string  `json:"name"`
		Description string  `json:"description,omitempty"`
		Price       float64 `json:"price"`
		Unit        string  `json:"unit"`
		Stock       int     `json:"stock"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Name == "" || req.Price <= 0 {
		return response.BadRequest(c, "Name and price are required")
	}

	product := models.NewProduct()
	product.Name = req.Name
	product.Description = req.Description
	product.Price = req.Price
	product.Unit = req.Unit
	product.Stock = req.Stock

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, product)
	if err != nil {
		return response.Error(c, 500, "Failed to create product")
	}

	return response.Success(c, 201, product)
}

// Update updates a product
func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	type UpdateRequest struct {
		Name        string  `json:"name,omitempty"`
		Description string  `json:"description,omitempty"`
		Price       float64 `json:"price,omitempty"`
		Unit        string  `json:"unit,omitempty"`
		Stock       *int    `json:"stock,omitempty"`
		IsActive    *bool   `json:"is_active,omitempty"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	update := bson.M{"updated_at": time.Now()}
	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Description != "" {
		update["description"] = req.Description
	}
	if req.Price > 0 {
		update["price"] = req.Price
	}
	if req.Unit != "" {
		update["unit"] = req.Unit
	}
	if req.Stock != nil {
		update["stock"] = *req.Stock
	}
	if req.IsActive != nil {
		update["is_active"] = *req.IsActive
	}

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update product")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Product not found")
	}

	return response.SuccessWithMessage(c, 200, "Successfully updated")
}

// Delete deletes a product
func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return response.Error(c, 500, "Failed to delete product")
	}
	if result.DeletedCount == 0 {
		return response.NotFound(c, "Product not found")
	}

	return response.SuccessWithMessage(c, 200, "Successfully deleted")
}

// UploadImage uploads product image
func (h *ProductHandler) UploadImage(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	formFile, err := c.FormFile("image")
	if err != nil {
		return response.BadRequest(c, "No image provided")
	}

	uploadResult, err := file.UploadFile(formFile)
	if err != nil {
		return response.Error(c, 500, "Failed to upload image")
	}

	collection := database.GetMongoCollection("products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	image := &models.Image{
		PublicID: uploadResult.PublicID,
		URL:      uploadResult.URL,
	}

	update := bson.M{
		"updated_at": time.Now(),
		"image":      image,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update product image")
	}

	return response.Success(c, 200, image)
}
