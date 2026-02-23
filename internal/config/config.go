package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	CDN      CDNConfig
	Upload   UploadConfig
	CORS     CORSConfig
	Cron     CronConfig
	Client   ClientConfig
	WhatsApp WhatsAppConfig
}

type AppConfig struct {
	Name string
	Env  string
	Port string
}

type DatabaseConfig struct {
	Driver string

	// MongoDB
	MongoURL string

	// PostgreSQL
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	// MySQL
	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string
}

type JWTConfig struct {
	AccessSecret   string
	RefreshSecret  string
	AccessExpiry   time.Duration
	RefreshExpiry  time.Duration
	GenesisPassword string
}

type CDNConfig struct {
	CloudName string
	APIKey    string
	APISecret string
	Folder    string
}

type UploadConfig struct {
	MaxFileSize       int64
	AllowedFileTypes  []string
}

type CORSConfig struct {
	AllowedOrigins string
	AllowedMethods string
	AllowedHeaders string
}

type CronConfig struct {
	Enabled bool
}

type ClientConfig struct {
	URL string
}

type WhatsAppConfig struct {
	SessionPath string
}

// Cfg holds the global configuration
var Cfg *Config

// Load loads configuration from environment variables
func Load() *Config {
	// Render sets PORT env var, use it as priority
	port := getEnv("PORT", "")
	if port == "" {
		port = getEnv("APP_PORT", "8000")
	}

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "BG-API"),
			Env:  getEnv("APP_ENV", "development"),
			Port: port,
		},
		Database: DatabaseConfig{
			Driver:           getEnv("DB_DRIVER", "mongodb"),
			MongoURL:         getEnv("MONGO_URL", "mongodb://localhost:27017/bgdb"),
			PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
			PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
			PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
			PostgresPassword: getEnv("POSTGRES_PASSWORD", "postgres"),
			PostgresDB:       getEnv("POSTGRES_DB", "bgdb"),
			MySQLHost:        getEnv("MYSQL_HOST", "localhost"),
			MySQLPort:        getEnv("MYSQL_PORT", "3306"),
			MySQLUser:        getEnv("MYSQL_USER", "root"),
			MySQLPassword:    getEnv("MYSQL_PASSWORD", "root"),
			MySQLDatabase:    getEnv("MYSQL_DATABASE", "bgdb"),
		},
		JWT: JWTConfig{
			AccessSecret:    getEnv("JWT_SECRET", "secret"),
			RefreshSecret:   getEnv("JWT_REFRESH_SECRET", "refresh-secret"),
			AccessExpiry:    getDurationEnv("JWT_ACCESS_EXPIRY", 24*time.Hour),
			RefreshExpiry:   getDurationEnv("JWT_REFRESH_EXPIRY", 168*time.Hour),
			GenesisPassword: getEnv("GENESIS_PASSWORD", ""),
		},
		CDN: CDNConfig{
			CloudName: getEnv("CDN_CLOUD_NAME", ""),
			APIKey:    getEnv("CDN_API_KEY", ""),
			APISecret: getEnv("CDN_API_SECRET", ""),
			Folder:    getEnv("CDN_FOLDER", "bg-uploads"),
		},
		Upload: UploadConfig{
			MaxFileSize:      getInt64Env("MAX_FILE_SIZE", 52428800),
			AllowedFileTypes: getSliceEnv("ALLOWED_FILE_TYPES", []string{"jpg", "jpeg", "png", "gif", "webp", "pdf"}),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
			AllowedMethods: getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
			AllowedHeaders: getEnv("CORS_ALLOWED_HEADERS", "Origin,Content-Type,Accept,Authorization"),
		},
		Cron: CronConfig{
			Enabled: getBoolEnv("CRON_ENABLED", false),
		},
		Client: ClientConfig{
			URL: getEnv("CLIENT_URL", "http://localhost:3001"),
		},
		WhatsApp: WhatsAppConfig{
			SessionPath: getEnv("WHATSAPP_SESSION_PATH", "./whatsapp-session"),
		},
	}

	Cfg = cfg
	return cfg
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		result := []string{}
		current := ""
		for _, char := range value {
			if char == ',' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(char)
			}
		}
		if current != "" {
			result = append(result, current)
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// IsDevelopment checks if app is running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}
