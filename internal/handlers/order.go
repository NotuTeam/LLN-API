package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"bg-go/internal/config"
	"bg-go/internal/database"
	"bg-go/internal/lib/notification"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrderHandler handles order routes
type OrderHandler struct{}

// NewOrderHandler creates a new order handler
func NewOrderHandler() *OrderHandler {
	return &OrderHandler{}
}

// generateToken generates a random token
func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateOrderNumber generates order number
func generateOrderNumber() string {
	return fmt.Sprintf("ORD-%s", time.Now().Format("20060102150405"))
}

// generateQRCode generates a QR code image as base64
func generateQRCode(content string) (string, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// List returns all orders with pagination and filters
func (h *OrderHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	search := c.Query("search")
	status := c.Query("status")

	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter["$or"] = []bson.M{
			{"order_number": bson.M{"$regex": search, "$options": "i"}},
		}
	}
	if status != "" {
		filter["status"] = status
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch orders")
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	cursor.All(ctx, &orders)

	// Populate sales data
	salesCollection := database.GetMongoCollection("sales")
	for i := range orders {
		if orders[i].SalesID != "" {
			salesObjID, _ := primitive.ObjectIDFromHex(orders[i].SalesID)
			sales := &models.Sales{}
			salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
			orders[i].Sales = sales
		}

		// Populate virtual product for items (using manual data)
		for j := range orders[i].Items {
			orders[i].Items[j].Product = &models.Product{
				Name:  orders[i].Items[j].ProductName,
				Price: orders[i].Items[j].UnitPrice,
				Unit:  orders[i].Items[j].Unit,
			}
		}
	}

	return response.SuccessWithPagination(c, 200, orders, response.CalculatePagination(int64(page), int64(limit), total))
}

// Detail returns a single order by ID
func (h *OrderHandler) Detail(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Populate sales data
	if order.SalesID != "" {
		salesCollection := database.GetMongoCollection("sales")
		salesObjID, _ := primitive.ObjectIDFromHex(order.SalesID)
		sales := &models.Sales{}
		salesCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(sales)
		order.Sales = sales
	}

	// Populate virtual product for items
	for j := range order.Items {
		order.Items[j].Product = &models.Product{
			Name:  order.Items[j].ProductName,
			Price: order.Items[j].UnitPrice,
			Unit:  order.Items[j].Unit,
		}
	}

	return response.Success(c, 200, order)
}

// CreateItem represents an item in the create order request
type CreateItem struct {
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Unit        string  `json:"unit"` // Optional, default "pcs"
}

// CreateRequest represents the create order request
type CreateRequest struct {
	SalesID string       `json:"sales_id"`
	Items   []CreateItem `json:"items"`
}

// Create creates a new order
func (h *OrderHandler) Create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate sales
	if req.SalesID == "" {
		return response.BadRequest(c, "Sales is required")
	}

	// Validate items
	if len(req.Items) == 0 {
		return response.BadRequest(c, "At least one item is required")
	}

	// Validate sales exists
	salesCollection := database.GetMongoCollection("sales")
	salesCtx, salesCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer salesCancel()

	salesObjID, _ := primitive.ObjectIDFromHex(req.SalesID)
	sales := &models.Sales{}
	err := salesCollection.FindOne(salesCtx, bson.M{"_id": salesObjID}).Decode(sales)
	if err != nil {
		return response.BadRequest(c, "Sales not found")
	}

	// Create order
	order := models.NewOrder()
	order.SalesID = req.SalesID
	order.Items = []models.OrderItem{}

	// Process items
	var totalPrice float64 = 0
	productNames := []string{}
	totalQuantity := 0

	for _, item := range req.Items {
		if item.ProductName == "" || item.Quantity <= 0 {
			continue
		}

		// Default unit to "pcs"
		unit := item.Unit
		if unit == "" {
			unit = "pcs"
		}

		subtotal := item.UnitPrice * float64(item.Quantity)

		order.Items = append(order.Items, models.OrderItem{
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Unit:        unit,
			Subtotal:    subtotal,
		})

		totalPrice += subtotal
		productNames = append(productNames, item.ProductName)
		totalQuantity += item.Quantity
	}

	if len(order.Items) == 0 {
		return response.BadRequest(c, "No valid items provided")
	}

	// Set totals
	order.Quantity = totalQuantity
	order.UnitPrice = totalPrice / float64(totalQuantity)
	order.TotalPrice = totalPrice

	// Generate invoice token
	invoiceToken := generateToken(32)
	order.OrderNumber = generateOrderNumber()
	order.Status = models.OrderStatusPending
	order.PaymentStatus = models.PaymentStatusPending
	order.InvoiceToken = invoiceToken
	order.InvoiceURL = fmt.Sprintf("%s/order/%s", config.Cfg.Client.URL, invoiceToken)

	// Save order
	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = collection.InsertOne(ctx, order)
	if err != nil {
		return response.Error(c, 500, "Failed to create order")
	}

	// Populate virtual product for items
	for i := range order.Items {
		order.Items[i].Product = &models.Product{
			Name:  order.Items[i].ProductName,
			Price: order.Items[i].UnitPrice,
			Unit:  order.Items[i].Unit,
		}
	}

	// Populate sales for response
	order.Sales = sales

	// Get first product name for notification
	firstProductName := ""
	if len(productNames) > 0 {
		firstProductName = productNames[0]
		if len(productNames) > 1 {
			firstProductName = fmt.Sprintf("%s (+%d lainnya)", productNames[0], len(productNames)-1)
		}
	}

	// Generate WhatsApp notification link
	notification.Init(config.Cfg.Client.URL)
	waLink, _ := notification.SendInvoiceNotification(
		sales.Phone,
		sales.Name,
		order.OrderNumber,
		firstProductName,
		totalQuantity,
		"item",
		totalPrice,
		invoiceToken,
	)

	// Check WhatsApp status
	waStatus := notification.WhatsAppStatus()

	return response.Success(c, 201, fiber.Map{
		"order":              order,
		"whatsapp_link":      waLink,
		"whatsapp_connected": waStatus["logged_in"].(bool),
		"whatsapp_auto_sent": waStatus["logged_in"].(bool),
	})
}

