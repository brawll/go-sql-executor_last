package handler

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"time"

	"go-sql-executor/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TemplatesHandler struct to hold database dependency
type TemplatesHandler struct {
	db *sql.DB
}

// New templates handler returns handler instance with database
func NewTemplatesHandler(database *sql.DB) *TemplatesHandler {
	return &TemplatesHandler{
		db: database,
	}
}

// Extract UUID from URL path
func ExtractUUIDFromPath(c *gin.Context) (uuid.UUID, error) {
	id := c.Param("id")
	return uuid.Parse(id)
}

// Create a new report template
func (h *TemplatesHandler) CreateTemplate(c *gin.Context) {
	var req models.ReportTemplateCreate
	var frontendReq models.FrontendTemplateCreate

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
			req = models.ReportTemplateCreate{
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

	template := models.ReportTemplate{
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

	_, err = h.db.Exec(query,
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

	response := models.ReportTemplateResponse{
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
func (h *TemplatesHandler) GetTemplates(c *gin.Context) {
	query := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template
		ORDER BY created_at DESC`

	rows, err := h.db.Query(query)
	if err != nil {
		log.Printf("Error querying templates: %v", err)
		c.JSON(500, gin.H{"error": "Failed to fetch templates"})
		return
	}
	defer rows.Close()

	var templates []models.ReportTemplateResponse

	for rows.Next() {
		var template models.ReportTemplate
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

		response := models.ReportTemplateResponse{
			ID:                  template.ID,
			Name:                template.Name,
			Logo:                template.Logo,
			LogoPosition:        template.LogoPosition,
			ReportTitle:         template.ReportTitle,
			ReportTitlePosition: template.ReportTitlePosition,
			ContactDetails: models.ContactDetails{
				Enabled:   template.ContactEnabled,
				Name:      template.ContactName,
				Address:   template.ContactAddress,
				Email:     template.ContactEmail,
				Phone:     template.ContactPhone,
				Position:  template.ContactPosition,
				TextColor: template.ContactTextColor,
			},
			Footer: models.Footer{
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
func (h *TemplatesHandler) GetTemplate(c *gin.Context) {
	templateID, err := ExtractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	var template models.ReportTemplate
	query := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template
		WHERE id = $1`

	err = h.db.QueryRow(query, templateID).Scan(
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

	response := models.ReportTemplateResponse{
		ID:                  template.ID,
		Name:                template.Name,
		Logo:                template.Logo,
		LogoPosition:        template.LogoPosition,
		ReportTitle:         template.ReportTitle,
		ReportTitlePosition: template.ReportTitlePosition,
		ContactDetails: models.ContactDetails{
			Enabled:   template.ContactEnabled,
			Name:      template.ContactName,
			Address:   template.ContactAddress,
			Email:     template.ContactEmail,
			Phone:     template.ContactPhone,
			Position:  template.ContactPosition,
			TextColor: template.ContactTextColor,
		},
		Footer: models.Footer{
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
func (h *TemplatesHandler) UpdateTemplate(c *gin.Context) {
	templateID, err := ExtractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	var req models.ReportTemplateCreate
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

	result, err := h.db.Exec(query,
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
	var template models.ReportTemplate
	selectQuery := `
		SELECT id, name, logo, logo_position, report_title, report_title_position,
			   contact_enabled, contact_name, contact_address, contact_email, contact_phone,
			   contact_position, contact_text_color, footer_enabled, footer_text, footer_position,
			   table_head_bg_color, table_head_text_color, created_at, updated_at
		FROM user_connection.report_template WHERE id = $1`

	err = h.db.QueryRow(selectQuery, templateID).Scan(
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

	response := models.ReportTemplateResponse{
		ID:                  template.ID,
		Name:                template.Name,
		Logo:                template.Logo,
		LogoPosition:        template.LogoPosition,
		ReportTitle:         template.ReportTitle,
		ReportTitlePosition: template.ReportTitlePosition,
		ContactDetails: models.ContactDetails{
			Enabled:   template.ContactEnabled,
			Name:      template.ContactName,
			Address:   template.ContactAddress,
			Email:     template.ContactEmail,
			Phone:     template.ContactPhone,
			Position:  template.ContactPosition,
			TextColor: template.ContactTextColor,
		},
		Footer: models.Footer{
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
func (h *TemplatesHandler) DeleteTemplate(c *gin.Context) {
	templateID, err := ExtractUUIDFromPath(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid template ID"})
		return
	}

	result, err := h.db.Exec("DELETE FROM user_connection.report_template WHERE id = $1", templateID)
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
