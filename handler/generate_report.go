package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-sql-executor/exporters"
	"go-sql-executor/utils"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-sql-executor/models"

	"github.com/gin-gonic/gin"
)

// TemplateConfig represents the flattened template configuration for PDF generation
type TemplateConfig struct {
	Name                string  `json:"name"`
	TableHeadBgColor    string  `json:"table_head_bg_color"`
	TableHeadTextColor  string  `json:"table_head_text_color"`
	Logo                *string `json:"logo"`
	LogoPosition        string  `json:"logo_position"`
	ReportTitle         *string `json:"report_title"`
	ReportTitlePosition string  `json:"report_title_position"`
	TitleTextColor      string  `json:"title_text_color"`
	ContactEnabled      bool    `json:"contact_enabled"`
	ContactName         *string `json:"contact_name"`
	ContactAddress      *string `json:"contact_address"`
	ContactEmail        *string `json:"contact_email"`
	ContactPhone        *string `json:"contact_phone"`
	ContactPosition     string  `json:"contact_position"`
	ContactTextColor    string  `json:"contact_text_color"`
	FooterEnabled       bool    `json:"footer_enabled"`
	FooterText          *string `json:"footer_text"`
	FooterPosition      string  `json:"footer_position"`
}

type ReportService models.ReportService

// NewReportService creates a new instance of ReportService
func NewReportService(db *sql.DB, generateHTML bool, generatePDF bool, exportCSV bool, exportExcel bool) *ReportService {
	tempDir := os.TempDir()
	return &ReportService{
		TempDir:      tempDir,
		Db:           db,
		GenerateHTML: generateHTML,
		GeneratePDF:  generatePDF,
		ExportCSV:    exportCSV,
		ExportExcel:  exportExcel,
	}
}

