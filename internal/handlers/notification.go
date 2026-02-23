package handlers

import (
	"context"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/notification"
	"bg-go/internal/lib/response"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NotificationHandler handles notification routes
type NotificationHandler struct{}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{}
}

// List returns all notifications with pagination
func (h *NotificationHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	status := c.Query("status")
	notifType := c.Query("type")

	skip := (page - 1) * limit

	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	if notifType != "" {
		filter["type"] = notifType
	}

	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, filter)

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch notifications")
	}
	defer cursor.Close(ctx)

	var notifs []notification.Notification
	cursor.All(ctx, &notifs)

	return response.SuccessWithPagination(c, 200, notifs, response.CalculatePagination(int64(page), int64(limit), total))
}

// GetPending returns all pending notifications
func (h *NotificationHandler) GetPending(c *fiber.Ctx) error {
	notifs, err := notification.GetPendingNotifications()
	if err != nil {
		return response.Error(c, 500, "Failed to fetch pending notifications")
	}

	return response.Success(c, 200, notifs)
}

// MarkAsSent marks a notification as sent
func (h *NotificationHandler) MarkAsSent(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid ID format")
	}

	err = notification.MarkAsSent(objID)
	if err != nil {
		return response.Error(c, 500, "Failed to mark notification as sent")
	}

	return response.SuccessWithMessage(c, 200, "Notification marked as sent")
}

// SendManual sends a manual notification
func (h *NotificationHandler) SendManual(c *fiber.Ctx) error {
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

	link := notification.GenerateWhatsAppLink(req.Phone, req.Message)

	return response.Success(c, 200, fiber.Map{
		"whatsapp_link": link,
		"phone":         req.Phone,
		"message":       req.Message,
	})
}

// GetStats returns notification statistics
func (h *NotificationHandler) GetStats(c *fiber.Ctx) error {
	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pendingCount, _ := collection.CountDocuments(ctx, bson.M{"status": "pending"})
	sentCount, _ := collection.CountDocuments(ctx, bson.M{"status": "sent"})

	// Count by type
	invoiceCount, _ := collection.CountDocuments(ctx, bson.M{"type": notification.NotificationTypeInvoice})
	deliveryCount, _ := collection.CountDocuments(ctx, bson.M{"type": notification.NotificationTypeDelivery})
	queueCount, _ := collection.CountDocuments(ctx, bson.M{"type": notification.NotificationTypeQueue})

	return response.Success(c, 200, fiber.Map{
		"pending": pendingCount,
		"sent":    sentCount,
		"by_type": fiber.Map{
			"invoice":  invoiceCount,
			"delivery": deliveryCount,
			"queue":    queueCount,
		},
	})
}