// Update updates an order
func (h *OrderHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	type UpdateRequest struct {
		Status string `json:"status,omitempty"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updated_at": time.Now()}
	if req.Status != "" {
		update["status"] = req.Status
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update order")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.SuccessWithMessage(c, 200, "Successfully updated")
}

// Delete cancels an order
func (h *OrderHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{"status": models.OrderStatusCancelled, "updated_at": time.Now()}},
	)
	if err != nil {
		return response.Error(c, 500, "Failed to cancel order")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.SuccessWithMessage(c, 200, "Order cancelled successfully")
}

// GetStats returns order statistics for dashboard
func (h *OrderHandler) GetStats(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Count by status
	pendingCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusPending})
	paidCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusPaid})
	confirmedCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusConfirmed})
	queuedCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusQueued})
	loadingCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusLoading})
	completedCount, _ := collection.CountDocuments(ctx, bson.M{"status": models.OrderStatusCompleted})

	// Today's stats
	today := time.Now().Format("2006-01-02")
	todayStart, _ := time.Parse("2006-01-02", today)
	todayEnd := todayStart.Add(24 * time.Hour)

	todayFilter := bson.M{
		"created_at": bson.M{
			"$gte": todayStart,
			"$lt":  todayEnd,
		},
	}
	todayOrders, _ := collection.CountDocuments(ctx, todayFilter)

	// Total revenue
	pipeline := []bson.M{
		{"$match": bson.M{"status": bson.M{"$ne": models.OrderStatusCancelled}}},
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$total_price"}}},
	}
	cursor, _ := collection.Aggregate(ctx, pipeline)
	var revenueResult []struct {
		Total float64 `bson:"total"`
	}
	cursor.All(ctx, &revenueResult)
	cursor.Close(ctx)

	totalRevenue := 0.0
	if len(revenueResult) > 0 {
		totalRevenue = revenueResult[0].Total
	}

	return response.Success(c, 200, fiber.Map{
		"pending":   pendingCount,
		"paid":      paidCount,
		"confirmed": confirmedCount,
		"queued":    queuedCount,
		"loading":   loadingCount,
		"completed": completedCount,
		"today":     todayOrders,
		"revenue":   totalRevenue,
	})
}

// FinishLoading marks an order as finished loading
func (h *OrderHandler) FinishLoading(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get order
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Check status
	if order.Status != models.OrderStatusLoading {
		return response.BadRequest(c, "Order is not in loading status")
	}

	// Update status
	now := time.Now()
	update := bson.M{
		"status":       models.OrderStatusCompleted,
		"completed_at": now,
		"updated_at":   now,
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to finish loading")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Loading finished successfully",
	})
}

// SubmitDriver submits driver data for an order
func (h *OrderHandler) SubmitDriver(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	type DriverRequest struct {
		DriverName   string `json:"driver_name"`
		DriverPhone  string `json:"driver_phone"`
		VehiclePlate string `json:"vehicle_plate"`
	}

	var req DriverRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate
	if req.DriverName == "" || req.DriverPhone == "" || req.VehiclePlate == "" {
		return response.BadRequest(c, "All driver fields are required")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get order
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Check status
	if order.Status != models.OrderStatusConfirmed {
		return response.BadRequest(c, "Order must be confirmed first")
	}

	// Generate QR code for queue
	qrToken := generateToken(16)
	qrCode, _ := generateQRCode(qrToken)

	// Update order
	update := bson.M{
		"driver_name":    req.DriverName,
		"driver_phone":   req.DriverPhone,
		"vehicle_plate":  req.VehiclePlate,
		"queue_barcode":  qrToken,
		"queue_qrcode":   qrCode,
		"updated_at":     time.Now(),
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to submit driver data")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.Success(c, 200, fiber.Map{
		"message":     "Driver data submitted successfully",
		"queue_token": qrToken,
	})
}

// CallQueue calls an order from the queue
func (h *OrderHandler) CallQueue(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get order
	order := &models.Order{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Check status
	if order.Status != models.OrderStatusQueued {
		return response.BadRequest(c, "Order is not in queue")
	}

	// Update status to loading
	now := time.Now()
	update := bson.M{
		"status":         models.OrderStatusLoading,
		"loading_at":     now,
		"updated_at":     now,
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to call order")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "Order not found")
	}

	return response.Success(c, 200, fiber.Map{
		"message": "Order called successfully",
	})
}
