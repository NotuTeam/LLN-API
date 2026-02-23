package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"bg-go/internal/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB holds database connections
type DB struct {
	Mongo   *mongo.Client
	MongoDB *mongo.Database
	Gorm    *gorm.DB
}

// Database connection instance
var DBInstance *DB

// Connect establishes database connection based on driver
func Connect(cfg *config.DatabaseConfig) (*DB, error) {
	db := &DB{}

	switch cfg.Driver {
	case "mongodb":
		if err := db.connectMongo(cfg); err != nil {
			return nil, err
		}
	case "postgres":
		if err := db.connectPostgres(cfg); err != nil {
			return nil, err
		}
	case "mysql":
		if err := db.connectMySQL(cfg); err != nil {
			return nil, err
		}
	case "sqlite":
		if err := db.connectSQLite(cfg); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	DBInstance = db
	return db, nil
}

// connectMongo connects to MongoDB
func (db *DB) connectMongo(cfg *config.DatabaseConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(cfg.MongoURL).
		SetMaxPoolSize(10).
		SetMinPoolSize(2).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	db.Mongo = client
	db.MongoDB = client.Database("LabaLaba")

	log.Println("✓ Connected to MongoDB")
	return nil
}

// connectPostgres connects to PostgreSQL
func (db *DB) connectPostgres(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresDB,
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	db.Gorm = gormDB
	log.Println("✓ Connected to PostgreSQL")
	return nil
}

// connectMySQL connects to MySQL
func (db *DB) connectMySQL(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.MySQLUser,
		cfg.MySQLPassword,
		cfg.MySQLHost,
		cfg.MySQLPort,
		cfg.MySQLDatabase,
	)

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	db.Gorm = gormDB
	log.Println("✓ Connected to MySQL")
	return nil
}

// connectSQLite connects to SQLite
func (db *DB) connectSQLite(cfg *config.DatabaseConfig) error {
	gormDB, err := gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite: %v", err)
	}

	db.Gorm = gormDB
	log.Println("✓ Connected to SQLite")
	return nil
}

// Close closes all database connections
func (db *DB) Close() error {
	if db.Mongo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.Mongo.Disconnect(ctx)
	}
	return nil
}

// GetMongoCollection returns a MongoDB collection
func GetMongoCollection(name string) *mongo.Collection {
	if DBInstance == nil || DBInstance.MongoDB == nil {
		return nil
	}
	return DBInstance.MongoDB.Collection(name)
}

// GetGormDB returns Gorm DB instance
func GetGormDB() *gorm.DB {
	if DBInstance == nil {
		return nil
	}
	return DBInstance.Gorm
}

// AutoMigrate runs auto migration for GORM models
func AutoMigrate(models ...interface{}) error {
	if DBInstance == nil || DBInstance.Gorm == nil {
		return fmt.Errorf("GORM database not connected")
	}
	return DBInstance.Gorm.AutoMigrate(models...)
}
