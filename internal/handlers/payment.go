package handlers

import (
	"context"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/file"
	"bg-go/internal/lib/response"
	"bg-go/internal/middleware"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PaymentHandler handles payment routes
type PaymentHandler struct{}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler() *PaymentHandler {
	return &PaymentHandler{}
}

// ListPending returns all orders with pending payments
func (h *PaymentHandler) ListPending(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	skip := (page - 1) * limit

	filter := bson.M{
		"payment_status": models.PaymentStatusPending,
		"payment_proof":  bson.M{"$ne": nil},
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "payment_uploaded_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch payments")
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	cursor.All(ctx, &orders)

	// Populate sales and product data
	salesCollection := database.GetMongoCollection("sales")
	productCollection := database.GetMongoCollection("products")

	for i := range orders {
		if orders[i].SalesID != "" {
			salesObjID, _ := primitive.ObjectIDFromHex(orders[i].SalesID)
			sales := &models.Sales{}
			salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
			orders[i].Sales = sales
		}
		if orders[i].ProductID != "" {
			productObjID, _ := primitive.ObjectIDFromHex(orders[i].ProductID)
			product := &models.Product{}
			productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(product)
			orders[i].Product = product
		}
	}

	return response.SuccessWithPagination(c, 200, orders, response.CalculatePagination(int64(page), int64(limit), total))
}

// Verify verifies a payment
func (h *PaymentHandler) Verify(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	userID := middleware.GetUserID(c)

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if order exists and has payment proof
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	if order.PaymentProof == nil {
		return response.BadRequest(c, "No payment proof uploaded")
	}

	if order.PaymentStatus != models.PaymentStatusPending {
		return response.BadRequest(c, "Payment is not in pending status")
	}

	now := time.Now()
	update := bson.M{
		"payment_status":      models.PaymentStatusVerified,
		"payment_verified_at": now,
		"payment_verified_by": userID,
		"status":              models.OrderStatusConfirmed,
		"updated_at":          now,
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to verify payment")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.SuccessWithMessage(c, 200, "Payment verified successfully")
}

// Reject rejects a payment
func (h *PaymentHandler) Reject(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	userID := middleware.GetUserID(c)

	type RejectRequest struct {
		Reason string `json:"reason"`
	}

	var req RejectRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if order exists
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	if order.PaymentStatus != models.PaymentStatusPending {
		return response.BadRequest(c, "Payment is not in pending status")
	}

	now := time.Now()
	update := bson.M{
		"payment_status":       models.PaymentStatusRejected,
		"payment_rejected_at":  now,
		"payment_rejected_by":  userID,
		"payment_reject_reason": req.Reason,
		"updated_at":           now,
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to reject payment")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.SuccessWithMessage(c, 200, "Payment rejected")
}

// UploadProof uploads payment proof (for client-side use with token)
func (h *PaymentHandler) UploadProof(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	formFile, err := c.FormFile("proof")
	if err != nil {
		return response.BadRequest(c, "No file provided")
	}

	uploadResult, err := file.UploadFile(formFile)
	if err != nil {
		return response.Error(c, 500, "Failed to upload file")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find order by invoice token
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	if order.Status != models.OrderStatusPending {
		return response.BadRequest(c, "Order is not in pending status")
	}

	now := time.Now()
	paymentProof := &models.Image{
		PublicID: uploadResult.PublicID,
		URL:      uploadResult.URL,
	}

	update := bson.M{
		"payment_proof":       paymentProof,
		"payment_uploaded_at": now,
		"status":              models.OrderStatusPaid,
		"updated_at":          now,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"invoice_token": token}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update order")
	}

	return response.Success(c, 200, fiber.Map{
		"message":       "Payment proof uploaded successfully",
		"payment_proof": paymentProof,
	})
}

// ListByStatus returns payments by status
func (h *PaymentHandler) ListByStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	if status != models.PaymentStatusPending && status != models.PaymentStatusVerified && status != models.PaymentStatusRejected {
		return response.BadRequest(c, "Invalid status")
	}

	skip := (page - 1) * limit

	filter := bson.M{"payment_status": status}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch payments")
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	cursor.All(ctx, &orders)

	return response.SuccessWithPagination(c, 200, orders, response.CalculatePagination(int64(page), int64(limit), total))
}
