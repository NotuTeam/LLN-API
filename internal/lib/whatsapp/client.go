package whatsapp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"bg-go/internal/config"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Client represents the WhatsApp client
type Client struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	device    *store.Device
	db        *sql.DB

	// QR code for pairing
	qrCode     string
	qrCodeData string // Base64 image

	// Connection state
	connected bool
	loggedIn  bool

	// Error state
	lastError string

	// Context for operations
	ctx    context.Context
	cancel context.CancelFunc

	// Mutex for thread safety
	mu sync.RWMutex
}

// WhatsApp client singleton
var WhatsApp *Client

// Init initializes the WhatsApp client
func Init() error {
	log.Printf("[WhatsApp] Starting initialization...")

	sessionPath := config.Cfg.WhatsApp.SessionPath
	if sessionPath == "" {
		sessionPath = "./whatsapp-session"
	}

	// Create session directory if not exists
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		log.Printf("[WhatsApp] Failed to create session directory: %v", err)
		return fmt.Errorf("failed to create session directory: %v", err)
	}
	log.Printf("[WhatsApp] Session directory: %s", sessionPath)

	// Database path - using modernc.org/sqlite (pure Go, no CGO)
	dbPath := fmt.Sprintf("%s/whatsapp.db", sessionPath)
	log.Printf("[WhatsApp] Database path: %s", dbPath)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Open database connection with modernc.org/sqlite
	// Driver name is "sqlite" (not "sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		cancel()
		log.Printf("[WhatsApp] Failed to open database: %v", err)
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Enable foreign keys for modernc.org/sqlite
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		cancel()
		log.Printf("[WhatsApp] Failed to enable foreign keys: %v", err)
		return fmt.Errorf("failed to enable foreign keys: %v", err)
	}
	log.Printf("[WhatsApp] Foreign keys enabled")

	// Test database connection
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		cancel()
		log.Printf("[WhatsApp] Failed to ping database: %v", err)
		return fmt.Errorf("failed to ping database: %v", err)
	}
	log.Printf("[WhatsApp] Database connection established")

	// Create SQL store using existing DB connection
	dbLog := waLog.Stdout("Database", "WARN", false)
	container := sqlstore.NewWithDB(db, "sqlite", dbLog)
	err = container.Upgrade(ctx)
	if err != nil {
		db.Close()
		cancel()
		log.Printf("[WhatsApp] Failed to upgrade container: %v", err)
		return fmt.Errorf("failed to upgrade container: %v", err)
	}
	log.Printf("[WhatsApp] SQL store created and upgraded")

	// Get or create device
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		db.Close()
		cancel()
		log.Printf("[WhatsApp] Failed to get device: %v", err)
		return fmt.Errorf("failed to get device: %v", err)
	}
	log.Printf("[WhatsApp] Device obtained")

	// Create client
	clientLog := waLog.Stdout("Client", "WARN", false)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	WhatsApp = &Client{
		container: container,
		device:    deviceStore,
		client:    client,
		db:        db,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Set up event handlers
	WhatsApp.setupEventHandlers()

	log.Printf("[WhatsApp] Initialization complete")
	return nil
}

// setupEventHandlers sets up all event handlers
func (c *Client) setupEventHandlers() {
	c.client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			c.handleConnected()
		case *events.Disconnected:
			c.handleDisconnected()
		case *events.LoggedOut:
			c.handleLoggedOut()
		case *events.PairSuccess:
			log.Printf("[WhatsApp] Pair success: %v", v.ID)
			c.handleConnected()
		case *events.PairError:
			log.Printf("[WhatsApp] Pair error: %v", v.Error)
			c.mu.Lock()
			c.lastError = fmt.Sprintf("Pair error: %v", v.Error)
			c.mu.Unlock()
		case *events.StreamError:
			log.Printf("[WhatsApp] Stream error: %v", v)
			c.mu.Lock()
			c.lastError = fmt.Sprintf("Stream error: %v", v)
			c.mu.Unlock()
		}
	})
}

// handleConnected handles connected event
func (c *Client) handleConnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	c.loggedIn = true
	c.qrCode = ""
	c.qrCodeData = ""
	c.lastError = ""
	log.Printf("[WhatsApp] Connected successfully!")
}

// handleDisconnected handles disconnected event
func (c *Client) handleDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	log.Printf("[WhatsApp] Disconnected")
}

// handleLoggedOut handles logged out event
func (c *Client) handleLoggedOut() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loggedIn = false
	c.connected = false
	log.Printf("[WhatsApp] Logged out")
}

