package config

import (
	"fmt"
	"log"

	"bubblewhite-backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the shared GORM handle used across repositories.
var DB *gorm.DB

// ConnectDatabase opens the MySQL connection described by the loaded Config.
func ConnectDatabase(cfg *Config) *gorm.DB {
	// Log exactly what we're about to connect to (never the password) —
	// on a hosting platform like Railway, "connection refused to
	// 127.0.0.1" almost always means DB_HOST never actually got set
	// (falling back to the default) rather than a real network problem.
	// This makes that immediately visible in the deploy logs instead of
	// having to guess whether an env var reference actually resolved.
	log.Printf("config: connecting to MySQL as %s@%s:%s/%s", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	logLevel := logger.Warn
	if cfg.AppEnv == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("config: failed to connect to database: %v", err)
	}

	DB = db
	log.Println("config: connected to MySQL database:", cfg.DBName)
	return db
}

// AutoMigrate creates/updates every table this app owns. Called once at boot.
func AutoMigrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.Permission{},
		&models.Role{},
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ContactMessage{},
		&models.Settings{},
		&models.Banner{},
		&models.Customer{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.PPCBankToken{},
		&models.PaymentMethod{},
		&models.OtpRequest{},
		&models.TelegramPhoneLink{},
		&models.AuditLog{},
	)
	if err != nil {
		log.Fatalf("config: migration failed: %v", err)
	}

	relaxCustomerColumns(db)

	log.Println("config: database migrated")
}

// relaxCustomerColumns drops the NOT NULL constraint on customers.phone and
// customers.password_hash for databases that already existed before
// Google/Facebook sign-in was added.
//
// GORM's AutoMigrate deliberately never loosens an existing column's
// constraints on its own (only adds missing tables/columns/indexes) — a
// brand-new database gets these columns created as nullable directly from
// the Customer struct's tags above, but an already-deployed database still
// has the original NOT NULL from before Phone/PasswordHash became
// pointers. Without this, the very first Google/Facebook signup on an
// existing deployment would fail at the database level with a NOT NULL
// constraint violation, even though the Go code now treats both as
// optional.
//
// Safe to run on every boot — MODIFY COLUMN re-applying the same
// definition is a harmless no-op once already applied.
func relaxCustomerColumns(db *gorm.DB) {
	if err := db.Exec("ALTER TABLE customers MODIFY COLUMN phone VARCHAR(50) NULL").Error; err != nil {
		log.Printf("config: could not relax customers.phone to nullable (safe to ignore on a brand-new database): %v", err)
	}
	if err := db.Exec("ALTER TABLE customers MODIFY COLUMN password_hash VARCHAR(255) NULL").Error; err != nil {
		log.Printf("config: could not relax customers.password_hash to nullable (safe to ignore on a brand-new database): %v", err)
	}
}