// GenerateReportHandler handles HTTP requests for report generation
func (rs *ReportService) GenerateReportHandler(c *gin.Context) {
	// Start tracking execution time
	startTime := time.Now()

	// Verify HTTP method
	if c.Request.Method != http.MethodPost {
		rs.writeErrorResponseGin(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
		return
	}

	// Parse request body
	var req models.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		rs.writeErrorResponseGin(c, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", map[string]interface{}{"parse_error": err.Error()})
		return
	}

	// Validate request
	if err := rs.validateRequest(&req); err != nil {
		rs.writeErrorResponseGin(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", map[string]interface{}{"field": "query", "reason": err.Error()})
		return
	}

	// Fetch template config if template_id is provided
	templateConfig, err := rs.fetchTemplateConfig(&req)
	if err != nil {
		log.Printf("Warning: Failed to fetch template config: %v", err)
		templateConfig = nil
	}

	// Query Execution
	var results []models.QueryResult
	var queryToExecute string

	// New approach: build query from connection and schema info
	userConnInfo, err := utils.GetUserDBConnection(req.ConnectionID, rs.Db)
	if err != nil {
		rs.writeErrorResponseGin(c, http.StatusInternalServerError, "CONNECTION_ERROR", fmt.Sprintf("Failed to get connection info: %v", err), nil)
		return
	}

	// Establish database connection
	connStr, driverName := utils.GetConnectionInfo(userConnInfo.DBType, userConnInfo.Hostname,
		userConnInfo.Port, userConnInfo.Username, userConnInfo.Password, userConnInfo.DBName)

	if connStr == "" {
		rs.writeErrorResponseGin(c, http.StatusBadRequest, "UNSUPPORTED_DB_TYPE", fmt.Sprintf("Unsupported database type: %s", userConnInfo.DBType), nil)
		return
	}

	db, err := sql.Open(driverName, connStr)
	if err != nil {
		rs.writeErrorResponseGin(c, http.StatusInternalServerError, "CONNECTION_ERROR", fmt.Sprintf("Failed to establish database connection: %v", err), nil)
		return
	}
	defer db.Close()

	// Build SELECT query and execute
	queryToExecute = rs.buildSelectQuery(userConnInfo.DBType, req.SchemaName, req.TableName, req.SelectedFields)
	results, err = rs.executeQuery(db, queryToExecute)
	log.Printf("Executed query using connection_id: %s, schema: %s, table: %s", req.ConnectionID, req.SchemaName, req.TableName)

	if err != nil {
		rs.writeErrorResponseGin(c, http.StatusInternalServerError, "QUERY_EXECUTION_ERROR", fmt.Sprintf("Failed to execute query: %v", err), nil)
		return
	}

	// Generate files and collect file information
	var files []models.FileInfo
	var errors []string

	// Export CSV
	if rs.ExportCSV {
		fmt.Printf("Generating CSV report...\n")
		csvExporter := exporters.NewCSVExporter()
		csvFilename := "query_results_combined.csv"

		if err := csvExporter.ExportToCSV(results, csvFilename); err != nil {
			log.Printf("Failed to export CSV: %v", err)
			errors = append(errors, fmt.Sprintf("CSV export failed: %v", err))
		} else {
			fileInfo := models.FileInfo{
				Type:     "csv",
				Filename: csvFilename,
			}

			if stats, err := os.Stat(csvFilename); err == nil {
				fileInfo.SizeBytes = stats.Size()
				fileInfo.Path, _ = filepath.Abs(csvFilename)
			}

			// Count columns and rows from the first successful result
			if len(results) > 0 && results[0].Data != nil {
				fileInfo.Columns = len(results[0].Data.Columns)
				fileInfo.Rows = len(results[0].Data.Rows)
			}

			files = append(files, fileInfo)
			fmt.Printf("✅ CSV exported: %s\n", csvFilename)
		}
	}

	// Export Excel
	if rs.ExportExcel && len(results) > 0 {
		result := results[0] // Process first result
		excelFilename := "query_results.xlsx"

		if err := exporters.ExportToExcel(result, excelFilename); err != nil {
			log.Printf("Failed to export Excel: %v", err)
			errors = append(errors, fmt.Sprintf("Excel export failed: %v", err))
		} else {
			fileInfo := models.FileInfo{
				Type:     "excel",
				Filename: excelFilename,
			}

			if stats, err := os.Stat(excelFilename); err == nil {
				fileInfo.SizeBytes = stats.Size()
				fileInfo.Path, _ = filepath.Abs(excelFilename)
			}

			if result.Data != nil {
				fileInfo.Columns = len(result.Data.Columns)
				fileInfo.Rows = len(result.Data.Rows)
			}

			files = append(files, fileInfo)
			fmt.Printf("✅ Excel exported: %s\n", excelFilename)
		}
	}

	// Generate HTML
	if rs.GenerateHTML {
		fmt.Printf("Generating HTML report...\n")

		htmlConfig := exporters.DefaultHTMLConfig()
		htmlConfig.CompanyName = "Rahul's Database Reports"
		htmlConfig.Title = "SQL Query Execution Report"
		htmlConfig.HeaderColor = "#2c3e50"
		htmlConfig.Theme = "light"

		htmlBytes, err := exporters.GeneratePaginatedHTML(htmlConfig, results)
		if err != nil {
			log.Printf("Failed to generate HTML: %v", err)
			errors = append(errors, fmt.Sprintf("HTML generation failed: %v", err))
		} else {
			htmlOutputFile := "query_results_report.html"

			if err := exporters.SaveHTMLToFile(htmlBytes, htmlOutputFile); err != nil {
				log.Printf("Failed to save HTML: %v", err)
				errors = append(errors, fmt.Sprintf("HTML save failed: %v", err))
			} else {
				fileInfo := models.FileInfo{
					Type:      "html",
					Filename:  htmlOutputFile,
					SizeBytes: int64(len(htmlBytes)),
				}

				fileInfo.Path, _ = filepath.Abs(htmlOutputFile)

				// Count columns and rows from the first successful result
				if len(results) > 0 && results[0].Data != nil {
					fileInfo.Columns = len(results[0].Data.Columns)
					fileInfo.Rows = len(results[0].Data.Rows)
				}

				files = append(files, fileInfo)
				fmt.Printf("✅ HTML report generated: %s\n", htmlOutputFile)
			}
		}
	}

	// Generate PDF
	if rs.GeneratePDF {
		fmt.Printf("Generating PDF report...\n")

		pdfConfig := exporters.DefaultPDFConfig()
		pdfConfig.CompanyName = "Rahul's Database Reports"
		pdfConfig.Title = "SQL Query Execution Report"
		pdfConfig.FontSize = 10
		pdfConfig.TableRowHeight = 20.0
		pdfConfig.MarginX = 20.0
		pdfConfig.MarginY = 20.0

		// Apply template config to PDF if available
		if templateConfig != nil {
			rs.applyTemplateToPDF(&pdfConfig, templateConfig)
		}

		pdfGen := exporters.NewPDFGenerator(pdfConfig)
		pdfBytes, err := pdfGen.GenerateWideHorizontalPDF(results)

		if err != nil {
			log.Printf("Failed to generate PDF: %v", err)
			errors = append(errors, fmt.Sprintf("PDF generation failed: %v", err))
		} else {
			pdfOutputFile := "query_results_wide_horizontal.pdf"

			if err := exporters.SavePDFToFile(pdfBytes, pdfOutputFile); err != nil {
				log.Printf("Failed to save PDF: %v", err)
				errors = append(errors, fmt.Sprintf("PDF save failed: %v", err))
			} else {
				fileInfo := models.FileInfo{
					Type:      "pdf",
					Filename:  pdfOutputFile,
					SizeBytes: int64(len(pdfBytes)),
				}

				fileInfo.Path, _ = filepath.Abs(pdfOutputFile)

				// Count columns and rows from the first successful result
				if len(results) > 0 && results[0].Data != nil {
					fileInfo.Columns = len(results[0].Data.Columns)
					fileInfo.Rows = len(results[0].Data.Rows)
				}

				files = append(files, fileInfo)
				fmt.Printf("✅ PDF generated: %s\n", pdfOutputFile)
			}
		}
	}

	// Calculate execution summary
	executionTime := time.Since(startTime).String()
	successfulQueries := 0
	failedQueries := 0

	for _, result := range results {
		if result.Status == "success" {
			successfulQueries++
		} else {
			failedQueries++
		}
	}

	summary := models.ExecutionSummary{
		TotalQueries:       len(results),
		SuccessfulQueries:  successfulQueries,
		FailedQueries:      failedQueries,
		TotalExecutionTime: executionTime,
	}

	// Prepare response
	response := models.ReportResponse{
		Success: len(errors) == 0,
		Files:   files,
		Summary: summary,
	}

	// Handle success/failure cases
	if len(results) > 0 && results[0].Status == "success" {

		if len(errors) > 0 {
			response.Message = fmt.Sprintf("Report executed successfully but some files failed to generate: %s", errors[0])
		} else {
			response.Message = "Report generated successfully"
		}
	} else if len(results) > 0 {
		response.Message = fmt.Sprintf("Query executed but with errors: %s", results[0].Error)
	} else {
		response.Message = "No query results available"
	}

	// var report_title, report_filename string
	// if req.ReportFilename == "" {
	// 	report_filename = "report_" + time.Now().Format("2006-01-02_15-04-05")
	// } else {
	// 	report_filename = req.ReportFilename
	// }

	// if req.ReportTitle == "" {
	// 	report_title = req.CategoryName + "_" + templateConfig.Name + "_" + req.TableName
	// }

	path := "reports"
	_, err3 := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err3) {
			fmt.Printf("path don't exist: %v\n", path)
			if err := os.Mkdir(path, 0755); err != nil {
				log.Fatal("err")
			} else {
				fmt.Printf("created directory %s", path)
			}
		} else {
			fmt.Printf("Error checking path %v\n", err3)
		}
		return

	}

	// Return response
	c.JSON(http.StatusOK, response)
}