// Connect starts the connection process
func (c *Client) Connect() error {
	log.Printf("[WhatsApp] Attempting to connect...")

	c.mu.Lock()
	c.lastError = ""
	c.qrCode = ""
	c.qrCodeData = ""
	c.mu.Unlock()

	// Check if already logged in
	if c.client.Store.ID == nil {
		// No ID stored, new login - need QR code
		log.Printf("[WhatsApp] No stored session, requesting QR code...")
		
		qrChan, err := c.client.GetQRChannel(c.ctx)
		if err != nil {
			// If error, it might be because we're already connected or connecting
			log.Printf("[WhatsApp] GetQRChannel error: %v, trying direct connect", err)
		} else {
			// Start goroutine to receive QR codes
			go func() {
				for evt := range qrChan {
					log.Printf("[WhatsApp] QR event: %s", evt.Event)
					
					if evt.Event == "code" {
						// New QR code received
						c.mu.Lock()
						c.qrCode = evt.Code
						
						// Generate QR code image
						png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
						if err == nil {
							c.qrCodeData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
							log.Printf("[WhatsApp] QR code image generated")
						}
						c.mu.Unlock()
					} else {
						log.Printf("[WhatsApp] QR event: %s", evt.Event)
					}
				}
				log.Printf("[WhatsApp] QR channel closed")
			}()
		}
	}

	// Connect
	err := c.client.Connect()
	if err != nil {
		log.Printf("[WhatsApp] Failed to connect: %v", err)
		c.mu.Lock()
		c.lastError = fmt.Sprintf("Connect error: %v", err)
		c.mu.Unlock()
		return err
	}

	log.Printf("[WhatsApp] Connect called successfully")
	return nil
}

// Disconnect disconnects the client
func (c *Client) Disconnect() {
	log.Printf("[WhatsApp] Disconnecting...")
	c.client.Disconnect()
}

// GetQRCode returns the current QR code
func (c *Client) GetQRCode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.qrCode
}

// GetQRCodeData returns the QR code as base64 image
func (c *Client) GetQRCodeData() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.qrCodeData
}

// IsConnected returns if client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// IsLoggedIn returns if client is logged in
func (c *Client) IsLoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loggedIn
}

// GetLastError returns the last error
func (c *Client) GetLastError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// GetStatus returns the current status
func (c *Client) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if has stored session
	hasSession := c.client.Store.ID != nil

	return map[string]interface{}{
		"connected":      c.connected,
		"logged_in":      c.loggedIn,
		"qr_code":        c.qrCode,
		"qr_code_image":  c.qrCodeData,
		"last_error":     c.lastError,
		"has_session":    hasSession,
	}
}

// parsePhoneToJID converts phone number to WhatsApp JID
func parsePhoneToJID(phone string) (types.JID, error) {
	// Clean phone number - keep only digits
	clean := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			clean += string(c)
		}
	}

	// Handle Indonesian numbers
	if len(clean) > 0 && clean[0] == '0' {
		clean = "62" + clean[1:]
	}

	if len(clean) < 10 || len(clean) > 15 {
		return types.JID{}, fmt.Errorf("invalid phone number length")
	}

	// Return JID
	return types.JID{
		Server: types.DefaultUserServer,
		User:   clean,
	}, nil
}

// SendMessage sends a text message to a phone number
func (c *Client) SendMessage(phone string, message string) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected")
	}

	// Parse phone to JID
	jid, err := parsePhoneToJID(phone)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	// Create text message
	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	// Send message
	_, err = c.client.SendMessage(ctx, jid, msg)
	if err != nil {
		log.Printf("[WhatsApp] Failed to send message: %v", err)
		return err
	}

	log.Printf("[WhatsApp] Message sent to %s", phone)
	return nil
}

// Close closes the database connection (call on shutdown)
func (c *Client) Close() {
	log.Printf("[WhatsApp] Closing...")
	c.client.Disconnect()
	if c.db != nil {
		c.db.Close()
	}
	if c.cancel != nil {
		c.cancel()
	}
}

// Restart restarts the connection
func (c *Client) Restart() error {
	log.Printf("[WhatsApp] Restarting...")
	c.Disconnect()
	time.Sleep(1 * time.Second)
	return c.Connect()
}

// Logout logs out and clears session
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Printf("[WhatsApp] Logging out...")

	if c.client != nil {
		err := c.client.Logout(c.ctx)
		if err != nil {
			log.Printf("[WhatsApp] Logout error: %v", err)
		}
	}

	c.loggedIn = false
	c.connected = false
	c.qrCode = ""
	c.qrCodeData = ""
	c.lastError = ""

	log.Printf("[WhatsApp] Logged out")
	return nil
}
