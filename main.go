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

	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/denisenkom/go-mssqldb" // MSSQL driver
	_ "github.com/go-sql-driver/mysql"   // MySQL driver
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2" // Oracle driver
)

//go:embed openapi.yaml
var openAPISpec []byte // This is a compiler directive, not a function call

func main() {

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

	// Setup handlers with database
	templatesHandler := handler.NewTemplatesHandler(db)
	connectionHandler := handler.NewConnectionHandler(db)
	noCodeGenerator := handler.NewNoCodeGenerator(db)
	handler.InitCategoryHandler(db)

	// Setup Gin router
	r := gin.Default()

	// Apply CORS middleware
	r.Use(gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}))

	// Setup routes
	r.GET("/health", reportService.HealthCheckHandler)
	r.POST("/api/generate-pdf", reportService.GenerateReportHandler)

	// Connection endpoints
	r.POST("/api/connections", connectionHandler.CreateConnection)
	r.GET("/api/connections", connectionHandler.GetConnections)
	r.DELETE("/api/connections/:connection_id", connectionHandler.DeleteConnection)

	// NoCode Generator endpoint
	r.GET("/api/tables/:connection_id", noCodeGenerator.GetTables)
	r.GET("/api/columns/:connection_id/:schema_name/:table_name", noCodeGenerator.GetColumnsPublic)
	//r.GET("/api/columns/:connection_id/:schema_name/:table_name", noCodeGenerator.GetAllColumns)
	r.POST("/api/generate-report", noCodeGenerator.ReportPreview)

	// Category Management Routes
	r.GET("/api/categories", handler.GetCategoryReports)
	r.GET("/api/categories/:category_name", handler.GetCategoryReportsByName)
	r.POST("/api/categories", handler.CreateCategory)
	r.DELETE("/api/categories/:category_name/reports/:report_id", handler.DeleteCategoryReport)
	r.PUT("/api/categories/:category_name/reports/:report_id/update-dates", handler.UpdateReportWithDates)
	r.POST("/api/categories/:category_name/reports/:report_id/schedule", handler.ScheduleReport)

	// Category Definition Routes
	r.GET("/api/category-definitions", handler.GetCategoryDefinitions)
	r.DELETE("/api/categories/definitions/:category_id", handler.DeleteCategory)

	// Templates Management Routes
	r.GET("/api/templates", templatesHandler.GetTemplates)
	r.POST("/api/templates", templatesHandler.CreateTemplate)
	r.GET("/api/templates/:id", templatesHandler.GetTemplate)
	r.PUT("/api/templates/:id", templatesHandler.UpdateTemplate)
	r.DELETE("/api/templates/:id", templatesHandler.DeleteTemplate)

	// Static reports routes
	//r.GET("/static-reports/direct-download", staticReports.directDownloadHandler)

	// Serve OpenAPI spec
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Header("Content-Type", "application/yaml")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Data(http.StatusOK, "application/yaml", openAPISpec)
	})

	// Serve Swagger UI
	r.GET("/docs/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/openapi.yaml"),
	)))

	log.Println("Combined services starting on :8000")
	log.Println("Available endpoints:")
	log.Println("  POST /api/v1/generate-report - Generate reports (raw SQL)")
	log.Println("  POST /api/generate-report - Generate reports (nocode table selector)")
	log.Println("  GET  /health - Health check")
	log.Println("  POST /api/connections - Create database connection")
	log.Println("  GET  /api/connections - Get all connections")
	log.Println("  DELETE /api/connections/:connection_id - Delete connection")
	log.Println("  GET  /api/tables/:connection_id - Get tables for connection")
	log.Println("  GET  /api/columns/public/:table_name - Get columns for table in public schema")
	log.Println("  GET /api/categories - Get all category reports")
	log.Println("  GET /api/categories/:category_name - Get reports by category")
	log.Println("  POST /api/categories - Create new category")
	log.Println("  DELETE /api/categories/:category_name/reports/:report_id - Delete category report")
	log.Println("  PUT /api/categories/:category_name/reports/:report_id/update-dates - Update report dates")
	log.Println("  POST /api/categories/:category_name/reports/:report_id/schedule - Schedule report")
	log.Println("  GET /api/category-definitions - Get all category definitions")
	log.Println("  DELETE /api/categories/definitions/:category_id - Delete category definition")
	log.Println("  GET /api/templates - Get all report templates")
	log.Println("  POST /api/templates - Create a new report template")
	log.Println("  GET /api/templates/:id - Get specific template by ID")
	log.Println("  PUT /api/templates/:id - Update an existing template")
	log.Println("  DELETE /api/templates/:id - Delete a template")
	log.Println("  GET  /docs/ - Interactive API documentation (Swagger UI)")
	log.Println("  GET  /openapi.yaml - OpenAPI 3.x specification")

	if err := r.Run(":8000"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
