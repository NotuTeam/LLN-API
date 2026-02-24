package routes

import (
	"bg-go/internal/handlers"
	"bg-go/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes sets up all routes
func SetupRoutes(app *fiber.App) {
	// Auth handler
	authHandler := handlers.NewAuthHandler()
	
	// Settings handler
	settingsHandler := handlers.NewSettingsHandler()
	
	// Migration handler
	migrationHandler := handlers.NewMigrationHandler()

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Server is running",
		})
	})

	// API v1 routes
	v1 := app.Group("/api/v1")

	// ============================================
	// Migration Routes (SUPERADMIN only)
	// ============================================
	migration := v1.Group("/migration", middleware.AuthGuard(), middleware.RoleGuard("SUPERADMIN"))
	migration.Get("/stats", migrationHandler.GetOrderStats)
	migration.Post("/cleanup-orders", migrationHandler.CleanupOrders)
	migration.Post("/reset-orders", migrationHandler.ResetOrders)

	// ============================================
	// Dashboard Routes (Protected)
	// ============================================
	dashboardHandler := handlers.NewDashboardHandler()
	dashboard := v1.Group("/dashboard", middleware.AuthGuard())
	dashboard.Get("/stats", dashboardHandler.GetStats)

	// ============================================
	// Auth Routes
	// ============================================
	auth := v1.Group("/auth")
	auth.Get("/genesis", authHandler.Genesis)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)

	// Protected auth routes
	authProtected := auth.Group("/", middleware.AuthGuard())
	authProtected.Get("/me", authHandler.Me)
	authProtected.Get("/users", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.ListUsers)
	authProtected.Get("/list", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.ListUsers) // Alias for frontend compatibility
	authProtected.Post("/register", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.Register)
	authProtected.Put("/users/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.UpdateUser)
	authProtected.Put("/adjust/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.UpdateUser) // Alias for frontend
	authProtected.Delete("/users/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.DeleteUser)
	authProtected.Delete("/takedown/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), authHandler.DeleteUser) // Alias for frontend

	// ============================================
	// Sales Routes (Protected)
	// ============================================
	salesHandler := handlers.NewSalesHandler()
	sales := v1.Group("/sales", middleware.AuthGuard())
	sales.Get("/", salesHandler.List)
	sales.Get("/:id", salesHandler.Detail)
	sales.Post("/", middleware.RoleGuard("SUPERADMIN", "ADMIN"), salesHandler.Create)
	sales.Put("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), salesHandler.Update)
	sales.Delete("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), salesHandler.Delete)

	// ============================================
	// Product Routes (Protected)
	// ============================================
	productHandler := handlers.NewProductHandler()
	products := v1.Group("/products", middleware.AuthGuard())
	products.Get("/", productHandler.List)
	products.Get("/:id", productHandler.Detail)
	products.Post("/", middleware.RoleGuard("SUPERADMIN", "ADMIN"), productHandler.Create)
	products.Put("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), productHandler.Update)
	products.Delete("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), productHandler.Delete)
	products.Post("/:id/image", middleware.RoleGuard("SUPERADMIN", "ADMIN"), productHandler.UploadImage)

	// ============================================
	// Order Routes (Protected)
	// ============================================
	orderHandler := handlers.NewOrderHandler()
	orders := v1.Group("/orders", middleware.AuthGuard())
	orders.Get("/", orderHandler.List)
	orders.Get("/stats", orderHandler.GetStats)
	orders.Get("/:id", orderHandler.Detail)
	orders.Post("/", middleware.RoleGuard("SUPERADMIN", "ADMIN"), orderHandler.Create)
	orders.Put("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), orderHandler.Update)
	orders.Delete("/:id", middleware.RoleGuard("SUPERADMIN", "ADMIN"), orderHandler.Delete)
	orders.Post("/:id/call", middleware.RoleGuard("SUPERADMIN", "ADMIN"), orderHandler.CallQueue)
	orders.Post("/:id/finish-loading", middleware.RoleGuard("SUPERADMIN", "ADMIN"), orderHandler.FinishLoading)

	// ============================================
	// Payment Routes (Protected)
	// ============================================
	paymentHandler := handlers.NewPaymentHandler()
	payments := v1.Group("/payments", middleware.AuthGuard())
	payments.Get("/pending", paymentHandler.ListPending)
	payments.Get("/status/:status", paymentHandler.ListByStatus)
	payments.Post("/:id/verify", middleware.RoleGuard("SUPERADMIN", "ADMIN"), paymentHandler.Verify)
	payments.Post("/:id/reject", middleware.RoleGuard("SUPERADMIN", "ADMIN"), paymentHandler.Reject)

	// ============================================
	// Queue Routes (Protected)
	// ============================================
	queueHandler := handlers.NewQueueHandler()
	queue := v1.Group("/queue", middleware.AuthGuard())
	queue.Get("/", queueHandler.List)
	queue.Get("/estimate", queueHandler.GetEstimate)
	queue.Get("/current", queueHandler.GetCurrent)
	queue.Post("/scan", middleware.RoleGuard("SUPERADMIN", "ADMIN"), queueHandler.Scan)
	queue.Post("/call-next", middleware.RoleGuard("SUPERADMIN", "ADMIN"), queueHandler.CallNext)

	// ============================================
	// Delivery Routes (Protected)
	// ============================================
	deliveryHandler := handlers.NewDeliveryHandler()
	delivery := v1.Group("/delivery", middleware.AuthGuard())
	delivery.Get("/", deliveryHandler.List)
	delivery.Get("/ready", deliveryHandler.ListReady)
	delivery.Get("/:id", deliveryHandler.Detail)
	delivery.Get("/order/:order_id", deliveryHandler.GetByOrder)
	delivery.Post("/", middleware.RoleGuard("SUPERADMIN", "ADMIN"), deliveryHandler.Create)

	// ============================================
	// Client Routes (Public with Token)
	// ============================================
	clientHandler := handlers.NewClientHandler()
	client := v1.Group("/client")

	// Invoice
	client.Get("/invoice/:token", clientHandler.GetInvoice)

	// Payment
	client.Post("/payment/:token", clientHandler.UploadPayment)

	// Driver
	client.Post("/driver/:token", clientHandler.SubmitDriver)
	client.Post("/driver/:token/photo", clientHandler.UploadVehiclePhoto)

	// Queue
	client.Get("/queue/:token", clientHandler.GetQueueStatus)

	// Delivery
	client.Get("/delivery/:token", clientHandler.GetDeliveryNote)

	// Order Status (for polling)
	client.Get("/status/:token", clientHandler.GetOrderStatus)

	// Client Settings (public)
	client.Get("/settings", settingsHandler.GetPublic)

	// ============================================
	// Notification Routes (Protected)
	// ============================================
	notificationHandler := handlers.NewNotificationHandler()
	notifications := v1.Group("/notifications", middleware.AuthGuard())
	notifications.Get("/", notificationHandler.List)
	notifications.Get("/pending", notificationHandler.GetPending)
	notifications.Get("/stats", notificationHandler.GetStats)
	notifications.Post("/:id/sent", middleware.RoleGuard("SUPERADMIN", "ADMIN"), notificationHandler.MarkAsSent)
	notifications.Post("/send", middleware.RoleGuard("SUPERADMIN", "ADMIN"), notificationHandler.SendManual)

	// ============================================
	// Settings Routes (Protected)
	// ============================================
	settings := v1.Group("/settings", middleware.AuthGuard())
	settings.Get("/", settingsHandler.Get)
	settings.Put("/", middleware.RoleGuard("SUPERADMIN", "ADMIN"), settingsHandler.Update)

	// ============================================
	// WhatsApp Routes (Protected)
	// ============================================
	whatsAppHandler := handlers.NewWhatsAppHandler()
	whatsapp := v1.Group("/whatsapp", middleware.AuthGuard())
	whatsapp.Get("/status", whatsAppHandler.GetStatus)
	whatsapp.Post("/connect", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.Connect)
	whatsapp.Post("/disconnect", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.Disconnect)
	whatsapp.Post("/logout", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.Logout)
	whatsapp.Post("/restart", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.Restart)
	whatsapp.Post("/send", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.SendMessage)
	whatsapp.Post("/test", middleware.RoleGuard("SUPERADMIN", "ADMIN"), whatsAppHandler.SendTestMessage)
}
