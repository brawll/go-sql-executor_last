package handler

import (
	"database/sql"
	"fmt"
	"go-sql-executor/exporters"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-sql-executor/models"

	"github.com/gin-gonic/gin"
)

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

	// Query Execution
	queries := []string{req.Query}
	fmt.Printf("Found %d queries to execute.\n", len(queries))

	var wg sync.WaitGroup
	resultChan := make(chan models.QueryResult, len(queries))

	for _, query := range queries {
		wg.Add(1)
		go executeQuery(&wg, rs.Db, query, resultChan)
	}

	wg.Wait()
	close(resultChan)

	// Collect results
	var results []models.QueryResult
	for result := range resultChan {
		results = append(results, result)
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

		htmlBytes, err := exporters.GenerateHTML(htmlConfig, results)
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

	// Return response
	c.JSON(http.StatusOK, response)
}

// validateRequest validates the incoming request
func (rs *ReportService) validateRequest(req *models.ReportRequest) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
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

// executeQuery executes a single SQL query and stores the complete result.
func executeQuery(
	wg *sync.WaitGroup,
	db *sql.DB,
	query string,
	resultChan chan<- models.QueryResult,
) {
	defer wg.Done()

	start := time.Now()
	timestamp := start.Format("2006-01-02 15:04:05")

	rows, err := db.Query(query)
	if err != nil {
		resultChan <- models.QueryResult{
			Query:     query,
			Error:     fmt.Sprintf("failed to execute query: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		resultChan <- models.QueryResult{
			Query:     query,
			Error:     fmt.Sprintf("failed to get columns: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}
		return
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
			resultChan <- models.QueryResult{
				Query:     query,
				Error:     fmt.Sprintf("failed to scan row: %v", err),
				Status:    "error",
				Duration:  time.Since(start).String(),
				Timestamp: timestamp,
			}
			return
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
			} else {
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = val
				}
			}
		}

		queryData.Rows = append(queryData.Rows, row)
	}

	if err := rows.Err(); err != nil {
		resultChan <- models.QueryResult{
			Query:     query,
			Error:     fmt.Sprintf("error iterating rows: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}
		return
	}

	resultChan <- models.QueryResult{
		Query:     query,
		Data:      &queryData,
		Status:    "success",
		Duration:  time.Since(start).String(),
		Timestamp: timestamp,
	}
}
