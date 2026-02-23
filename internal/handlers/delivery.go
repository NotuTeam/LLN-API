package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"bg-go/internal/config"
	"bg-go/internal/database"
	"bg-go/internal/lib/notification"
	"bg-go/internal/lib/response"
	"bg-go/internal/middleware"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DeliveryHandler handles delivery routes
type DeliveryHandler struct{}

// NewDeliveryHandler creates a new delivery handler
func NewDeliveryHandler() *DeliveryHandler {
	return &DeliveryHandler{}
}

// generateDeliveryToken generates a unique delivery token
func generateDeliveryToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateNoteNumber generates delivery note number
func generateNoteNumber() string {
	return fmt.Sprintf("SJ-%s", time.Now().Format("20060102150405"))
}

// List returns all delivery notes
func (h *DeliveryHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	skip := (page - 1) * limit

	collection := database.GetMongoCollection("delivery_notes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, bson.M{})

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch delivery notes")
	}
	defer cursor.Close(ctx)

	var notes []models.DeliveryNote
	cursor.All(ctx, &notes)

	return response.SuccessWithPagination(c, 200, notes, response.CalculatePagination(int64(page), int64(limit), total))
}

// ListReady returns orders ready for delivery note creation (loading finished)
func (h *DeliveryHandler) ListReady(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	skip := (page - 1) * limit

	// Filter for orders that:
	// 1. Have status "loading"
	// 2. Have loading_finished_at set (not nil)
	// 3. Don't have delivery_note_id yet
	filter := bson.M{
		"status":              models.OrderStatusLoading,
		"loading_finished_at": bson.M{"$exists": true, "$ne": nil},
		"delivery_note_id":    bson.M{"$in": []interface{}{"", nil}},
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "loading_finished_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch orders")
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

// Detail returns a single delivery note by ID
func (h *DeliveryHandler) Detail(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("delivery_notes")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	note := &models.DeliveryNote{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(note)
	if err != nil {
		return response.NotFound(c, "Delivery note not found")
	}

	return response.Success(c, 200, note)
}

// Create creates a new delivery note from an order
func (h *DeliveryHandler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		OrderID string `json:"order_id"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.OrderID == "" {
		return response.BadRequest(c, "Order ID is required")
	}

	userID := middleware.GetUserID(c)

	orderObjID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID format")
	}

	orderCollection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get order
	order := &models.Order{}
	err = orderCollection.FindOne(ctx, bson.M{"_id": orderObjID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Check order status
	if order.Status != models.OrderStatusLoading {
		return response.BadRequest(c, "Order is not in loading status")
	}

	// Get sales data
	salesCollection := database.GetMongoCollection("sales")
	sales := &models.Sales{}
	if order.SalesID != "" {
		salesObjID, _ := primitive.ObjectIDFromHex(order.SalesID)
		salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
	}

	// Get product data
	productCollection := database.GetMongoCollection("products")
	product := &models.Product{}
	if order.ProductID != "" {
		productObjID, _ := primitive.ObjectIDFromHex(order.ProductID)
		productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(product)
	}

	// Create delivery note
	token := generateDeliveryToken()
	note := models.NewDeliveryNote()
	note.OrderID = req.OrderID
	note.NoteNumber = generateNoteNumber()
	note.Token = token
	note.CreatedBy = userID

	// Snapshot data
	note.SalesName = sales.Name
	note.SalesPhone = sales.Phone
	note.ProductName = product.Name
	note.ProductQty = order.Quantity
	note.ProductUnit = product.Unit
	note.DriverName = order.DriverName
	note.DriverPhone = order.DriverPhone
	note.VehiclePlate = order.VehiclePlate

	// Save delivery note
	deliveryCollection := database.GetMongoCollection("delivery_notes")
	_, err = deliveryCollection.InsertOne(ctx, note)
	if err != nil {
		return response.Error(c, 500, "Failed to create delivery note")
	}

	// Update order
	now := time.Now()
	orderUpdate := bson.M{
		"delivery_note_id":  note.ID.Hex(),
		"delivery_note_url": fmt.Sprintf("%s/delivery/%s", config.Cfg.Client.URL, token),
		"status":            models.OrderStatusCompleted,
		"completed_at":      now,
		"updated_at":        now,
	}

	_, err = orderCollection.UpdateOne(ctx, bson.M{"_id": orderObjID}, bson.M{"$set": orderUpdate})
	if err != nil {
		return response.Error(c, 500, "Failed to update order")
	}

	// Generate WhatsApp notification link
	notification.Init(config.Cfg.Client.URL)
	waLink, _ := notification.SendDeliveryNotification(
		sales.Phone,
		sales.Name,
		note.NoteNumber,
		product.Name,
		order.Quantity,
		product.Unit,
		order.DriverName,
		order.VehiclePlate,
		token,
	)

	// Check WhatsApp status for frontend
	waStatus := notification.WhatsAppStatus()

	// Populate order for response
	note.Order = order

	return response.Success(c, 201, fiber.Map{
		"delivery_note":  note,
		"whatsapp_link":  waLink,
		"whatsapp_connected": waStatus["logged_in"].(bool),
		"whatsapp_auto_sent": waStatus["logged_in"].(bool),
	})
}

// GetByToken returns delivery note by token (for client)
func (h *DeliveryHandler) GetByToken(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	collection := database.GetMongoCollection("delivery_notes")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	note := &models.DeliveryNote{}
	err := collection.FindOne(ctx, bson.M{"token": token}).Decode(note)
	if err != nil {
		return response.NotFound(c, "Delivery note not found")
	}

	return response.Success(c, 200, note)
}

// GetByOrder returns delivery note by order ID
func (h *DeliveryHandler) GetByOrder(c *fiber.Ctx) error {
	orderID := c.Params("order_id")

	orderObjID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID format")
	}

	orderCollection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := &models.Order{}
	err = orderCollection.FindOne(ctx, bson.M{"_id": orderObjID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	if order.DeliveryNoteID == "" {
		return response.NotFound(c, "No delivery note for this order")
	}

	deliveryCollection := database.GetMongoCollection("delivery_notes")
	note := &models.DeliveryNote{}
	noteObjID, _ := primitive.ObjectIDFromHex(order.DeliveryNoteID)
	err = deliveryCollection.FindOne(ctx, bson.M{"_id": noteObjID}).Decode(note)
	if err != nil {
		return response.NotFound(c, "Delivery note not found")
	}

	return response.Success(c, 200, note)
}