// fetchTemplateConfig fetches and transforms template configuration
func (rs *ReportService) fetchTemplateConfig(req *models.ReportRequest) (*TemplateConfig, error) {
	templateConfig := &TemplateConfig{}

	// Check if template_id is provided
	if req.TemplateID == nil {
		log.Println("No template_id provided, using default config")
		return nil, nil // No template, use defaults
	}

	log.Printf("Fetching template config for template_id: %s", *req.TemplateID)

	// Fetch template from template service
	templateURL := fmt.Sprintf("http://localhost:8000/api/templates/%s", *req.TemplateID)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(templateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch template: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("template service returned status: %d", resp.StatusCode)
	}

	// Parse the template response
	var rawTemplate map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawTemplate); err != nil {
		return nil, fmt.Errorf("failed to decode template response: %v", err)
	}

	log.Printf("Raw template config from service: %+v", rawTemplate["id"])

	// Check if template_config is provided directly in request payload as fallback
	if len(rawTemplate) == 0 && req.TemplateConfig != nil {
		log.Printf("Using template config from request payload as fallback")
		rawTemplate = req.TemplateConfig
	}

	// Transform nested structure to flat structure (same as Python version)
	templateConfig = rs.transformTemplateConfig(rawTemplate)

	log.Printf("Transformed template config: %+v", templateConfig)
	return templateConfig, nil
}

