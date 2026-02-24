package handlers

import (
	"context"
	"time"

	"bg-go/internal/config"
	"bg-go/internal/database"
	"bg-go/internal/lib/crypt"
	"bg-go/internal/lib/jwt"
	"bg-go/internal/lib/response"
	"bg-go/internal/middleware"
	"bg-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AuthHandler handles auth routes
type AuthHandler struct{}

// NewAuthHandler creates a new auth handler
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

// Genesis creates the initial superadmin account
// @Summary Create genesis account
// @Description Create the initial superadmin account (one-time setup)
// @Tags Auth
// @Param password query string true "Genesis password"
// @Success 201 {object} map[string]interface{}
// @Router /auth/genesis [get]
func (h *AuthHandler) Genesis(c *fiber.Ctx) error {
	cfg := config.Cfg

	// Check genesis password
	// password := c.Query("password")
	// if password != cfg.JWT.GenesisPassword {
	// 	return response.Error(c, 401, "Invalid genesis password")
	// }

	// Check if superadmin exists
	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, _ := collection.CountDocuments(ctx, bson.M{"role": models.RoleSuperAdmin})
	if count > 0 {
		return response.Error(c, 400, "Genesis account already created")
	}

	// Create superadmin
	hashedPassword, err := crypt.HashPassword(cfg.JWT.GenesisPassword)
	if err != nil {
		return response.Error(c, 500, "Failed to hash password")
	}

	user := models.NewUser()
	user.Username = "superadmin"
	user.DisplayName = "Super Admin"
	user.Password = hashedPassword
	user.Role = models.RoleSuperAdmin

	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		return response.Error(c, 500, "Failed to create genesis account")
	}

	// Generate tokens
	tokenPair, err := jwt.GenerateTokenPair(user.ID.Hex(), user.Role)
	if err != nil {
		return response.Error(c, 500, "Failed to generate tokens")
	}

	// Return response (Express style: data at root level for auth)
	return response.SuccessWithData(c, 201, fiber.Map{
		"status":        201,
		"message":       "success",
		"id":            user.ID.Hex(),
		"username":      user.Username,
		"display_name":  user.DisplayName,
		"role":          user.Role,
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
	})
}

// Login authenticates a user
// @Summary Login
// @Description Authenticate user and get tokens
// @Tags Auth
// @Param body body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]interface{}
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	type LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Find user
	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user := &models.User{}
	err := collection.FindOne(ctx, bson.M{"username": req.Username}).Decode(user)
	if err != nil {
		return response.Error(c, 400, "Invalid credentials")
	}

	// Verify password
	if !crypt.CheckPassword(req.Password, user.Password) {
		return response.Error(c, 400, "Invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return response.Error(c, 403, "Account is deactivated")
	}

	// Generate tokens
	tokenPair, err := jwt.GenerateTokenPair(user.ID.Hex(), user.Role)
	if err != nil {
		return response.Error(c, 500, "Failed to generate tokens")
	}

	// Set refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    tokenPair.RefreshToken,
		HTTPOnly: true,
		MaxAge:   int(config.Cfg.JWT.RefreshExpiry.Seconds()),
	})

	// Return response (Express style: data at root level)
	return response.SuccessWithData(c, 200, fiber.Map{
		"status":       200,
		"message":      "success",
		"id":           user.ID.Hex(),
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"access_token": tokenPair.AccessToken,
		"expires_in":   tokenPair.ExpiresIn,
	})
}

// Me returns current user info
// @Summary Get current user
// @Description Get authenticated user information
// @Tags Auth
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return response.Error(c, 400, "Invalid user ID")
	}

	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user := &models.User{}
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(user)
	if err != nil {
		return response.Error(c, 404, "User not found")
	}

	// Return response (Express style: data at root level)
	return response.SuccessWithData(c, 200, fiber.Map{
		"status":       200,
		"message":      "success",
		"id":           user.ID.Hex(),
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"email":        user.Email,
		"is_active":    user.IsActive,
		"created_at":   user.CreatedAt,
	})
}

