package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ============================================
// MongoDB Models
// ============================================

// BaseModel for MongoDB documents
type BaseModel struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

// Image represents an image with CDN info
type Image struct {
	PublicID string `json:"public_id" bson:"public_id"`
	URL      string `json:"url" bson:"url"`
}

// User model (MongoDB)
type User struct {
	BaseModel   `bson:",inline"`
	Username    string `json:"username" bson:"username"`
	DisplayName string `json:"display_name" bson:"display_name"`
	Password    string `json:"-" bson:"password"`
	Role        string `json:"role" bson:"role"`
	Email       string `json:"email" bson:"email,omitempty"`
	IsActive    bool   `json:"is_active" bson:"is_active"`
}

// NewUser creates a new User instance (MongoDB)
func NewUser() *User {
	return &User{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		IsActive: true,
	}
}

// ============================================
// Sales Model
// ============================================

// Sales model for sales representatives
type Sales struct {
	BaseModel `bson:",inline"`
	Name      string `json:"name" bson:"name"`
	Phone     string `json:"phone" bson:"phone"`
	Email     string `json:"email,omitempty" bson:"email,omitempty"`
	Address   string `json:"address,omitempty" bson:"address,omitempty"`
	IsActive  bool   `json:"is_active" bson:"is_active"`
}

// NewSales creates a new Sales instance
func NewSales() *Sales {
	return &Sales{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		IsActive: true,
	}
}

// ============================================
// Product Model (DEPRECATED - kept for backward compatibility)
// ============================================

// Product model for products (DEPRECATED - products now input manually in order)
type Product struct {
	BaseModel   `bson:",inline"`
	Name        string  `json:"name" bson:"name"`
	Description string  `json:"description,omitempty" bson:"description,omitempty"`
	Price       float64 `json:"price" bson:"price"`
	Unit        string  `json:"unit" bson:"unit"`
	Stock       int     `json:"stock" bson:"stock"`
	Image       *Image  `json:"image,omitempty" bson:"image,omitempty"`
	IsActive    bool    `json:"is_active" bson:"is_active"`
}

// NewProduct creates a new Product instance
func NewProduct() *Product {
	return &Product{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		IsActive: true,
	}
}

// ============================================
// Order Model
// ============================================

// OrderItem represents a single item in an order
// Products are now entered manually (no longer linked to Product master)
type OrderItem struct {
	// Product info (entered manually)
	ProductName string `json:"product_name" bson:"product_name"`
	UnitPrice   float64 `json:"unit_price" bson:"unit_price"`
	Quantity    int     `json:"quantity" bson:"quantity"`
	Unit        string  `json:"unit" bson:"unit"` // Default: "pcs"
	Subtotal    float64 `json:"subtotal" bson:"subtotal"`

	// Legacy fields for backward compatibility
	ProductID string   `json:"product_id" bson:"product_id"`
	Product   *Product `json:"product,omitempty" bson:"product,omitempty"`
}