// transformTemplateConfig converts nested template structure to flat structure
func (rs *ReportService) transformTemplateConfig(rawTemplate map[string]interface{}) *TemplateConfig {
	config := &TemplateConfig{}

	// Set defaults
	config.TableHeadBgColor = "#f8fafc"
	config.TableHeadTextColor = "#000000"
	config.LogoPosition = "left"
	config.ReportTitlePosition = "center"
	config.TitleTextColor = "#0e0e0e"
	config.ContactEnabled = false
	config.ContactPosition = "right"
	config.ContactTextColor = "#1a237e"
	config.FooterEnabled = false
	config.FooterPosition = "center"

	// Extract top-level values with defaults
	if val, ok := rawTemplate["tableHeadBgColor"].(string); ok && val != "" {
		config.TableHeadBgColor = val
	}
	if val, ok := rawTemplate["tableHeadTextColor"].(string); ok && val != "" {
		config.TableHeadTextColor = val
	}
	if val, ok := rawTemplate["logo"].(string); ok && val != "" {
		config.Logo = new(string)
		*config.Logo = val
	}
	if val, ok := rawTemplate["logoPosition"].(string); ok && val != "" {
		config.LogoPosition = val
	}
	if val, ok := rawTemplate["reportTitle"].(string); ok && val != "" {
		config.ReportTitle = new(string)
		*config.ReportTitle = val
	}
	if val, ok := rawTemplate["reportTitlePosition"].(string); ok && val != "" {
		config.ReportTitlePosition = val
	}
	if val, ok := rawTemplate["titleTextColor"].(string); ok && val != "" {
		config.TitleTextColor = val
	}

	// Handle nested contactDetails
	if contactDetails, ok := rawTemplate["contactDetails"].(map[string]interface{}); ok {
		if enabled, ok := contactDetails["enabled"].(bool); ok {
			config.ContactEnabled = enabled
		}

		if name, ok := contactDetails["name"].(string); ok && name != "" {
			config.ContactName = new(string)
			*config.ContactName = name
		}
		if address, ok := contactDetails["address"].(string); ok && address != "" {
			config.ContactAddress = new(string)
			*config.ContactAddress = address
		}
		if email, ok := contactDetails["email"].(string); ok && email != "" {
			config.ContactEmail = new(string)
			*config.ContactEmail = email
		}
		if phone, ok := contactDetails["phone"].(string); ok && phone != "" {
			config.ContactPhone = new(string)
			*config.ContactPhone = phone
		}
		if position, ok := contactDetails["position"].(string); ok && position != "" {
			config.ContactPosition = position
		}
		if textColor, ok := contactDetails["textColor"].(string); ok && textColor != "" {
			config.ContactTextColor = textColor
		}
	}

	// Handle nested footer
	if footer, ok := rawTemplate["footer"].(map[string]interface{}); ok {
		if enabled, ok := footer["enabled"].(bool); ok {
			config.FooterEnabled = enabled
		}

		if text, ok := footer["text"].(string); ok && text != "" {
			config.FooterText = new(string)
			*config.FooterText = text
		}
		if position, ok := footer["position"].(string); ok && position != "" {
			config.FooterPosition = position
		}
	}

	return config
}

