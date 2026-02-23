package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueueHandler handles queue routes
type QueueHandler struct{}

// NewQueueHandler creates a new queue handler
func NewQueueHandler() *QueueHandler {
	return &QueueHandler{}
}

// generateQueueToken generates a unique queue token
func generateQueueToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// List returns all orders in queue
func (h *QueueHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	status := c.Query("status") // queued, loading, completed

	skip := (page - 1) * limit

	filter := bson.M{}
	if status == "queued" {
		filter["status"] = models.OrderStatusQueued
	} else if status == "loading" {
		filter["status"] = models.OrderStatusLoading
	} else if status == "completed" {
		filter["status"] = bson.M{"$in": []string{models.OrderStatusCompleted}}
	} else {
		// Default: show queued and loading
		filter["status"] = bson.M{"$in": []string{models.OrderStatusQueued, models.OrderStatusLoading}}
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "queue_number", Value: 1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch queue")
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

// Scan scans a barcode and creates queue entry
func (h *QueueHandler) Scan(c *fiber.Ctx) error {
	type ScanRequest struct {
		Barcode string `json:"barcode"`
	}

	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Barcode == "" {
		return response.BadRequest(c, "Barcode is required")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find order by queue barcode
	order := &models.Order{}
	err := collection.FindOne(ctx, bson.M{"queue_barcode": req.Barcode}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Invalid barcode or order not found")
	}

	// Check if order is in confirmed status (driver data filled)
	if order.Status != models.OrderStatusConfirmed {
		return response.BadRequest(c, fmt.Sprintf("Order is not ready for queue. Current status: %s", order.Status))
	}

	// Check if driver data is complete
	if order.DriverName == "" || order.DriverPhone == "" || order.VehiclePlate == "" {
		return response.BadRequest(c, "Driver data is incomplete")
	}

	// Get current max queue number for today
	today := time.Now().Format("2006-01-02")
	todayStart, _ := time.Parse("2006-01-02", today)
	todayEnd := todayStart.Add(24 * time.Hour)

	queueFilter := bson.M{
		"queue_entered_at": bson.M{
			"$gte": todayStart,
			"$lt":  todayEnd,
		},
	}
	
	maxQueue := 0
	queueCursor, _ := collection.Find(ctx, queueFilter)
	var todayOrders []models.Order
	queueCursor.All(ctx, &todayOrders)
	for _, o := range todayOrders {
		if o.QueueNumber > maxQueue {
			maxQueue = o.QueueNumber
		}
	}

	// Generate queue number and token
	queueNumber := maxQueue + 1
	queueToken := generateQueueToken()
	now := time.Now()

	// Calculate estimated time (30 min per queue ahead)
	ordersAhead := queueNumber - 1
	estimatedMinutes := ordersAhead * models.QueueDurationMinutes
	estimatedTime := now.Add(time.Duration(estimatedMinutes) * time.Minute)

	update := bson.M{
		"queue_number":     queueNumber,
		"queue_token":      queueToken,
		"queue_barcode":    "", // Clear barcode after scanning
		"queue_entered_at": now,
		"estimated_time":   estimatedTime.Format("15:04"),
		"status":           models.OrderStatusQueued,
		"updated_at":       now,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"_id": order.ID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to create queue entry")
	}

	// Get updated order
	collection.FindOne(ctx, bson.M{"_id": order.ID}).Decode(order)

	return response.Success(c, 200, fiber.Map{
		"message":        "Queue entry created successfully",
		"queue_number":   queueNumber,
		"estimated_time": estimatedTime.Format("15:04"),
		"order":          order,
	})
}

// GetEstimate returns queue estimation for an order
func (h *QueueHandler) GetEstimate(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get current loading order
	loadingOrder := &models.Order{}
	err := collection.FindOne(
		ctx,
		bson.M{"status": models.OrderStatusLoading},
		options.FindOne().SetSort(bson.D{{Key: "queue_number", Value: 1}}),
	).Decode(loadingOrder)

	// Count orders in queue ahead
	queueCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusQueued})

	// Calculate current estimated wait
	estimatedWait := 0
	if err == nil {
		// Someone is loading, add their remaining time + queue ahead
		estimatedWait = int(queueCount) * models.QueueDurationMinutes
	} else {
		// No one loading, just queue ahead
		estimatedWait = int(queueCount) * models.QueueDurationMinutes
	}

	return response.Success(c, 200, fiber.Map{
		"current_loading": loadingOrder,
		"queue_count":     queueCount,
		"estimated_wait":  fmt.Sprintf("%d minutes", estimatedWait),
		"loading":         err == nil,
	})
}

// GetCurrent returns currently loading order
func (h *QueueHandler) GetCurrent(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := &models.Order{}
	err := collection.FindOne(
		ctx,
		bson.M{"status": models.OrderStatusLoading},
		options.FindOne().SetSort(bson.D{{Key: "queue_called_at", Value: -1}}),
	).Decode(order)

	if err != nil {
		return response.Success(c, 200, fiber.Map{
			"loading": false,
			"order":   nil,
		})
	}

	// Populate sales and product data
	if order.SalesID != "" {
		salesCollection := database.GetMongoCollection("sales")
		salesObjID, _ := primitive.ObjectIDFromHex(order.SalesID)
		sales := &models.Sales{}
		salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
		order.Sales = sales
	}
	if order.ProductID != "" {
		productCollection := database.GetMongoCollection("products")
		productObjID, _ := primitive.ObjectIDFromHex(order.ProductID)
		product := &models.Product{}
		productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(product)
		order.Product = product
	}

	return response.Success(c, 200, fiber.Map{
		"loading": true,
		"order":   order,
	})
}

// CallNext calls the next order in queue
func (h *QueueHandler) CallNext(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if there's already an order loading
	loadingCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusLoading})
	if loadingCount > 0 {
		return response.BadRequest(c, "There is already an order being loaded")
	}

	// Get next order in queue
	order := &models.Order{}
	err := collection.FindOne(
		ctx,
		bson.M{"status": models.OrderStatusQueued},
		options.FindOne().SetSort(bson.D{{Key: "queue_number", Value: 1}}),
	).Decode(order)

	if err != nil {
		return response.NotFound(c, "No orders in queue")
	}

	now := time.Now()
	update := bson.M{
		"status":             models.OrderStatusLoading,
		"loading_started_at": now,
		"queue_called_at":    now,
		"updated_at":         now,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"_id": order.ID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to call next order")
	}

	// Get updated order
	collection.FindOne(ctx, bson.M{"_id": order.ID}).Decode(order)

	// Populate sales and product data
	if order.SalesID != "" {
		salesCollection := database.GetMongoCollection("sales")
		salesObjID, _ := primitive.ObjectIDFromHex(order.SalesID)
		sales := &models.Sales{}
		salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
		order.Sales = sales
	}
	if order.ProductID != "" {
		productCollection := database.GetMongoCollection("products")
		productObjID, _ := primitive.ObjectIDFromHex(order.ProductID)
		product := &models.Product{}
		productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(product)
		order.Product = product
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Next order called",
		"order":   order,
	})
}
