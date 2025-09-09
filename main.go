package main

import (
	"database/sql"
	_ "embed"
	"fmt"
	"go-sql-executor/handler"
	"go-sql-executor/models"
	"log"
	"net/http"
	"os"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/denisenkom/go-mssqldb" // MSSQL driver
	_ "github.com/go-sql-driver/mysql"   // MySQL driver
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed openapi.yaml
var openAPISpec []byte // This is a compiler directive, not a function call

func main() {

	startConnectionService()

	generatePDF := true
	exportCSV := true
	exportExcel := true
	generateHTML := true

	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, relying on OS environment.")
	}

	// LOAD CONFIGURATION from environment variables
	selectedDB := os.Getenv("DB_DRIVER")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	sslmode := os.Getenv("SSL_MODE")

	// Database configuration
	dbConfigs := map[string]models.DBConfig{
		"postgres": {
			DriverName: "postgres",
			DataSourceName: fmt.Sprintf("host=%s port=%s "+
				"user=%s password=%s "+
				"dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPassword, dbName, sslmode),
		},
		"mysql": {
			DriverName:     "mysql",
			DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb",
		},
	}

	config, exists := dbConfigs[selectedDB]
	if !exists {
		log.Fatalf("Database '%s' not configured", selectedDB)
	}

	// Database Connection
	dsn := config.DataSourceName
	db, err := sql.Open(selectedDB, dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Pinging the database to verify the connection is alive/established
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	fmt.Printf("Successfully connected to %s! database\n", selectedDB)

	// Initialize service
	reportService := handler.NewReportService(db, generateHTML, generatePDF, exportCSV, exportExcel)

	// Setup routes
	http.HandleFunc("/health", reportService.HealthCheckHandler)
	http.HandleFunc("/api/v1/generate-report", reportService.GenerateReportHandler)

	// Serve OpenAPI spec
	http.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(openAPISpec)
	})

	// Serve Swagger UI
	http.HandleFunc("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/openapi.yaml"),
	))

	// Configure server
	server := &http.Server{
		Addr:         ":8081",
		Handler:      nil,               // Use default ServeMux
		ReadTimeout:  30 * time.Second,  // Prevent slow client attacks
		WriteTimeout: 300 * time.Second, // Allow time for large file generation
		IdleTimeout:  60 * time.Second,
		//	MaxHeaderBytes: 1 << 20, // 1MB max header size
	}

	log.Println("Report generation service starting on :8081")
	log.Println("Available endpoints:")
	log.Println("  POST /api/v1/generate-report - Generate reports")
	log.Println("  GET  /health - Health check")
	log.Println("  GET  /docs/ - Interactive API documentation (Swagger UI)")
	log.Println("  GET  /openapi.yaml - OpenAPI 3.x specification")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}

func startConnectionService() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on OS environment.")
	}

	// LOAD CONFIGURATION from environment variables
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	sslmode := os.Getenv("SSL_MODE")

	// Get database URL from environment
	databaseURL := fmt.Sprintf("host=%s port=%s "+
		"user=%s password=%s "+
		"dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPassword, dbName, sslmode)
	if databaseURL == "" {
		log.Fatal("DATABASE ENV not properly configured.")
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize schemas and tables
	if err := handler.InitializeSchemas(db); err != nil {
		log.Fatalf("Failed to initialize schemas: %v", err)
	}

	// Create connection handler
	connectionHandler := handler.NewConnectionHandler(db)

	// Setup routes with Go 1.22+ enhanced ServeMux
	mux := http.NewServeMux()

	// Apply CORS middleware
	handler := handler.CorsMiddleware(mux)

	// Connection endpoints with method and path parameter support
	mux.HandleFunc("POST /api/connections", connectionHandler.CreateConnection)
	mux.HandleFunc("GET /api/connections", connectionHandler.GetConnections)
	//mux.HandleFunc("GET /api/connections/{connection_id}", connectionHandler.GetConnection)
	mux.HandleFunc("DELETE /api/connections/{connection_id}", connectionHandler.DeleteConnection)

	// Start server
	port := os.Getenv("CONNECTION_SERVICE_PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Connection handler service starting on port %s", port)
	log.Println("Available endpoints:")
	log.Println("  POST /api/connections - Create database connection")
	log.Println("  GET  /api/connections - Get all connections")
	log.Println("  DELETE /api/connections/{id} - Delete connection")
	log.Println("  GET  /openapi.yaml - OpenAPI 3.x specification")
	//log.Println("  GET  /api/connections/{id} - Get connection by ID")

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