// applyTemplateToPDF applies template configuration to PDF config
func (rs *ReportService) applyTemplateToPDF(pdfConfig *models.PDFConfig, templateConfig *TemplateConfig) {
	// Apply title configuration
	if templateConfig.ReportTitle != nil {
		pdfConfig.Title = *templateConfig.ReportTitle
	}

	// Apply header color configuration
	// Convert hex color to RGB values
	if templateConfig.TableHeadBgColor != "" {
		rgb := hexToRGB(templateConfig.TableHeadBgColor)
		pdfConfig.HeaderColor = []uint8{rgb[0], rgb[1], rgb[2]}
	}

	log.Printf("Applied template config to PDF: title=%s, header_color=%s",
		pdfConfig.Title, templateConfig.TableHeadBgColor)
}

// hexToRGB converts hex color string to RGB values
func hexToRGB(hex string) [3]uint8 {
	// Handle #RRGGBB format or RRGGBB
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}

	if len(hex) != 6 {
		// Return default from Python example
		return [3]uint8{248, 250, 252}
	}

	// Convert hex to RGB
	r, g, b := hexToInt(hex[0:2]), hexToInt(hex[2:4]), hexToInt(hex[4:6])
	return [3]uint8{r, g, b}
}

// hexToInt converts hex string to int
func hexToInt(hex string) uint8 {
	var result uint8
	if len(hex) != 2 {
		return 0
	}

	for i := 0; i < 2; i++ {
		char := hex[i]
		var val uint8
		if char >= '0' && char <= '9' {
			val = char - '0'
		} else if char >= 'a' && char <= 'f' {
			val = char - 'a' + 10
		} else if char >= 'A' && char <= 'F' {
			val = char - 'A' + 10
		} else {
			return 0
		}

		if i == 0 {
			result = val * 16
		} else {
			result += val
		}
	}

	return result
}

// validateRequest validates the incoming request
func (rs *ReportService) validateRequest(req *models.ReportRequest) error {
	// For new payload format - validate required fields
	if req.ConnectionID != "" {
		if req.SchemaName == "" {
			return fmt.Errorf("schema_name is required when connection_id is provided")
		}
		if req.TableName == "" {
			return fmt.Errorf("table_name is required when connection_id is provided")
		}
		if len(req.SelectedFields) == 0 {
			return fmt.Errorf("selected_fields cannot be empty when connection_id is provided")
		}
		return nil
	} else {
		return fmt.Errorf("not recevied any connection_id")
	}

}

// quoteIdentifier quotes database identifiers (table/column names) to handle SQL keywords
func (rs *ReportService) quoteIdentifier(dbType, identifier string) string {
	if identifier == "" {
		return identifier
	}

	switch dbType {
	case "PostgreSQL":
		// PostgreSQL uses double quotes for identifiers
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	case "MySQL":
		// MySQL uses backticks for identifiers
		return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
	case "MSSQL":
		// SQL Server uses square brackets for identifiers
		return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
	default:
		// Default fallback - PostgreSQL style
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	}
}