// Refresh refreshes access token
// @Summary Refresh token
// @Description Refresh access token using refresh token
// @Tags Auth
// @Success 200 {object} map[string]interface{}
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return response.Error(c, 401, "No refresh token")
	}

	claims, err := jwt.VerifyRefreshToken(refreshToken)
	if err != nil {
		return response.Unauthorized(c, "Invalid refresh token")
	}

	// Generate new tokens
	tokenPair, err := jwt.GenerateTokenPair(claims.UserID, claims.Role)
	if err != nil {
		return response.Error(c, 500, "Failed to generate tokens")
	}

	// Set new refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    tokenPair.RefreshToken,
		HTTPOnly: true,
		MaxAge:   int(config.Cfg.JWT.RefreshExpiry.Seconds()),
	})

	return response.SuccessWithData(c, 200, fiber.Map{
		"status":       200,
		"message":      "success",
		"access_token": tokenPair.AccessToken,
		"expires_in":   tokenPair.ExpiresIn,
	})
}

// ListUsers lists all users (admin only)
// @Summary List users
// @Description Get all users
// @Tags Auth
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param search query string false "Search query"
// @Success 200 {object} map[string]interface{}
// @Router /auth/users [get]
func (h *AuthHandler) ListUsers(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	search := c.Query("search")

	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter = bson.M{
			"$or": []bson.M{
				{"username": bson.M{"$regex": search, "$options": "i"}},
				{"display_name": bson.M{"$regex": search, "$options": "i"}},
			},
		}
	}

	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get total count
	total, _ := collection.CountDocuments(ctx, filter)

	// Get users
	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return response.Error(c, 500, "Failed to fetch users")
	}
	defer cursor.Close(ctx)

	var users []models.User
	cursor.All(ctx, &users)

	return response.SuccessWithPagination(c, 200, users, response.CalculatePagination(int64(page), int64(limit), total))
}

// Register creates a new user (admin only)
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	type RegisterRequest struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Email       string `json:"email,omitempty"`
		Role        string `json:"role,omitempty"`
	}

	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate
	if req.Username == "" || req.Password == "" {
		return response.BadRequest(c, "Username and password required")
	}

	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if username exists
	count, _ := collection.CountDocuments(ctx, bson.M{"username": req.Username})
	if count > 0 {
		return response.Error(c, 400, "Username already exists")
	}

	// Hash password
	hashedPassword, err := crypt.HashPassword(req.Password)
	if err != nil {
		return response.Error(c, 500, "Failed to hash password")
	}

	// Create user
	user := models.NewUser()
	user.Username = req.Username
	user.DisplayName = req.DisplayName
	user.Password = hashedPassword
	user.Email = req.Email
	if req.Role != "" {
		user.Role = req.Role
	} else {
		user.Role = models.RoleUser
	}

	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		return response.Error(c, 500, "Failed to create user")
	}

	return response.Success(c, 201, fiber.Map{
		"id":           user.ID.Hex(),
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
	})
}

// UpdateUser updates a user (admin only)
func (h *AuthHandler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	type UpdateRequest struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password,omitempty"`
		Email       string `json:"email,omitempty"`
		Role        string `json:"role,omitempty"`
		IsActive    *bool  `json:"is_active,omitempty"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build update
	update := bson.M{"updated_at": time.Now()}
	if req.Username != "" {
		// Check if username exists for another user
		count, _ := collection.CountDocuments(ctx, bson.M{
			"username": req.Username,
			"_id":      bson.M{"$ne": objID},
		})
		if count > 0 {
			return response.Error(c, 400, "Username already exists")
		}
		update["username"] = req.Username
	}
	if req.DisplayName != "" {
		update["display_name"] = req.DisplayName
	}
	if req.Password != "" {
		hashedPassword, err := crypt.HashPassword(req.Password)
		if err != nil {
			return response.Error(c, 500, "Failed to hash password")
		}
		update["password"] = hashedPassword
	}
	if req.Email != "" {
		update["email"] = req.Email
	}
	if req.Role != "" {
		update["role"] = req.Role
	}
	if req.IsActive != nil {
		update["is_active"] = *req.IsActive
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		return response.Error(c, 500, "Failed to update user")
	}
	if result.MatchedCount == 0 {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, 200, fiber.Map{
		"message": "User updated successfully",
	})
}

// DeleteUser deletes a user (admin only)
func (h *AuthHandler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	collection := database.GetMongoCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if user exists and is not superadmin
	var user models.User
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	if user.Role == models.RoleSuperAdmin {
		return response.Error(c, 400, "Cannot delete superadmin")
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return response.Error(c, 500, "Failed to delete user")
	}
	if result.DeletedCount == 0 {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, 200, fiber.Map{
		"message": "User deleted successfully",
	})
}
