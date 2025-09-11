package handler

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// Database connection
var db *sql.DB

// ReportTemplate represents the database model
type ReportTemplate struct {
	ID                  uuid.UUID `json:"id" db:"id"`
	Name                string    `json:"name" db:"name"`
	Logo                *string   `json:"logo" db:"logo"`
	LogoPosition        string    `json:"logo_position" db:"logo_position"`
	ReportTitle         *string   `json:"report_title" db:"report_title"`
	ReportTitlePosition string    `json:"report_title_position" db:"report_title_position"`
	ContactEnabled      bool      `json:"contact_enabled" db:"contact_enabled"`
	ContactName         *string   `json:"contact_name" db:"contact_name"`
	ContactAddress      *string   `json:"contact_address" db:"contact_address"`
	ContactEmail        *string   `json:"contact_email" db:"contact_email"`
	ContactPhone        *string   `json:"contact_phone" db:"contact_phone"`
	ContactPosition     string    `json:"contact_position" db:"contact_position"`
	ContactTextColor    string    `json:"contact_text_color" db:"contact_text_color"`
	FooterEnabled       bool      `json:"footer_enabled" db:"footer_enabled"`
	FooterText          *string   `json:"footer_text" db:"footer_text"`
	FooterPosition      string    `json:"footer_position" db:"footer_position"`
	TableHeadBgColor    string    `json:"table_head_bg_color" db:"table_head_bg_color"`
	TableHeadTextColor  string    `json:"table_head_text_color" db:"table_head_text_color"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// Request/Response models
type ReportTemplateCreate struct {
	Name                string         `json:"name"`
	Logo                *string        `json:"logo,omitempty"`
	LogoPosition        string         `json:"logoPosition"`
	ReportTitle         *string        `json:"reportTitle,omitempty"`
	ReportTitlePosition string         `json:"reportTitlePosition"`
	ContactDetails      ContactDetails `json:"contactDetails"`
	Footer              Footer         `json:"footer"`
	TableHeadBgColor    string         `json:"tableHeadBgColor"`
	TableHeadTextColor  string         `json:"tableHeadTextColor"`
}

// Frontend-compatible request model (for CUSTOM initialization)
type FrontendTemplateCreate struct {
	Name         string         `json:"name"`
	ConnectionID string         `json:"connection_id,omitempty"`
	SchemaName   string         `json:"schema_name,omitempty"`
	TableName    string         `json:"table_name,omitempty"`
	CategoryName string         `json:"category_name,omitempty"`
	Logo         *string        `json:"logo,omitempty"`
	LogoPosition string         `json:"logoPosition,omitempty"`
	Contact      ContactDetails `json:"contact,omitempty"`    // Frontend uses "contact" instead of "contactDetails"
	TheadColor   string         `json:"theadColor,omitempty"` // Frontend uses "theadColor" instead of "tableHeadBgColor"

	// Optional fields that might be missing from frontend
	ContactDetails     ContactDetails `json:"contactDetails,omitempty"`
	Footer             Footer         `json:"footer,omitempty"`
	TableHeadBgColor   string         `json:"tableHeadBgColor,omitempty"`
	TableHeadTextColor string         `json:"tableHeadTextColor,omitempty"`
	ReportTitle        *string        `json:"reportTitle,omitempty"`
	ReportTitlePos     string         `json:"reportTitlePosition,omitempty"`
}

type ContactDetails struct {
	Enabled   bool    `json:"enabled"`
	Name      *string `json:"name,omitempty"`
	Address   *string `json:"address,omitempty"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Position  string  `json:"position"`
	TextColor string  `json:"textColor"`
}

type Footer struct {
	Enabled  bool    `json:"enabled"`
	Text     *string `json:"text,omitempty"`
	Position string  `json:"position"`
}

