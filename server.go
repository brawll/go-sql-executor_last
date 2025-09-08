package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// type DatabaseConfig struct {
// 	Host     string `json:"host"`
// 	Port     int    `json:"port"`
// 	Database string `json:"database"`
// 	Username string `json:"username"`
// 	Password string `json:"password"`
// 	Driver   string `json:"driver"` // "postgres", "mysql", etc.
// }

// ReportRequest represents the incoming request structure
type ReportRequest struct {
	Query string `json:"query"`
	//	Database   DatabaseConfig    `json:"database"`
	// Parameters map[string]string `json:"parameters"` // Optional query parameters
}

// OpenAPI-compliant response structures
type ReportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	// Data    *QueryResult     `json:"data,omitempty"`
	Files   []FileInfo       `json:"files,omitempty"`
	Summary ExecutionSummary `json:"summary"`
}

type FileInfo struct {
	Type      string `json:"type"`
	Filename  string `json:"filename"`
	Path      string `json:"path,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Columns   int    `json:"columns,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

type ExecutionSummary struct {
	TotalQueries       int    `json:"total_queries"`
	SuccessfulQueries  int    `json:"successful_queries"`
	FailedQueries      int    `json:"failed_queries"`
	TotalExecutionTime string `json:"total_execution_time"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Database  string `json:"database"`
}

type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp"`
}

type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ReportService handles the core business logic
type ReportService struct {
	tempDir      string
	db           *sql.DB
	generateHTML bool
	generatePDF  bool
	exportCSV    bool
	exportExcel  bool
}

// NewReportService creates a new instance of ReportService
func NewReportService(db *sql.DB, generateHTML bool, generatePDF bool, exportCSV bool, exportExcel bool) *ReportService {
	tempDir := os.TempDir()
	return &ReportService{
		tempDir:      tempDir,
		db:           db,
		generateHTML: generateHTML,
		generatePDF:  generatePDF,
		exportCSV:    exportCSV,
		exportExcel:  exportExcel,
	}
}

// GenerateReportHandler handles HTTP requests for report generation
func (rs *ReportService) GenerateReportHandler(w http.ResponseWriter, r *http.Request) {
	// Set content type for all responses
	w.Header().Set("Content-Type", "application/json")

	// Start tracking execution time
	startTime := time.Now()

	// Verify HTTP method
	if r.Method != http.MethodPost {
		rs.writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
		return
	}

	// Parse request body
	var req ReportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Strict JSON parsing

	if err := decoder.Decode(&req); err != nil {
		rs.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", map[string]interface{}{"parse_error": err.Error()})
		return
	}

	// Validate request
	if err := rs.validateRequest(&req); err != nil {
		rs.writeErrorResponse(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", map[string]interface{}{"field": "query", "reason": err.Error()})
		return
	}

	// Query Execution
	queries := []string{req.Query}
	fmt.Printf("Found %d queries to execute.\n", len(queries))

	var wg sync.WaitGroup
	resultChan := make(chan QueryResult, len(queries))

	for _, query := range queries {
		wg.Add(1)
		go executeQuery(&wg, rs.db, query, resultChan)
	}

	wg.Wait()
	close(resultChan)

	// Collect results
	var results []QueryResult
	for result := range resultChan {
		results = append(results, result)
	}

	// Generate files and collect file information
	var files []FileInfo
	var errors []string

	// Export CSV
	if rs.exportCSV {
		fmt.Printf("Generating CSV report...\n")
		csvExporter := NewCSVExporter()
		csvFilename := "query_results_combined.csv"

		if err := csvExporter.ExportToCSV(results, csvFilename); err != nil {
			log.Printf("Failed to export CSV: %v", err)
			errors = append(errors, fmt.Sprintf("CSV export failed: %v", err))
		} else {
			fileInfo := FileInfo{
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
	if rs.exportExcel && len(results) > 0 {
		result := results[0] // Process first result
		excelFilename := "query_results.xlsx"

		if err := ExportToExcel(result, excelFilename); err != nil {
			log.Printf("Failed to export Excel: %v", err)
			errors = append(errors, fmt.Sprintf("Excel export failed: %v", err))
		} else {
			fileInfo := FileInfo{
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
	if rs.generateHTML {
		fmt.Printf("Generating HTML report...\n")

		htmlConfig := DefaultHTMLConfig()
		htmlConfig.CompanyName = "Rahul's Database Reports"
		htmlConfig.Title = "SQL Query Execution Report"
		htmlConfig.HeaderColor = "#2c3e50"
		htmlConfig.Theme = "light"

		htmlBytes, err := GenerateHTML(htmlConfig, results)
		if err != nil {
			log.Printf("Failed to generate HTML: %v", err)
			errors = append(errors, fmt.Sprintf("HTML generation failed: %v", err))
		} else {
			htmlOutputFile := "query_results_report.html"

			if err := saveHTMLToFile(htmlBytes, htmlOutputFile); err != nil {
				log.Printf("Failed to save HTML: %v", err)
				errors = append(errors, fmt.Sprintf("HTML save failed: %v", err))
			} else {
				fileInfo := FileInfo{
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
	if rs.generatePDF {
		fmt.Printf("Generating PDF report...\n")

		pdfConfig := DefaultPDFConfig()
		pdfConfig.CompanyName = "Rahul's Database Reports"
		pdfConfig.Title = "SQL Query Execution Report"
		pdfConfig.FontSize = 10
		pdfConfig.TableRowHeight = 20.0
		pdfConfig.MarginX = 20.0
		pdfConfig.MarginY = 20.0

		pdfGen := NewPDFGenerator(pdfConfig)
		pdfBytes, err := pdfGen.GenerateWideHorizontalPDF(results)

		if err != nil {
			log.Printf("Failed to generate PDF: %v", err)
			errors = append(errors, fmt.Sprintf("PDF generation failed: %v", err))
		} else {
			pdfOutputFile := "query_results_wide_horizontal.pdf"

			if err := savePDFToFile(pdfBytes, pdfOutputFile); err != nil {
				log.Printf("Failed to save PDF: %v", err)
				errors = append(errors, fmt.Sprintf("PDF save failed: %v", err))
			} else {
				fileInfo := FileInfo{
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

	summary := ExecutionSummary{
		TotalQueries:       len(results),
		SuccessfulQueries:  successfulQueries,
		FailedQueries:      failedQueries,
		TotalExecutionTime: executionTime,
	}

	// Prepare response
	response := ReportResponse{
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
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (rs *ReportService) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database connection
	dbStatus := "disconnected"
	if rs.db != nil {
		if err := rs.db.Ping(); err == nil {
			dbStatus = "connected"
		}
	}

	// Determine overall health status
	healthStatus := "healthy"
	if dbStatus == "disconnected" {
		healthStatus = "unhealthy"
		http.Error(w, `{"error":"Database connection failed"}`, http.StatusServiceUnavailable)
		return
	}

	response := HealthResponse{
		Status:    healthStatus,
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   "report-generator",
		Version:   "1.0.0",
		Database:  dbStatus,
	}

	fmt.Println(healthStatus)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateRequest validates the incoming request
func (rs *ReportService) validateRequest(req *ReportRequest) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// writeErrorResponse writes a structured error response according to OpenAPI spec
func (rs *ReportService) writeErrorResponse(w http.ResponseWriter, statusCode int, code string, message string, details map[string]interface{}) {
	errorResponse := ErrorResponse{
		Success: false,
		Error: ErrorInfo{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if details != nil {
		errorResponse.Error.Details = details
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}
