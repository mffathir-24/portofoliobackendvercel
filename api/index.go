package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"gintugas/database"
	routers "gintugas/modules"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	router    *gin.Engine
	db        *sql.DB
	gormDB    *gorm.DB
	once      sync.Once
	initError error
)

// Handler adalah entry point untuk Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	// Initialize router hanya sekali
	once.Do(func() {
		initError = initializeApp()
	})

	if initError != nil {
		http.Error(w, fmt.Sprintf("Initialization error: %v", initError), http.StatusInternalServerError)
		return
	}

	// Serve request dengan Gin router
	router.ServeHTTP(w, r)
}

func initializeApp() error {
	fmt.Println("🚀 Initializing Gintugas API for Vercel...")

	// Setup Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Setup database
	var dbConnected bool
	db, gormDB, dbConnected = setupDatabase()

	// Run migrations jika database connected
	if dbConnected && db != nil {
		fmt.Println("🗄️ Running database migrations...")
		if err := database.DBMigrate(db); err != nil {
			fmt.Printf("⚠️ Database migrations failed: %v\n", err)
		} else {
			fmt.Println("✅ Database migrations completed")
		}
	}

	// Setup router
	router = gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Health check
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Gintugas API on Vercel",
			"version": "1.0",
			"status":  "running",
		})
	})

	router.GET("/api/health", func(c *gin.Context) {
		dbStatus := "not_configured"
		if db != nil {
			if err := db.Ping(); err == nil {
				dbStatus = "connected"
			} else {
				dbStatus = "disconnected"
			}
		}

		c.JSON(200, gin.H{
			"status":          "ok",
			"service":         "gintugas-api",
			"database":        dbStatus,
			"upload_provider": os.Getenv("UPLOAD_PROVIDER"),
			"environment":     "vercel",
		})
	})

	// Initialize all routes
	routers.Initiator(router, db, gormDB)

	fmt.Println("✅ Gintugas API initialized successfully")
	return nil
}

func setupDatabase() (*sql.DB, *gorm.DB, bool) {
	dbURL := getDatabaseURL()

	if dbURL == "" {
		fmt.Println("⚠️ No database URL configured")
		return nil, nil, false
	}

	fmt.Println("🔌 Connecting to database...")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("⚠️ Database connection failed: %v\n", err)
		return nil, nil, false
	}

	// Test connection
	if err := db.Ping(); err != nil {
		fmt.Printf("⚠️ Database ping failed: %v\n", err)
		db.Close()
		return nil, nil, false
	}

	fmt.Println("✅ Database connected")

	// Configure connection pool for serverless
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)

	// Setup GORM
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		fmt.Printf("⚠️ GORM setup failed: %v\n", err)
		return db, nil, true
	}

	fmt.Println("✅ GORM initialized")
	return db, gormDB, true
}

func getDatabaseURL() string {
	// Priority 1: DATABASE_URL from environment
	if url := os.Getenv("DATABASE_URL"); url != "" {
		url = strings.ReplaceAll(url, "@#", "%40%23")
		if !strings.Contains(url, "sslmode=") {
			if strings.Contains(url, "?") {
				url += "&sslmode=require"
			} else {
				url += "?sslmode=require"
			}
		}
		return url
	}

	// Priority 2: Build from separate parameters
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	if host != "" && user != "" && password != "" && dbname != "" {
		if port == "" {
			port = "5432"
		}
		return fmt.Sprintf(
			"postgresql://%s:%s@%s:%s/%s?sslmode=require&connect_timeout=10",
			user, password, host, port, dbname,
		)
	}

	return ""
}
