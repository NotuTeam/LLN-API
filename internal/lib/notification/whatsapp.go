package notification

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/whatsapp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationType represents different notification types
type NotificationType string

const (
	NotificationTypeInvoice  NotificationType = "invoice"
	NotificationTypeDelivery NotificationType = "delivery"
	NotificationTypeQueue    NotificationType = "queue"
)

// Notification represents a notification record
type Notification struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type      NotificationType   `json:"type" bson:"type"`
	Phone     string             `json:"phone" bson:"phone"`
	Message   string             `json:"message" bson:"message"`
	Link      string             `json:"link" bson:"link"`
	OrderID   string             `json:"order_id" bson:"order_id"`
	Status    string             `json:"status" bson:"status"` // pending, sent, failed
	SentAt    *time.Time         `json:"sent_at,omitempty" bson:"sent_at,omitempty"`
	SentVia   string             `json:"sent_via,omitempty" bson:"sent_via,omitempty"` // "whatsapp" or "wa.me"
}

// WhatsAppConfig holds WhatsApp configuration
type WhatsAppConfig struct {
	ClientURL string
}

// Config is the global notification config
var Config WhatsAppConfig

// Init initializes the notification service
func Init(clientURL string) {
	Config = WhatsAppConfig{
		ClientURL: clientURL,
	}
}

// GenerateWhatsAppLink generates a wa.me link with pre-filled message
func GenerateWhatsAppLink(phone string, message string) string {
	// Clean phone number (remove +, spaces, dashes)
	cleanPhone := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			cleanPhone += string(c)
		}
	}

	// Encode message
	encodedMessage := url.QueryEscape(message)

	return fmt.Sprintf("https://wa.me/%s?text=%s", cleanPhone, encodedMessage)
}

// sendViaWhatsApp tries to send via whatsmeow if connected
// Returns true if sent, false if fallback to wa.me link needed
func sendViaWhatsApp(phone string, message string) bool {
	if whatsapp.WhatsApp == nil {
		return false
	}

	if !whatsapp.WhatsApp.IsLoggedIn() {
		return false
	}

	err := whatsapp.WhatsApp.SendMessage(phone, message)
	if err != nil {
		log.Printf("[Notification] Failed to send via WhatsApp: %v", err)
		return false
	}

	log.Printf("[Notification] Message sent via WhatsApp to %s", phone)
	return true
}

// SendInvoiceNotification creates invoice notification and sends via WhatsApp if connected
func SendInvoiceNotification(phone string, salesName string, orderNumber string, productName string, quantity int, unit string, totalPrice float64, invoiceToken string) (string, error) {
	invoiceURL := fmt.Sprintf("%s/order/%s", Config.ClientURL, invoiceToken)

	message := fmt.Sprintf(`Halo %s,

Invoice order Anda telah dibuat:

No. Order: %s
Produk: %s
Jumlah: %d %s
Total: Rp %.0f

Silakan akses link berikut untuk melakukan pembayaran:
%s

Terima kasih.`,
		salesName, orderNumber, productName, quantity, unit, totalPrice, invoiceURL)

	// Try to send via WhatsApp if connected
	sentVia := "wa.me"
	if sendViaWhatsApp(phone, message) {
		sentVia = "whatsapp"
	}

	// Save notification record
	now := time.Now()
	notification := Notification{
		ID:      primitive.NewObjectID(),
		Type:    NotificationTypeInvoice,
		Phone:   phone,
		Message: message,
		Link:    invoiceURL,
		Status:  "sent",
		SentVia: sentVia,
	}
	
	if sentVia == "whatsapp" {
		notification.SentAt = &now
	}

	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		return "", err
	}

	// Always return wa.me link as fallback
	return GenerateWhatsAppLink(phone, message), nil
}

// SendDeliveryNotification creates delivery notification and sends via WhatsApp if connected
func SendDeliveryNotification(phone string, salesName string, noteNumber string, productName string, qty int, unit string, driverName string, vehiclePlate string, deliveryToken string) (string, error) {
	deliveryURL := fmt.Sprintf("%s/order/%s", Config.ClientURL, deliveryToken)

	message := fmt.Sprintf(`Halo %s,

Surat jalan untuk order Anda telah dibuat:

No. Surat Jalan: %s
Produk: %s
Jumlah: %d %s
Driver: %s
No. Polisi: %s

Silakan akses link berikut untuk melihat surat jalan:
%s

Terima kasih.`,
		salesName, noteNumber, productName, qty, unit, driverName, vehiclePlate, deliveryURL)

	// Try to send via WhatsApp if connected
	sentVia := "wa.me"
	if sendViaWhatsApp(phone, message) {
		sentVia = "whatsapp"
	}

	// Save notification record
	now := time.Now()
	notification := Notification{
		ID:      primitive.NewObjectID(),
		Type:    NotificationTypeDelivery,
		Phone:   phone,
		Message: message,
		Link:    deliveryURL,
		Status:  "sent",
		SentVia: sentVia,
	}
	
	if sentVia == "whatsapp" {
		notification.SentAt = &now
	}

	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		return "", err
	}

	return GenerateWhatsAppLink(phone, message), nil
}

// SendQueueNotification creates queue notification and sends via WhatsApp if connected
func SendQueueNotification(phone string, salesName string, orderNumber string, queueNumber int, estimatedTime string, queueToken string) (string, error) {
	queueURL := fmt.Sprintf("%s/order/%s", Config.ClientURL, queueToken)

	message := fmt.Sprintf(`Halo %s,

Update antrian order Anda:

No. Order: %s
No. Antrian: #%d
Estimasi: %s

Silakan pantau status antrian Anda melalui link:
%s

Terima kasih.`,
		salesName, orderNumber, queueNumber, estimatedTime, queueURL)

	// Try to send via WhatsApp if connected
	sentVia := "wa.me"
	if sendViaWhatsApp(phone, message) {
		sentVia = "whatsapp"
	}

	// Save notification record
	now := time.Now()
	notification := Notification{
		ID:      primitive.NewObjectID(),
		Type:    NotificationTypeQueue,
		Phone:   phone,
		Message: message,
		Link:    queueURL,
		Status:  "sent",
		SentVia: sentVia,
	}
	
	if sentVia == "whatsapp" {
		notification.SentAt = &now
	}

	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		return "", err
	}

	return GenerateWhatsAppLink(phone, message), nil
}

// MarkAsSent marks a notification as sent
func MarkAsSent(notificationID primitive.ObjectID) error {
	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	update := bson.M{
		"status":  "sent",
		"sent_at": now,
	}

	_, err := collection.UpdateOne(ctx, bson.M{"_id": notificationID}, bson.M{"$set": update})
	return err
}

// GetPendingNotifications returns all pending notifications
func GetPendingNotifications() ([]Notification, error) {
	collection := database.GetMongoCollection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{"status": "pending"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notifications []Notification
	cursor.All(ctx, &notifications)

	return notifications, nil
}

// WhatsAppStatus returns current WhatsApp connection status
func WhatsAppStatus() map[string]interface{} {
	if whatsapp.WhatsApp == nil {
		return map[string]interface{}{
			"initialized": false,
			"connected":   false,
			"logged_in":   false,
		}
	}

	return map[string]interface{}{
		"initialized": true,
		"connected":   whatsapp.WhatsApp.IsConnected(),
		"logged_in":   whatsapp.WhatsApp.IsLoggedIn(),
	}
}