// Order model for customer orders
type Order struct {
	BaseModel `bson:",inline"`

	// Basic Info
	OrderNumber string `json:"order_number" bson:"order_number"`

	// Sales Info
	SalesID string `json:"sales_id" bson:"sales_id"`
	Sales   *Sales  `json:"sales,omitempty" bson:"sales,omitempty"`

	// Order Items (multiple products - entered manually)
	Items []OrderItem `json:"items" bson:"items"`

	// Legacy fields for backwards compatibility
	ProductID  string   `json:"product_id" bson:"product_id"`
	Product    *Product `json:"product,omitempty" bson:"product,omitempty"`
	Quantity   int      `json:"quantity" bson:"quantity"`
	UnitPrice  float64  `json:"unit_price" bson:"unit_price"`
	TotalPrice float64  `json:"total_price" bson:"total_price"`

	// Status
	Status string `json:"status" bson:"status"`

	// Client Access
	InvoiceToken string `json:"invoice_token" bson:"invoice_token"`
	InvoiceURL   string `json:"invoice_url" bson:"invoice_url"`

	// Payment Info
	PaymentProof       *Image     `json:"payment_proof,omitempty" bson:"payment_proof,omitempty"`
	PaymentStatus      string     `json:"payment_status" bson:"payment_status"`
	PaymentUploadedAt  *time.Time `json:"payment_uploaded_at,omitempty" bson:"payment_uploaded_at,omitempty"`
	PaymentVerifiedAt  *time.Time `json:"payment_verified_at,omitempty" bson:"payment_verified_at,omitempty"`
	PaymentVerifiedBy  string     `json:"payment_verified_by,omitempty" bson:"payment_verified_by,omitempty"`
	PaymentRejectedAt  *time.Time `json:"payment_rejected_at,omitempty" bson:"payment_rejected_at,omitempty"`
	PaymentRejectedBy  string     `json:"payment_rejected_by,omitempty" bson:"payment_rejected_by,omitempty"`
	PaymentRejectReason string    `json:"payment_reject_reason,omitempty" bson:"payment_reject_reason,omitempty"`

	// Driver Info
	DriverName     string     `json:"driver_name,omitempty" bson:"driver_name,omitempty"`
	DriverPhone    string     `json:"driver_phone,omitempty" bson:"driver_phone,omitempty"`
	VehiclePlate   string     `json:"vehicle_plate,omitempty" bson:"vehicle_plate,omitempty"`
	VehiclePhoto   *Image     `json:"vehicle_photo,omitempty" bson:"vehicle_photo,omitempty"`
	DriverFilledAt *time.Time `json:"driver_filled_at,omitempty" bson:"driver_filled_at,omitempty"`

	// Queue Info
	QueueNumber    int        `json:"queue_number,omitempty" bson:"queue_number,omitempty"`
	QueueToken     string     `json:"queue_token,omitempty" bson:"queue_token,omitempty"`
	QueueBarcode   string     `json:"queue_barcode,omitempty" bson:"queue_barcode,omitempty"`
	QueueQRCode    string     `json:"queue_qrcode,omitempty" bson:"queue_qrcode,omitempty"` // Base64 QR code image
	QueueEnteredAt *time.Time `json:"queue_entered_at,omitempty" bson:"queue_entered_at,omitempty"`
	EstimatedTime  string     `json:"estimated_time,omitempty" bson:"estimated_time,omitempty"`
	QueueCalledAt  *time.Time `json:"queue_called_at,omitempty" bson:"queue_called_at,omitempty"`

	// Loading Info
	LoadingStartedAt  *time.Time `json:"loading_started_at,omitempty" bson:"loading_started_at,omitempty"`
	LoadingFinishedAt *time.Time `json:"loading_finished_at,omitempty" bson:"loading_finished_at,omitempty"`

	// Delivery Info
	DeliveryNoteID  string     `json:"delivery_note_id,omitempty" bson:"delivery_note_id,omitempty"`
	DeliveryNoteURL string     `json:"delivery_note_url,omitempty" bson:"delivery_note_url,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// NewOrder creates a new Order instance
func NewOrder() *Order {
	return &Order{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Status:       OrderStatusPending,
		PaymentStatus: PaymentStatusPending,
	}
}

// ============================================
// DeliveryNote Model
// ============================================

// DeliveryNote model for delivery notes
type DeliveryNote struct {
	BaseModel `bson:",inline"`

	// Reference
	OrderID string  `json:"order_id" bson:"order_id"`
	Order   *Order  `json:"order,omitempty" bson:"order,omitempty"`

	// Note Info
	NoteNumber string `json:"note_number" bson:"note_number"`

	// Snapshot Data (for historical purposes)
	SalesName    string `json:"sales_name" bson:"sales_name"`
	SalesPhone   string `json:"sales_phone" bson:"sales_phone"`
	ProductName  string `json:"product_name" bson:"product_name"`
	ProductQty   int    `json:"product_qty" bson:"product_qty"`
	ProductUnit  string `json:"product_unit" bson:"product_unit"`
	DriverName   string `json:"driver_name" bson:"driver_name"`
	DriverPhone  string `json:"driver_phone" bson:"driver_phone"`
	VehiclePlate string `json:"vehicle_plate" bson:"vehicle_plate"`

	// Access Token
	Token string `json:"token" bson:"token"`

	// Created By
	CreatedBy string `json:"created_by" bson:"created_by"`
}

// NewDeliveryNote creates a new DeliveryNote instance
func NewDeliveryNote() *DeliveryNote {
	return &DeliveryNote{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// ============================================
// Company Settings Model
// ============================================

// CompanySettings holds company information and bank details
type CompanySettings struct {
	BaseModel `bson:",inline"`

	// Company Info
	Name    string `json:"name" bson:"name"`
	Address string `json:"address" bson:"address"`
	Phone   string `json:"phone" bson:"phone"`
	Email   string `json:"email" bson:"email"`

	// Bank Accounts
	BankName    string `json:"bank_name" bson:"bank_name"`
	BankAccount string `json:"bank_account" bson:"bank_account"`
	BankHolder  string `json:"bank_holder" bson:"bank_holder"`

	// Secondary Bank (optional)
	BankName2    string `json:"bank_name_2,omitempty" bson:"bank_name_2,omitempty"`
	BankAccount2 string `json:"bank_account_2,omitempty" bson:"bank_account_2,omitempty"`
	BankHolder2  string `json:"bank_holder_2,omitempty" bson:"bank_holder_2,omitempty"`

	// WhatsApp Number for notifications
	WhatsAppNumber string `json:"whatsapp_number" bson:"whatsapp_number"`
}

// NewCompanySettings creates a new CompanySettings instance
func NewCompanySettings() *CompanySettings {
	return &CompanySettings{
		BaseModel: BaseModel{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// ============================================
// Constants
// ============================================

// UserRole constants
const (
	RoleSuperAdmin = "SUPERADMIN"
	RoleAdmin      = "ADMIN"
	RoleUser       = "USER"
)

// Order Status constants
const (
	OrderStatusPending   = "pending"   // Order created, waiting for payment
	OrderStatusPaid      = "paid"      // Payment uploaded
	OrderStatusConfirmed = "confirmed" // Payment verified
	OrderStatusQueued    = "queued"    // Driver data filled, in queue
	OrderStatusLoading   = "loading"   // Being loaded
	OrderStatusCompleted = "completed" // Delivery note created
	OrderStatusCancelled = "cancelled" // Order cancelled
)

// Payment Status constants
const (
	PaymentStatusPending  = "pending"
	PaymentStatusVerified = "verified"
	PaymentStatusRejected = "rejected"
)

// Queue Duration (30 minutes per queue)
const QueueDurationMinutes = 30
