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

// DashboardHandler handles dashboard routes
type DashboardHandler struct{}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// GetStats returns dashboard statistics
func (h *DashboardHandler) GetStats(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orderCollection := database.GetMongoCollection("orders")
	salesCollection := database.GetMongoCollection("sales")

	// Order stats
	totalOrders, _ := orderCollection.CountDocuments(ctx, bson.M{})
	pendingOrders, _ := orderCollection.CountDocuments(ctx, bson.M{"status": models.OrderStatusPending})
	paidOrders, _ := orderCollection.CountDocuments(ctx, bson.M{"status": models.OrderStatusPaid})
	confirmedOrders, _ := orderCollection.CountDocuments(ctx, bson.M{"status": models.OrderStatusConfirmed})
	completedOrders, _ := orderCollection.CountDocuments(ctx, bson.M{"status": models.OrderStatusCompleted})
	cancelledOrders, _ := orderCollection.CountDocuments(ctx, bson.M{"status": models.OrderStatusCancelled})

	// Today's orders
	today := time.Now().Format("2006-01-02")
	todayStart, _ := time.Parse("2006-01-02", today)
	todayEnd := todayStart.Add(24 * time.Hour)

	todayFilter := bson.M{
		"created_at": bson.M{
			"$gte": todayStart,
			"$lt":  todayEnd,
		},
	}
	todayOrders, _ := orderCollection.CountDocuments(ctx, todayFilter)

	// Revenue
	revenuePipeline := []bson.M{
		{"$match": bson.M{"status": bson.M{"$ne": models.OrderStatusCancelled}}},
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$total_price"}}},
	}
	revenueCursor, _ := orderCollection.Aggregate(ctx, revenuePipeline)
	var revenueResult []struct {
		Total float64 `bson:"total"`
	}
	revenueCursor.All(ctx, &revenueResult)
	revenueCursor.Close(ctx)

	totalRevenue := 0.0
	if len(revenueResult) > 0 {
		totalRevenue = revenueResult[0].Total
	}

	// Today's revenue
	todayRevenuePipeline := []bson.M{
		{"$match": bson.M{
			"status":     bson.M{"$ne": models.OrderStatusCancelled},
			"created_at": bson.M{"$gte": todayStart, "$lt": todayEnd},
		}},
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$total_price"}}},
	}
	todayRevenueCursor, _ := orderCollection.Aggregate(ctx, todayRevenuePipeline)
	var todayRevenueResult []struct {
		Total float64 `bson:"total"`
	}
	todayRevenueCursor.All(ctx, &todayRevenueResult)
	todayRevenueCursor.Close(ctx)

	todayRevenue := 0.0
	if len(todayRevenueResult) > 0 {
		todayRevenue = todayRevenueResult[0].Total
	}

	// Sales stats
	totalSales, _ := salesCollection.CountDocuments(ctx, bson.M{})
	activeSales, _ := salesCollection.CountDocuments(ctx, bson.M{"is_active": true})

	// Top sales by revenue
	topSalesPipeline := []bson.M{
		{"$match": bson.M{"status": bson.M{"$ne": models.OrderStatusCancelled}}},
		{"$group": bson.M{
			"_id":           "$sales_id",
			"order_count":   bson.M{"$sum": 1},
			"total_revenue": bson.M{"$sum": "$total_price"},
		}},
		{"$sort": bson.M{"total_revenue": -1}},
		{"$limit": 5},
	}
	topSalesCursor, _ := orderCollection.Aggregate(ctx, topSalesPipeline)
	var topSalesResult []struct {
		ID           string  `bson:"_id"`
		OrderCount   int     `bson:"order_count"`
		TotalRevenue float64 `bson:"total_revenue"`
	}
	topSalesCursor.All(ctx, &topSalesResult)
	topSalesCursor.Close(ctx)

	// Populate sales names
	type TopSale struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Phone        string  `json:"phone"`
		OrderCount   int     `json:"order_count"`
		TotalRevenue float64 `json:"total_revenue"`
	}
	topSales := []TopSale{}
	for _, ts := range topSalesResult {
		objID, _ := primitive.ObjectIDFromHex(ts.ID)
		sales := &models.Sales{}
		salesCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(sales)
		topSales = append(topSales, TopSale{
			ID:           ts.ID,
			Name:         sales.Name,
			Phone:        sales.Phone,
			OrderCount:   ts.OrderCount,
			TotalRevenue: ts.TotalRevenue,
		})
	}

	// Recent orders
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(10)
	recentCursor, _ := orderCollection.Find(ctx, bson.M{}, findOptions)
	var recentOrders []models.Order
	recentCursor.All(ctx, &recentOrders)
	recentCursor.Close(ctx)

	// Populate sales names for recent orders
	type RecentOrder struct {
		ID          string    `json:"id"`
		OrderNumber string    `json:"order_number"`
		Status      string    `json:"status"`
		TotalPrice  float64   `json:"total_price"`
		CreatedAt   time.Time `json:"created_at"`
		SalesName   string    `json:"sales_name"`
	}
	recentOrdersResult := []RecentOrder{}
	for _, order := range recentOrders {
		salesName := ""
		if order.SalesID != "" {
			objID, _ := primitive.ObjectIDFromHex(order.SalesID)
			sales := &models.Sales{}
			salesCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(sales)
			salesName = sales.Name
		}
		recentOrdersResult = append(recentOrdersResult, RecentOrder{
			ID:          order.ID.Hex(),
			OrderNumber: order.OrderNumber,
			Status:      order.Status,
			TotalPrice:  order.TotalPrice,
			CreatedAt:   order.CreatedAt,
			SalesName:   salesName,
		})
	}

	return response.Success(c, 200, fiber.Map{
		"orders": fiber.Map{
			"total":         totalOrders,
			"pending":       pendingOrders,
			"paid":          paidOrders,
			"confirmed":     confirmedOrders,
			"completed":     completedOrders,
			"cancelled":     cancelledOrders,
			"today":         todayOrders,
			"revenue":       totalRevenue,
			"today_revenue": todayRevenue,
		},
		"sales": fiber.Map{
			"total":  totalSales,
			"active": activeSales,
		},
		"top_sales":     topSales,
		"recent_orders": recentOrdersResult,
	})
}