// buildSelectQuery constructs SELECT query from schema, table, and fields with proper identifier quoting
func (rs *ReportService) buildSelectQuery(dbType, schemaName, tableName string, selectedFields []string) string {
	// Quote all column names
	quotedFields := make([]string, len(selectedFields))
	for i, field := range selectedFields {
		quotedFields[i] = rs.quoteIdentifier(dbType, field)
	}

	// Quote table and schema names
	quotedTableName := rs.quoteIdentifier(dbType, tableName)
	quotedSchemaName := rs.quoteIdentifier(dbType, schemaName)

	var query string

	switch dbType {
	case "PostgreSQL":
		if schemaName != "" && schemaName != "public" {
			query = fmt.Sprintf("SELECT %s FROM %s.%s",
				strings.Join(quotedFields, ", "), quotedSchemaName, quotedTableName)
		} else {
			query = fmt.Sprintf("SELECT %s FROM %s",
				strings.Join(quotedFields, ", "), quotedTableName)
		}
	case "MySQL":
		if schemaName != "" {
			query = fmt.Sprintf("SELECT %s FROM %s.%s",
				strings.Join(quotedFields, ", "), quotedSchemaName, quotedTableName)
		} else {
			query = fmt.Sprintf("SELECT %s FROM %s",
				strings.Join(quotedFields, ", "), quotedTableName)
		}
	case "MSSQL":
		if schemaName != "" {
			query = fmt.Sprintf("SELECT %s FROM %s.%s",
				strings.Join(quotedFields, ", "), quotedSchemaName, quotedTableName)
		} else {
			query = fmt.Sprintf("SELECT %s FROM %s",
				strings.Join(quotedFields, ", "), quotedTableName)
		}
	default:
		// Default fallback - PostgreSQL style
		if schemaName != "" {
			query = fmt.Sprintf("SELECT %s FROM %s.%s",
				strings.Join(quotedFields, ", "), quotedSchemaName, quotedTableName)
		} else {
			query = fmt.Sprintf("SELECT %s FROM %s",
				strings.Join(quotedFields, ", "), quotedTableName)
		}
	}

	log.Printf("Built query for %s: %s", dbType, query)
	return query
}

// executeQuery executes a single SQL query and returns the complete result synchronously
func (rs *ReportService) executeQuery(db *sql.DB, query string) ([]models.QueryResult, error) {
	start := time.Now()
	timestamp := start.Format("2006-01-02 15:04:05")

	rows, err := db.Query(query)
	if err != nil {
		return []models.QueryResult{{
			Query:     query,
			Error:     fmt.Sprintf("failed to execute query: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return []models.QueryResult{{
			Query:     query,
			Error:     fmt.Sprintf("failed to get columns: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}}, nil
	}

	var queryData models.QueryData
	queryData.Columns = columns
	queryData.Rows = make([]map[string]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return []models.QueryResult{{
				Query:     query,
				Error:     fmt.Sprintf("failed to scan row: %v", err),
				Status:    "error",
				Duration:  time.Since(start).String(),
				Timestamp: timestamp,
			}}, nil
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
			} else {
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else if t, ok := val.(time.Time); ok {
					// Format timestamp without timezone - only include microseconds when present
					if t.Nanosecond() > 0 {
						row[col] = t.Format("2006-01-02 15:04:05.000000")
					} else {
						row[col] = t.Format("2006-01-02 15:04:05")
					}
				} else {
					row[col] = val
				}
			}
		}

		queryData.Rows = append(queryData.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return []models.QueryResult{{
			Query:     query,
			Error:     fmt.Sprintf("error iterating rows: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}}, nil
	}

	return []models.QueryResult{{
		Query:     query,
		Data:      &queryData,
		Status:    "success",
		Duration:  time.Since(start).String(),
		Timestamp: timestamp,
	}}, nil
}

// writeErrorResponseGin writes a structured error response according to OpenAPI spec for Gin
func (rs *ReportService) writeErrorResponseGin(c *gin.Context, statusCode int, code string, message string, details map[string]interface{}) {
	errorResponse := models.ErrorResponse{
		Success: false,
		Error: models.ErrorInfo{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if details != nil {
		errorResponse.Error.Details = details
	}

	c.JSON(statusCode, errorResponse)
}
