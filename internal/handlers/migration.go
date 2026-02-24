package handlers

import (
	"context"
	"fmt"
	"time"

	"bg-go/internal/database"
	"bg-go/internal/lib/response"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MigrationHandler handles data migration/cleanup
type MigrationHandler struct{}

// NewMigrationHandler creates a new migration handler
func NewMigrationHandler() *MigrationHandler {
	return &MigrationHandler{}
}

// CleanupOrders cleans up old/obsolete fields from orders
func (h *MigrationHandler) CleanupOrders(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orderCollection := database.GetMongoCollection("orders")
	productCollection := database.GetMongoCollection("products")

	// 1. Drop products collection (no longer used)
	err := productCollection.Drop(ctx)
	if err != nil {
		fmt.Printf("Warning: Could not drop products collection: %v\n", err)
	}

	// 2. Clean up orders - remove obsolete fields
	// Fields to unset:
	// - product_id (legacy)
	// - unit_price (legacy, now in items)
	// - invoice_url (redundant, we have invoice_token)
	// - payment_verified_by (not used)
	// - payment_rejected_by (not used)
	// - loading_started_at (not used)
	// - loading_finished_at (not used)
	unsetFields := bson.M{
		"product_id":          "",
		"unit_price":          "",
		"invoice_url":         "",
		"payment_verified_by": "",
		"payment_rejected_by": "",
		"loading_started_at":  "",
		"loading_finished_at": "",
	}

	// Update all orders
	result, err := orderCollection.UpdateMany(
		ctx,
		bson.M{},
		bson.M{"$unset": unsetFields},
	)
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("Failed to cleanup orders: %v", err))
	}

	// 3. Clean up order items - remove product_id from items array
	// We need to iterate through orders and update items
	cursor, err := orderCollection.Find(ctx, bson.M{})
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("Failed to find orders: %v", err))
	}
	defer cursor.Close(ctx)

	ordersUpdated := 0
	for cursor.Next(ctx) {
		var order struct {
			ID    primitive.ObjectID `bson:"_id"`
			Items []bson.M           `bson:"items"`
		}
		cursor.Decode(&order)

		if len(order.Items) > 0 {
			// Clean up each item
			cleanItems := make([]bson.M, len(order.Items))
			for i, item := range order.Items {
				// Keep only relevant fields
				cleanItem := bson.M{
					"product_name": item["product_name"],
					"unit_price":   item["unit_price"],
					"quantity":     item["quantity"],
					"unit":         item["unit"],
					"subtotal":     item["subtotal"],
				}
				// Set default unit if missing
				if cleanItem["unit"] == nil || cleanItem["unit"] == "" {
					cleanItem["unit"] = "pcs"
				}
				cleanItems[i] = cleanItem
			}

			// Update order
			_, err := orderCollection.UpdateByID(
				ctx,
				order.ID,
				bson.M{"$set": bson.M{"items": cleanItems}},
			)
			if err == nil {
				ordersUpdated++
			}
		}
	}

	return response.Success(c, 200, fiber.Map{
		"message":               "Cleanup completed",
		"orders_modified":        result.ModifiedCount,
		"orders_items_cleaned":  ordersUpdated,
		"products_dropped":      err == nil,
	})
}

// ResetOrders drops all orders and related collections (for fresh start)
func (h *MigrationHandler) ResetOrders(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Drop collections
	orderCollection := database.GetMongoCollection("orders")
	deliveryCollection := database.GetMongoCollection("delivery_notes")
	notificationCollection := database.GetMongoCollection("notifications")
	productCollection := database.GetMongoCollection("products")

	// Drop orders
	err1 := orderCollection.Drop(ctx)
	
	// Drop delivery notes
	err2 := deliveryCollection.Drop(ctx)
	
	// Drop notifications
	err3 := notificationCollection.Drop(ctx)
	
	// Drop products (no longer needed)
	err4 := productCollection.Drop(ctx)

	return response.Success(c, 200, fiber.Map{
		"message":           "Reset completed",
		"orders_dropped":    err1 == nil,
		"delivery_dropped":  err2 == nil,
		"notifications_dropped": err3 == nil,
		"products_dropped":  err4 == nil,
	})
}

// GetOrderStats shows current order data statistics
func (h *MigrationHandler) GetOrderStats(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orderCollection := database.GetMongoCollection("orders")
	productCollection := database.GetMongoCollection("products")
	deliveryCollection := database.GetMongoCollection("delivery_notes")
	salesCollection := database.GetMongoCollection("sales")

	// Count documents
	orderCount, _ := orderCollection.CountDocuments(ctx, bson.M{})
	productCount, _ := productCollection.CountDocuments(ctx, bson.M{})
	deliveryCount, _ := deliveryCollection.CountDocuments(ctx, bson.M{})
	salesCount, _ := salesCollection.CountDocuments(ctx, bson.M{})

	// Count orders with legacy fields
	ordersWithProductID, _ := orderCollection.CountDocuments(ctx, bson.M{"product_id": bson.M{"$exists": true, "$ne": ""}})
	ordersWithInvoiceURL, _ := orderCollection.CountDocuments(ctx, bson.M{"invoice_url": bson.M{"$exists": true, "$ne": ""}})
	ordersWithItems, _ := orderCollection.CountDocuments(ctx, bson.M{"items": bson.M{"$exists": true, "$ne": nil}})

	// Sample order to show structure
	var sampleOrder bson.M
	orderCollection.FindOne(ctx, bson.M{}).Decode(&sampleOrder)

	return response.Success(c, 200, fiber.Map{
		"counts": fiber.Map{
			"orders":    orderCount,
			"products":  productCount,
			"delivery":  deliveryCount,
			"sales":     salesCount,
		},
		"legacy_fields": fiber.Map{
			"orders_with_product_id": ordersWithProductID,
			"orders_with_invoice_url": ordersWithInvoiceURL,
			"orders_with_items": ordersWithItems,
		},
		"sample_order": sampleOrder,
	})
}
