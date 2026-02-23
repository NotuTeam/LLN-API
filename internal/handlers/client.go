package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/file"
	"bg-go/internal/lib/qrcode"
	"bg-go/internal/lib/response"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ClientHandler handles client-side routes (public with token)
type ClientHandler struct{}

// NewClientHandler creates a new client handler
func NewClientHandler() *ClientHandler {
	return &ClientHandler{}
}

// generateQueueBarcode generates a unique barcode for queue
func generateQueueBarcode() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetInvoice returns invoice data by token
func (h *ClientHandler) GetInvoice(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := &models.Order{}
	err := collection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Invoice not found")
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

	return response.Success(c, 200, order)
}

// UploadPayment uploads payment proof by token
func (h *ClientHandler) UploadPayment(c *fiber.Ctx) error {
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

	// Check if payment can be uploaded
	if order.Status != models.OrderStatusPending {
		return response.BadRequest(c, "Cannot upload payment for this order status")
	}

	if order.PaymentStatus == models.PaymentStatusVerified {
		return response.BadRequest(c, "Payment already verified")
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
		"status":        models.OrderStatusPaid,
	})
}

// SubmitDriver submits driver data by token
func (h *ClientHandler) SubmitDriver(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
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

	if req.DriverName == "" || req.DriverPhone == "" || req.VehiclePlate == "" {
		return response.BadRequest(c, "Driver name, phone, and vehicle plate are required")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find order by invoice token
	order := &models.Order{}
	err := collection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// Check if payment is verified
	if order.PaymentStatus != models.PaymentStatusVerified {
		return response.BadRequest(c, "Payment must be verified first")
	}

	// Generate barcode and QR code
	barcode := generateQueueBarcode()
	qrCodeBase64, err := qrcode.GenerateQRCode(barcode)
	if err != nil {
		return response.Error(c, 500, "Failed to generate QR code")
	}

	now := time.Now()
	update := bson.M{
		"driver_name":      req.DriverName,
		"driver_phone":     req.DriverPhone,
		"vehicle_plate":    req.VehiclePlate,
		"driver_filled_at": now,
		"queue_barcode":    barcode,
		"queue_qrcode":     qrCodeBase64,
		"updated_at":       now,
	}

	_, err = collection.UpdateOne(ctx, bson.M{"invoice_token": token}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update order")
	}

	// Get updated order
	collection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)

	return response.Success(c, 200, fiber.Map{
		"message":       "Driver data submitted successfully",
		"queue_barcode": order.QueueBarcode,
		"queue_qrcode":  order.QueueQRCode,
	})
}

// UploadVehiclePhoto uploads vehicle photo by token
func (h *ClientHandler) UploadVehiclePhoto(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	formFile, err := c.FormFile("photo")
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

	vehiclePhoto := &models.Image{
		PublicID: uploadResult.PublicID,
		URL:      uploadResult.URL,
	}

	update := bson.M{
		"vehicle_photo": vehiclePhoto,
		"updated_at":    time.Now(),
	}

	_, err = collection.UpdateOne(ctx, bson.M{"invoice_token": token}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update order")
	}

	return response.Success(c, 200, fiber.Map{
		"message":       "Vehicle photo uploaded successfully",
		"vehicle_photo": vehiclePhoto,
	})
}

// GetQueueStatus returns queue status by token
func (h *ClientHandler) GetQueueStatus(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	orderCollection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find order by invoice token
	order := &models.Order{}
	err := orderCollection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	// If order is queued or loading, calculate estimated time
	estimatedWait := ""
	var ordersAhead int64 = 0
	currentLoading := false

	if order.Status == models.OrderStatusQueued {
		// Count orders ahead in queue
		filter := bson.M{
			"status":       bson.M{"$in": []string{models.OrderStatusQueued, models.OrderStatusLoading}},
			"queue_number": bson.M{"$lt": order.QueueNumber},
		}
		ordersAhead, _ = orderCollection.CountDocuments(ctx, filter)

		// Calculate estimated wait
		estimatedMinutes := int(ordersAhead) * models.QueueDurationMinutes
		estimatedWait = formatDuration(estimatedMinutes)
	}

	if order.Status == models.OrderStatusLoading {
		currentLoading = true
	}

	// Get currently loading order info
	currentOrder := &models.Order{}
	err = orderCollection.FindOne(ctx, bson.M{"status": models.OrderStatusLoading}).Decode(currentOrder)
	isLoading := err == nil

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
		"order":           order,
		"status":          order.Status,
		"queue_number":    order.QueueNumber,
		"estimated_wait":  estimatedWait,
		"orders_ahead":    ordersAhead,
		"current_loading": currentLoading,
		"is_loading":      isLoading,
	})
}

// GetDeliveryNote returns delivery note by token
func (h *ClientHandler) GetDeliveryNote(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	deliveryCollection := database.GetMongoCollection("delivery_notes")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to find by delivery note token first
	note := &models.DeliveryNote{}
	err := deliveryCollection.FindOne(ctx, bson.M{"token": token}).Decode(note)
	if err == nil {
		return response.Success(c, 200, note)
	}

	// Try to find by order's invoice token
	orderCollection := database.GetMongoCollection("orders")
	order := &models.Order{}
	err = orderCollection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Delivery note not found")
	}

	if order.DeliveryNoteID == "" {
		return response.NotFound(c, "Delivery note not ready")
	}

	noteObjID, _ := primitive.ObjectIDFromHex(order.DeliveryNoteID)
	err = deliveryCollection.FindOne(ctx, bson.M{"_id": noteObjID}).Decode(note)
	if err != nil {
		return response.NotFound(c, "Delivery note not found")
	}

	return response.Success(c, 200, note)
}

// GetOrderStatus returns current order status for polling
func (h *ClientHandler) GetOrderStatus(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return response.BadRequest(c, "Token is required")
	}

	collection := database.GetMongoCollection("orders")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	order := &models.Order{}
	err := collection.FindOne(ctx, bson.M{"invoice_token": token}).Decode(order)
	if err != nil {
		return response.NotFound(c, "Order not found")
	}

	return response.Success(c, 200, fiber.Map{
		"status":            order.Status,
		"payment_status":    order.PaymentStatus,
		"queue_number":      order.QueueNumber,
		"estimated_time":    order.EstimatedTime,
		"delivery_note_id":  order.DeliveryNoteID,
		"delivery_note_url": order.DeliveryNoteURL,
	})
}

// formatDuration formats minutes to human readable string
func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d menit", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		return fmt.Sprintf("%d jam", hours)
	}
	return fmt.Sprintf("%d jam %d menit", hours, mins)
}
