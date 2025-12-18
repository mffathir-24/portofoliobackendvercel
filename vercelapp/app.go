package vercelapp

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gintugas/database"
	routers "gintugas/modules"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/lib/pq"
)

type GinApp struct {
	Router *gin.Engine
	DB     *sql.DB
	GormDB *gorm.DB
}

// InitializeApp membuat instance Gin untuk Vercel
func InitializeApp() *GinApp {
	fmt.Println("🚀 Initializing Gintugas API for Vercel...")

	// Load env jika ada
	if os.Getenv("GIN_MODE") != "release" {
		godotenv.Load("config/.env")
	}

	app := &GinApp{}
	app.setupDatabase()
	app.setupRouter()

	return app
}

func (app *GinApp) setupDatabase() {
	dbURL := getDatabaseURL()

	if os.Getenv("SKIP_DATABASE") == "true" {
		fmt.Println("⚠️ Skipping database connection")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("⚠️ Database connection failed: %v\n", err)
		return
	}

	// Test connection
	err = db.PingContext(ctx)
	if err != nil {
		fmt.Printf("⚠️ Database ping failed: %v\n", err)
		return
	}

	fmt.Println("✅ Database connected")

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Setup GORM
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		fmt.Printf("⚠️ GORM setup failed: %v\n", err)
		app.DB = db
		return
	}

	app.DB = db
	app.GormDB = gormDB

	// Run migrations jika diperlukan
	if os.Getenv("RUN_MIGRATIONS") == "true" && db != nil {
		fmt.Println("🗄️ Running database migrations...")
		err := database.DBMigrate(db)
		if err != nil {
			fmt.Printf("⚠️ Migrations failed: %v\n", err)
		} else {
			fmt.Println("✅ Migrations completed")
		}
	}
}

func (app *GinApp) setupRouter() {
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		dbStatus := "not_connected"
		if app.DB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			if err := app.DB.PingContext(ctx); err == nil {
				dbStatus = "connected"
			} else {
				dbStatus = "disconnected"
			}
		}

		c.JSON(200, gin.H{
			"status":    "ok",
			"service":   "gintugas-api",
			"timestamp": time.Now().Unix(),
			"database":  dbStatus,
			"platform":  "vercel",
		})
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Gintugas API on Vercel",
			"version": "1.0",
			"endpoints": []string{
				"/health",
				"/api/v1/projects",
				"/api/v1/tasks",
			},
		})
	})

	// API routes dari modules kamu
	if app.DB != nil {
		routers.Initiator(router, app.DB, app.GormDB)
	}

	// Fallback untuk 404
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error":   "Endpoint not found",
			"path":    c.Request.URL.Path,
			"message": "Check / for available endpoints",
		})
	})

	app.Router = router
}

func (app *GinApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Gin memerlukan body yang bisa dibaca multiple times
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Handle request dengan Gin
	app.Router.ServeHTTP(w, r)
}

func getDatabaseURL() string {
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

	// Fallback ke hardcoded URL (untuk development saja)
	return "postgresql://postgres:Bg3644aa%40%23@db.yiujndqqbacipqozosdm.supabase.co:5432/postgres?sslmode=require"
}