type ReportTemplateResponse struct {
	ID                  uuid.UUID      `json:"id"`
	Name                string         `json:"name"`
	Logo                *string        `json:"logo"`
	LogoPosition        string         `json:"logoPosition"`
	ReportTitle         *string        `json:"reportTitle"`
	ReportTitlePosition string         `json:"reportTitlePosition"`
	ContactDetails      ContactDetails `json:"contactDetails"`
	Footer              Footer         `json:"footer"`
	TableHeadBgColor    string         `json:"tableHeadBgColor"`
	TableHeadTextColor  string         `json:"tableHeadTextColor"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

// Database configuration
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Initialize database connection
func initDB() error {
	config := DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: "RahulM6?",
		DBName:   getEnv("DB_NAME", "sql-executor"),
	}

	// Decode password if it's base64 encoded
	if strings.Contains(config.Password, "$2") {
		// This is a hashed password, use it directly
	} else if decoded, err := base64.StdEncoding.DecodeString(config.Password); err == nil {
		config.Password = string(decoded)
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	fmt.Println("✅ Successfully connected to PostgreSQL database")

	return nil
}

// Helper function to get environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Extract UUID from URL path
func extractUUIDFromPath(c *gin.Context) (uuid.UUID, error) {
	id := c.Param("id")
	return uuid.Parse(id)
}

// Create a new report template
func createTemplate(c *gin.Context) {
	var req ReportTemplateCreate
	var frontendReq FrontendTemplateCreate

	// Read the request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Try Python/standard format first (matches user's frontend code)
	if err := json.Unmarshal(bodyBytes, &req); err == nil && req.Name != "" {
		// Successfully parsed Python format - use as-is
		log.Println("✅ Parsed Python-style request format")
	} else {
		// Fall back to frontend format
		if err := json.Unmarshal(bodyBytes, &frontendReq); err == nil && frontendReq.Name != "" {
			log.Println("✅ Parsed frontend-style request format")
			// Convert frontend format to standard format
			req = ReportTemplateCreate{
				Name:                frontendReq.Name,
				Logo:                frontendReq.Logo,
				LogoPosition:        frontendReq.LogoPosition,
				ReportTitle:         frontendReq.ReportTitle,
				ReportTitlePosition: frontendReq.ReportTitlePos,
				ContactDetails:      frontendReq.Contact, // Use frontend's "contact" field
				Footer:              frontendReq.Footer,
				TableHeadBgColor:    frontendReq.TheadColor, // Use frontend's "theadColor" field
				TableHeadTextColor:  frontendReq.TableHeadTextColor,
			}
		} else {
			c.JSON(400, gin.H{"error": "Invalid JSON payload - unable to parse either Python or frontend format"})
			return
		}
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "Template name is required"})
		return
	}

	// Set defaults for missing fields
	if req.LogoPosition == "" {
		req.LogoPosition = "left"
	}
	if req.ContactDetails.Position == "" {
		req.ContactDetails.Position = "right"
	}
	if req.Footer.Position == "" {
		req.Footer.Position = "center"
	}
	if req.TableHeadBgColor == "" {
		req.TableHeadBgColor = "#f3f4f6"
	}
	if req.TableHeadTextColor == "" {
		req.TableHeadTextColor = "#000000"
	}

	template := ReportTemplate{
		ID:                  uuid.New(),
		Name:                req.Name,
		Logo:                req.Logo,
		LogoPosition:        req.LogoPosition,
		ReportTitle:         req.ReportTitle,
		ReportTitlePosition: req.ReportTitlePosition,
		ContactEnabled:      req.ContactDetails.Enabled,
		ContactName:         req.ContactDetails.Name,
		ContactAddress:      req.ContactDetails.Address,
		ContactEmail:        req.ContactDetails.Email,
		ContactPhone:        req.ContactDetails.Phone,
		ContactPosition:     req.ContactDetails.Position,
		ContactTextColor:    req.ContactDetails.TextColor,
		FooterEnabled:       req.Footer.Enabled,
		FooterText:          req.Footer.Text,
		FooterPosition:      req.Footer.Position,
		TableHeadBgColor:    req.TableHeadBgColor,
		TableHeadTextColor:  req.TableHeadTextColor,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	query := `
		INSERT INTO user_connection.report_template (
			id, name, logo, logo_position, report_title, report_title_position,
			contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			table_head_bg_color, table_head_text_color, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)`

	_, err = db.Exec(query,
		template.ID, template.Name, template.Logo, template.LogoPosition, template.ReportTitle,
		template.ReportTitlePosition, template.ContactEnabled, template.ContactName,
		template.ContactAddress, template.ContactEmail, template.ContactPhone,
		template.ContactPosition, template.ContactTextColor, template.FooterEnabled,
		template.FooterText, template.FooterPosition, template.TableHeadBgColor,
		template.TableHeadTextColor, template.CreatedAt, template.UpdatedAt)

	if err != nil {
		log.Printf("Error creating template: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create template"})
		return
	}

	response := ReportTemplateResponse{
		ID:                  template.ID,
		Name:                template.Name,
		Logo:                template.Logo,
		LogoPosition:        template.LogoPosition,
		ReportTitle:         template.ReportTitle,
		ReportTitlePosition: template.ReportTitlePosition,
		ContactDetails:      req.ContactDetails,
		Footer:              req.Footer,
		TableHeadBgColor:    template.TableHeadBgColor,
		TableHeadTextColor:  template.TableHeadTextColor,
		CreatedAt:           template.CreatedAt,
		UpdatedAt:           template.UpdatedAt,
	}

	// Return frontend-compatible response structure
	frontendResponse := gin.H{
		"success": true,
		"id":      response.ID.String(),
		"message": "Template saved successfully!",
	}
	c.JSON(201, frontendResponse)
}

// Get all report templates
func getTemplates(c *gin.Context) {
	query := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template
		ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error querying templates: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch templates"})
		return
	}
	defer rows.Close()

	var templates []ReportTemplateResponse

	for rows.Next() {
		var template ReportTemplate
		err := rows.Scan(
			&template.ID, &template.Name, &template.Logo, &template.LogoPosition,
			&template.ReportTitle, &template.ReportTitlePosition, &template.ContactEnabled,
			&template.ContactName, &template.ContactAddress, &template.ContactEmail,
			&template.ContactPhone, &template.ContactPosition, &template.ContactTextColor,
			&template.FooterEnabled, &template.FooterText, &template.FooterPosition,
			&template.TableHeadBgColor, &template.TableHeadTextColor,
			&template.CreatedAt, &template.UpdatedAt)

		if err != nil {
			log.Printf("Error scanning template: %v", err)
			continue
		}

		response := ReportTemplateResponse{
			ID:                  template.ID,
			Name:                template.Name,
			Logo:                template.Logo,
			LogoPosition:        template.LogoPosition,
			ReportTitle:         template.ReportTitle,
			ReportTitlePosition: template.ReportTitlePosition,
			ContactDetails: ContactDetails{
				Enabled:   template.ContactEnabled,
				Name:      template.ContactName,
				Address:   template.ContactAddress,
				Email:     template.ContactEmail,
				Phone:     template.ContactPhone,
				Position:  template.ContactPosition,
				TextColor: template.ContactTextColor,
			},
			Footer: Footer{
				Enabled:  template.FooterEnabled,
				Text:     template.FooterText,
				Position: template.FooterPosition,
			},
			TableHeadBgColor:   template.TableHeadBgColor,
			TableHeadTextColor: template.TableHeadTextColor,
			CreatedAt:          template.CreatedAt,
			UpdatedAt:          template.UpdatedAt,
		}

		templates = append(templates, response)
	}

	c.JSON(200, templates)
}

// Get a specific template by ID
func getTemplate(c *gin.Context) {
	templateID, err := extractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	var template ReportTemplate
	query := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template
		WHERE id = $1`

	err = db.QueryRow(query, templateID).Scan(
		&template.ID, &template.Name, &template.Logo, &template.LogoPosition,
		&template.ReportTitle, &template.ReportTitlePosition, &template.ContactEnabled,
		&template.ContactName, &template.ContactAddress, &template.ContactEmail,
		&template.ContactPhone, &template.ContactPosition, &template.ContactTextColor,
		&template.FooterEnabled, &template.FooterText, &template.FooterPosition,
		&template.TableHeadBgColor, &template.TableHeadTextColor,
		&template.CreatedAt, &template.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "Template not found"})
		return
	}
	if err != nil {
		log.Printf("Error fetching template: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch template"})
		return
	}

	response := ReportTemplateResponse{
		ID:                  template.ID,
		Name:                template.Name,
		Logo:                template.Logo,
		LogoPosition:        template.LogoPosition,
		ReportTitle:         template.ReportTitle,
		ReportTitlePosition: template.ReportTitlePosition,
		ContactDetails: ContactDetails{
			Enabled:   template.ContactEnabled,
			Name:      template.ContactName,
			Address:   template.ContactAddress,
			Email:     template.ContactEmail,
			Phone:     template.ContactPhone,
			Position:  template.ContactPosition,
			TextColor: template.ContactTextColor,
		},
		Footer: Footer{
			Enabled:  template.FooterEnabled,
			Text:     template.FooterText,
			Position: template.FooterPosition,
		},
		TableHeadBgColor:   template.TableHeadBgColor,
		TableHeadTextColor: template.TableHeadTextColor,
		CreatedAt:          template.CreatedAt,
		UpdatedAt:          template.UpdatedAt,
	}

	c.JSON(200, response)
}

