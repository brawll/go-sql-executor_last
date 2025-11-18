package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-sql-executor/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Global variables
var categoryDB *sql.DB
var categoryReportsDir string

func InitCategoryHandler(db *sql.DB) {
	// Initialize main database connection for categories
	categoryDB = db
	// Setup reports directory for category reports
	categoryReportsDir = filepath.Join(".", "reports", "categories")
	os.MkdirAll(categoryReportsDir, 0755)
}

// Database helper functions
func validateDateFormat(dateStr string) bool {
	if dateStr == "" {
		return true // Empty dates are valid
	}

	patterns := []string{
		`^\d{4}-\d{2}-\d{2}$`,                   // YYYY-MM-DD
		`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, // YYYY-MM-DD HH:MM:SS
		`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`,       // YYYY-MM-DD HH:MM
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, dateStr)
		if matched {
			return true
		}
	}
	return false
}

// getBaseUrl constructs the base URL from request
func getBaseUrl(c *gin.Context) string {
	// Prefer explicit BASE_URL from environment if provided (e.g. https://reports.mycompany.com)
	if envBase := os.Getenv("BASE_URL"); envBase != "" {
		return envBase
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	host := c.Request.Host
	if host == "" {
		host = "localhost:8000" // fallback
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

// CATEGORY ENDPOINT HANDLERS

// GET /api/categories
func GetCategoryReports(c *gin.Context) {
	// Get base URL for download links
	baseUrl := getBaseUrl(c)

	// Get all reports with category information
	query := `
		SELECT
			ct.id,
			ct.category_name,
			ct.report_name,
			ct.filename,
			ct.created_at,
			ct.user_token,
			COALESCE(ct.expiry_time, NOW() + INTERVAL '10 years') as expiry_time
		FROM user_connection.category_table ct
		ORDER BY ct.created_at DESC
	`

	rows, err := categoryDB.Query(query)
	if err != nil {
		log.Printf("Error querying category reports: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	var reports []models.CategoryReportResponse

	for rows.Next() {
		var report models.CategoryReportResponse
		var token, filename string
		var expiryTime time.Time

		err := rows.Scan(
			&report.ID,
			&report.CategoryName,
			&report.ReportName,
			&filename,
			&report.CreatedAt,
			&token,
			&expiryTime,
		)
		if err != nil {
			log.Printf("Error scanning category report row: %v", err)
			continue
		}

		// Clean filename
		report.ReportFilename = filepath.Base(filename)

		// Check if token is expired and refresh if needed
		hasExpired := expiryTime.Before(time.Now())
		if hasExpired {
			token = generateSecureToken()
			newExpiry := time.Now().AddDate(10, 0, 0) // 10 years

			// Update token in database
			_, err = categoryDB.Exec(`
				UPDATE user_connection.category_table
				SET user_token = $1, expiry_time = $2
				WHERE id = $3
			`, token, newExpiry, report.ID)

			if err != nil {
				log.Printf("Error updating token: %v", err)
			}
		}

		// Generate download and preview URLs
		report.DownloadUrl = fmt.Sprintf("%s/static-reports/direct-download?token=%s", baseUrl, token)
		report.PreviewUrl = fmt.Sprintf("%s/static-reports/preview?token=%s", baseUrl, token)

		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating category report rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing results"})
		return
	}

	c.JSON(http.StatusOK, reports)
}

// GET /api/categories/{category_name}
func GetCategoryReportsByName(c *gin.Context) {
	categoryName, err := url.QueryUnescape(c.Param("category_name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}

	baseUrl := getBaseUrl(c)

	// Get reports for specific category
	query := `
		SELECT
			ct.id,
			ct.category_name,
			ct.report_name,
			ct.filename,
			ct.created_at,
			ct.user_token,
			COALESCE(ct.expiry_time, NOW() + INTERVAL '10 years') as expiry_time
		FROM user_connection.category_table ct
		WHERE ct.category_name = $1
		ORDER BY ct.created_at DESC
	`

	rows, err := categoryDB.Query(query, categoryName)
	if err != nil {
		log.Printf("Error querying category reports by name: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	reports := []models.CategoryReportResponse{}

	for rows.Next() {
		var report models.CategoryReportResponse
		var token, filename string
		var expiryTime time.Time

		err := rows.Scan(
			&report.ID,
			&report.CategoryName,
			&report.ReportName,
			&filename,
			&report.CreatedAt,
			&token,
			&expiryTime,
		)
		if err != nil {
			log.Printf("Error scanning category report row: %v", err)
			continue
		}

		// Clean filename
		report.ReportFilename = filepath.Base(filename)

		// Check if token is expired and refresh if needed
		hasExpired := expiryTime.Before(time.Now())
		if hasExpired {
			token = generateSecureToken()
			newExpiry := time.Now().AddDate(10, 0, 0) // 10 years

			// Update token in database
			_, err = categoryDB.Exec(`
				UPDATE user_connection.category_table
				SET user_token = $1, expiry_time = $2
				WHERE id = $3
			`, token, newExpiry, report.ID)

			if err != nil {
				log.Printf("Error updating token: %v", err)
			}
		}

		// Generate download and preview URLs
		report.DownloadUrl = fmt.Sprintf("%s/static-reports/direct-download?token=%s", baseUrl, token)
		report.PreviewUrl = fmt.Sprintf("%s/static-reports/preview?token=%s", baseUrl, token)

		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating category report rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing results"})
		return
	}

	// if len(reports) == 0 {
	// 	c.JSON(http.StatusOK, gin.H{"error": fmt.Sprintf("No reports found in category '%s'", categoryName)})
	// 	return
	// }

	c.JSON(http.StatusOK, reports)
}

// DELETE /api/categories/{category_name}/reports/{report_id}
func DeleteCategoryReport(c *gin.Context) {
	categoryName, err := url.QueryUnescape(c.Param("category_name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}

	reportID := c.Param("report_id")

	// Get report details before deletion
	var filename, filePath string
	err = categoryDB.QueryRow(`
		SELECT filename, file_path
		FROM user_connection.category_table
		WHERE id = $1 AND category_name = $2
	`, reportID, categoryName).Scan(&filename, &filePath)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
			return
		}
		log.Printf("Error querying report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}

	// Delete physical files if they exist
	filesToDelete := []string{filePath}
	filesToDelete = append(filesToDelete, filepath.Join(categoryReportsDir, filename))

	// Try alternate locations
	tempDir := os.TempDir()
	filesToDelete = append(filesToDelete, filepath.Join(tempDir, filename))
	filesToDelete = append(filesToDelete, filepath.Join(".", "reports", filename))

	for _, file := range filesToDelete {
		if _, err := os.Stat(file); err == nil {
			if err := os.Remove(file); err != nil {
				log.Printf("Warning: Failed to delete file %s: %v", file, err)
			}
		}
	}

	// Remove token from global token cache (if any)
	// Note: This would require access to the main application's token cache

	// Delete from category_table
	_, err = categoryDB.Exec(`
		DELETE FROM user_connection.category_table
		WHERE id = $1 AND category_name = $2
	`, reportID, categoryName)
	if err != nil {
		log.Printf("Error deleting from category_table: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete report"})
		return
	}

	// Also delete from temp_reports if it exists
	_, err = categoryDB.Exec(`
		DELETE FROM user_connection.temp_reports
		WHERE filename = $1
	`, filename)
	if err != nil {
		log.Printf("Warning: Failed to delete from temp_reports: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Report deleted successfully from category '%s'", categoryName),
	})
}

// POST /api/categories/{category_name}/reports/{report_id}/schedule
func ScheduleReport(c *gin.Context) {
	categoryName, err := url.QueryUnescape(c.Param("category_name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}

	reportID := c.Param("report_id")

	var req models.ScheduleReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate schedule request
	if err := validateScheduleRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get original report details
	var originalReport struct {
		ReportName    string
		ConnectionID  string
		Query         string
		StartDate     *string
		EndDate       *string
		QueryTemplate *string
	}

	err = categoryDB.QueryRow(`
		SELECT report_name, connection_id, query, last_start_date, last_end_date, original_query_template
		FROM user_connection.category_table
		WHERE id = $1 AND category_name = $2
	`, reportID, categoryName).Scan(
		&originalReport.ReportName,
		&originalReport.ConnectionID,
		&originalReport.Query,
		&originalReport.StartDate,
		&originalReport.EndDate,
		&originalReport.QueryTemplate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
			return
		}
		log.Printf("Error querying original report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}

	// Check if already scheduled
	var existingID string
	err = categoryDB.QueryRow(`
		SELECT id FROM user_connection.scheduled_reports
		WHERE original_report_id = $1 AND is_active = 'active'
	`, reportID).Scan(&existingID)

	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Report is already scheduled"})
		return
	}

	// Calculate next executions
	nextExecutions := calculateNextExecutions(req)

	// Create scheduled report
	scheduleID := uuid.New().String()
	token := generateSecureToken()
	expiry := time.Now().AddDate(10, 0, 0) // 10 years

	// Build scheduled times string
	scheduledTimesStr := ""
	if len(req.ScheduledTimes) > 0 {
		scheduledTimesStr = strings.Join(req.ScheduledTimes, ",")
	}

	// Insert scheduled report
	_, err = categoryDB.Exec(`
		INSERT INTO user_connection.scheduled_reports (
			id, original_report_id, original_category_name, report_name,
			filename, file_path, query, original_query_template,
			connection_id, schedule_type, scheduled_times,
			user_token, expiry_time, last_start_date, last_end_date,
			is_active, next_execution_times, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW())
	`, scheduleID, reportID, categoryName, originalReport.ReportName,
		"", "", originalReport.Query, originalReport.QueryTemplate,
		originalReport.ConnectionID, req.ScheduleType, scheduledTimesStr,
		token, expiry, originalReport.StartDate, originalReport.EndDate,
		"active", jsonArrayToString(nextExecutions))

	if err != nil {
		log.Printf("Error creating scheduled report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Report scheduled successfully",
		"scheduled_report": gin.H{
			"id":              scheduleID,
			"schedule_type":   req.ScheduleType,
			"next_executions": nextExecutions,
		},
	})
}

// POST /api/categories
func CreateCategory(c *gin.Context) {
	var req models.CategoryCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if category already exists
	var existingID string
	err := categoryDB.QueryRow(`
		SELECT id FROM user_connection.category_definitions
		WHERE category_name = $1
	`, req.CategoryName).Scan(&existingID)

	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Category '%s' already exists", req.CategoryName)})
		return
	}

	// Create new category
	categoryID := uuid.New().String()
	_, err = categoryDB.Exec(`
		INSERT INTO user_connection.category_definitions (
			id, category_name, description, created_at
		) VALUES ($1, $2, $3, NOW())
	`, categoryID, req.CategoryName, req.Description)

	if err != nil {
		log.Printf("Error creating category: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, models.CategoryResponse{
		ID:           categoryID,
		CategoryName: req.CategoryName,
		Description:  req.Description,
		CreatedAt:    time.Now(),
	})
}

// GET /api/category-definitions
func GetCategoryDefinitions(c *gin.Context) {
	query := `
		SELECT
            id,
            category_name,
            COALESCE(description, '') AS description,
            created_at
        FROM user_connection.category_definitions
        ORDER BY category_name
	`

	rows, err := categoryDB.Query(query)
	if err != nil {
		log.Printf("Error querying category definitions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	var categories []models.CategoryResponse

	for rows.Next() {
		var category models.CategoryResponse
		err := rows.Scan(
			&category.ID,
			&category.CategoryName,
			&category.Description,
			&category.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning category row: %v", err)
			continue
		}
		categories = append(categories, category)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating category rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing results"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// DELETE /api/categories/definitions/{category_id}
func DeleteCategory(c *gin.Context) {
	categoryID := c.Param("category_id")

	// Check if category exists and get details
	var categoryName string
	var reportsCount int

	err := categoryDB.QueryRow(`
		SELECT category_name FROM user_connection.category_definitions WHERE id = $1
	`, categoryID).Scan(&categoryName)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		log.Printf("Error querying category: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}

	// Check if category has reports
	err = categoryDB.QueryRow(`
		SELECT COUNT(*) FROM user_connection.category_table WHERE category_name = $1
	`, categoryName).Scan(&reportsCount)

	if err != nil {
		log.Printf("Error counting reports: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}

	if reportsCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Cannot delete category '%s' because it contains %d reports", categoryName, reportsCount),
		})
		return
	}

	// Delete category
	_, err = categoryDB.Exec(`
		DELETE FROM user_connection.category_definitions WHERE id = $1
	`, categoryID)

	if err != nil {
		log.Printf("Error deleting category: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Category '%s' deleted successfully", categoryName),
	})
}

// PUT /api/categories/{category_name}/reports/{report_id}/update-dates
func UpdateReportWithDates(c *gin.Context) {
	categoryName, err := url.QueryUnescape(c.Param("category_name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category name"})
		return
	}

	reportID := c.Param("report_id")

	var req models.ReportUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate date formats
	if !validateDateFormat(req.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}

	if !validateDateFormat(req.EndDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	// Get current report details
	var currentReport struct {
		QueryTemplate *string
		ConnectionID  string
		LastStartDate *string
		LastEndDate   *string
	}

	err = categoryDB.QueryRow(`
		SELECT original_query_template, connection_id, last_start_date, last_end_date
		FROM user_connection.category_table
		WHERE id = $1 AND category_name = $2
	`, reportID, categoryName).Scan(
		&currentReport.QueryTemplate,
		&currentReport.ConnectionID,
		&currentReport.LastStartDate,
		&currentReport.LastEndDate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
			return
		}
		log.Printf("Error querying report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query failed"})
		return
	}

	// For simplicity, this implementation provides a basic framework
	// In practice, you'd need to implement the full date filtering logic
	// similar to the Python version

	c.JSON(http.StatusOK, gin.H{
		"message":       "Report update functionality - implementation needed",
		"report_id":     reportID,
		"category_name": categoryName,
		"dates": gin.H{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})
}

// HELPER FUNCTIONS

func generateSecureToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func validateScheduleRequest(req models.ScheduleReportRequest) error {
	switch req.ScheduleType {
	case "test_minutes":
		if req.MinutesFromNow <= 0 {
			return fmt.Errorf("minutes_from_now must be greater than 0")
		}
	case "daily":
		if len(req.ScheduledTimes) == 0 {
			return fmt.Errorf("scheduled_times is required for daily schedule")
		}
		// Validate time formats
		timeRegex := regexp.MustCompile(`^\d{2}:\d{2}$`)
		for _, timeStr := range req.ScheduledTimes {
			if !timeRegex.MatchString(timeStr) {
				return fmt.Errorf("invalid time format: %s, use HH:MM", timeStr)
			}
		}
	case "weekly":
		if req.WeeklyDay == "" || req.WeeklyTime == "" {
			return fmt.Errorf("weekly_day and weekly_time are required for weekly schedule")
		}
	case "monthly":
		if req.MonthlyDay <= 0 || req.MonthlyDay > 31 || req.MonthlyTime == "" {
			return fmt.Errorf("valid monthly_day (1-31) and monthly_time are required")
		}
	case "yearly":
		if req.YearlyMonth <= 0 || req.YearlyMonth > 12 || req.YearlyDay <= 0 || req.YearlyDay > 31 || req.YearlyTime == "" {
			return fmt.Errorf("valid yearly_month (1-12), yearly_day (1-31), and yearly_time are required")
		}
	default:
		return fmt.Errorf("invalid schedule_type: %s", req.ScheduleType)
	}
	return nil
}

func calculateNextExecutions(req models.ScheduleReportRequest) []string {
	// Simplified implementation - in practice you'd need more complex logic
	// similar to the Python scheduler service

	var executions []string

	now := time.Now()
	switch req.ScheduleType {
	case "test_minutes":
		executionTime := now.Add(time.Duration(req.MinutesFromNow) * time.Minute)
		executions = append(executions, executionTime.Format("2006-01-02 15:04:05"))

	case "daily":
		for _, timeStr := range req.ScheduledTimes {
			if timeStr > now.Format("15:04") {
				// Today
				todayStr := now.Format("2006-01-02") + " " + timeStr + ":00"
				if t, err := time.Parse("2006-01-02 15:04:05", todayStr); err == nil {
					executions = append(executions, t.Format("2006-01-02 15:04:05"))
				}
			} else {
				// Tomorrow
				tomorrow := now.AddDate(0, 0, 1)
				tomorrowStr := tomorrow.Format("2006-01-02") + " " + timeStr + ":00"
				if t, err := time.Parse("2006-01-02 15:04:05", tomorrowStr); err == nil {
					executions = append(executions, t.Format("2006-01-02 15:04:05"))
				}
			}
		}
	default:
		// Basic implementation for other types
		nextExecution := now.Add(24 * time.Hour) // Next day
		executions = append(executions, nextExecution.Format("2006-01-02 15:04:05"))
	}

	return executions
}

func jsonArrayToString(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}

	data, _ := json.Marshal(arr)
	return string(data)
}