// Update an existing template
func updateTemplate(c *gin.Context) {
	templateID, err := extractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	var req ReportTemplateCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "Template name is required"})
		return
	}

	query := `
		UPDATE user_connection.report_template SET
			name = $1, logo = $2, logo_position = $3, report_title = $4,
			report_title_position = $5, contact_enabled = $6, contact_name = $7,
			contact_address = $8, contact_email = $9, contact_phone = $10,
			contact_position = $11, contact_text_color = $12, footer_enabled = $13,
			footer_text = $14, footer_position = $15, table_head_bg_color = $16,
			table_head_text_color = $17, updated_at = $18
		WHERE id = $19`

	result, err := db.Exec(query,
		req.Name, req.Logo, req.LogoPosition, req.ReportTitle, req.ReportTitlePosition,
		req.ContactDetails.Enabled, req.ContactDetails.Name, req.ContactDetails.Address,
		req.ContactDetails.Email, req.ContactDetails.Phone, req.ContactDetails.Position,
		req.ContactDetails.TextColor, req.Footer.Enabled, req.Footer.Text,
		req.Footer.Position, req.TableHeadBgColor, req.TableHeadTextColor, time.Now(), templateID)

	if err != nil {
		log.Printf("Error updating template: %v", err)
		c.JSON(500, gin.H{"error": "Failed to update template"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		c.JSON(500, gin.H{"error": "Failed to update template"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Template not found"})
		return
	}

	// Fetch updated template
	var template ReportTemplate
	selectQuery := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template WHERE id = $1`

	err = db.QueryRow(selectQuery, templateID).Scan(
		&template.ID, &template.Name, &template.Logo, &template.LogoPosition,
		&template.ReportTitle, &template.ReportTitlePosition, &template.ContactEnabled,
		&template.ContactName, &template.ContactAddress, &template.ContactEmail,
		&template.ContactPhone, &template.ContactPosition, &template.ContactTextColor,
		&template.FooterEnabled, &template.FooterText, &template.FooterPosition,
		&template.TableHeadBgColor, &template.TableHeadTextColor,
		&template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		log.Printf("Error fetching updated template: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch updated template"})
		return
	}

	response := ReportTemplateResponse{
		ID:                  template.ID,
		Name:                template.Name,
		Logo:                template.Logo,
		LogoPosition:        template.LogoPosition,
		ReportTitle:         template.ReportTitle,
		ReportTitlePosition: template.ReportTitlePosition,
		ContactDetails: ContactDetails{
			Enabled:   template.ContactEnabled,
			Name:      template.ContactName,
			Address:   template.ContactAddress,
			Email:     template.ContactEmail,
			Phone:     template.ContactPhone,
			Position:  template.ContactPosition,
			TextColor: template.ContactTextColor,
		},
		Footer: Footer{
			Enabled:  template.FooterEnabled,
			Text:     template.FooterText,
			Position: template.FooterPosition,
		},
		TableHeadBgColor:   template.TableHeadBgColor,
		TableHeadTextColor: template.TableHeadTextColor,
		CreatedAt:          template.CreatedAt,
		UpdatedAt:          template.UpdatedAt,
	}

	c.JSON(200, response)
}

// Delete a template
func deleteTemplate(c *gin.Context) {
	templateID, err := extractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	result, err := db.Exec("DELETE FROM user_connection.report_template WHERE id = $1", templateID)
	if err != nil {
		log.Printf("Error deleting template: %v", err)
		c.JSON(500, gin.H{"error": "Failed to delete template"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		c.JSON(500, gin.H{"error": "Failed to delete template"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Template deleted successfully"})
}

// func main() {
// 	// Initialize database connection
// 	if err := initDB(); err != nil {
// 		log.Fatal("Failed to initialize database:", err)
// 	}
// 	defer db.Close()

// 	// Use default Gin configuration with CORS
// 	r := gin.Default()

// 	// Add CORS policy for the default router
// 	r.Use(func(c *gin.Context) {
// 		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
// 		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
// 		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Accept-Encoding")
// 		c.Header("Access-Control-Allow-Credentials", "true")

// 		if c.Request.Method == "OPTIONS" {
// 			c.AbortWithStatus(204)
// 			return
// 		}

// 		c.Next()
// 	})

// 	// First add a simple health check to test routing
// 	r.GET("/health", func(c *gin.Context) {
// 		c.JSON(200, gin.H{"status": "ok", "message": "Server is running"})
// 	})

// 	// Define API routes
// 	r.GET("/api/templates", getTemplates)
// 	r.POST("/api/templates", createTemplate)
// 	r.GET("/api/templates/:id", getTemplate)
// 	r.PUT("/api/templates/:id", updateTemplate)
// 	r.DELETE("/api/templates/:id", deleteTemplate)

// 	// Get port from environment or use default
// 	port := os.Getenv("PORT")
// 	if port == "" {
// 		port = "8000"
// 	}

// 	fmt.Printf("🚀 Template Handler Server starting on port %s\n", port)
// 	fmt.Println("📋 Available endpoints:")
// 	fmt.Println("  GET   /health (health check)")
// 	fmt.Println("  GET    /api/templates")
// 	fmt.Println("  POST   /api/templates")
// 	fmt.Println("  GET    /api/templates/{id}")
// 	fmt.Println("  PUT    /api/templates/{id}")
// 	fmt.Println("  DELETE /api/templates/{id}")

// 	log.Fatal(r.Run(":" + port))
// }
