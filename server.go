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
	// Verify HTTP method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ReportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Strict JSON parsing

	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error parsing request: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := rs.validateRequest(&req); err != nil {
		log.Printf("Request validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Query Execution
	queries := []string{req.Query}

	if len(queries) == 0 {
		log.Fatalf("No queries found")
	}
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

	displayResults(results)

	// CSV Export
	if rs.exportCSV {
		fmt.Printf("\nExporting to CSV...\n")
		csvExporter := NewCSVExporter()

		csvFilename := "query_results_combined.csv"
		if err := csvExporter.ExportToCSV(results, csvFilename); err != nil {
			log.Printf("Failed to export CSV: %v", err)
		} else {
			fmt.Printf("📄 CSV exported: %s\n", csvFilename)
			if absPath, err := filepath.Abs(csvFilename); err == nil {
				fmt.Printf("   📍 Full path: %s\n", absPath)
			}
		}
	}
	// Since you execute one query at a time, process the first result
	if len(results) > 0 {
		result := results[0] // Get first (and likely only) result
		if rs.exportExcel {
			excelFilename := "query_results.xlsx"
			if err := ExportToExcel(result, excelFilename); err != nil {
				log.Printf("Failed to export Excel: %v", err)
			} else {
				fmt.Printf("📊 Excel exported: %s\n", excelFilename)
				if result.Data != nil {
					fmt.Printf("   📋 %d columns × %d rows\n",
						len(result.Data.Columns), len(result.Data.Rows))
				}
			}
		}
	}

	if rs.generateHTML {
		fmt.Printf("\nGenerating HTML report...\n")

		htmlConfig := DefaultHTMLConfig()
		htmlConfig.CompanyName = "Rahul's Database Reports"
		htmlConfig.Title = "SQL Query Execution Report"
		htmlConfig.HeaderColor = "#2c3e50"
		htmlConfig.Theme = "light" // or "dark"

		htmlBytes, err := GenerateHTML(htmlConfig, results)
		if err != nil {
			log.Printf("Failed to generate HTML: %v", err)
		} else {
			htmlOutputFile := "query_results_report.html"
			if err := saveHTMLToFile(htmlBytes, htmlOutputFile); err != nil {
				log.Printf("Failed to save HTML: %v", err)
			} else {
				fmt.Printf("🌐 HTML report saved to: %s\n", htmlOutputFile)
				fmt.Printf("   📏 Open in web browser to see horizontally scrollable table\n")

				if absPath, err := filepath.Abs(htmlOutputFile); err == nil {
					fmt.Printf("   📍 Full path: %s\n", absPath)
				}
				fmt.Printf("   📊 File size: %.2f KB\n", float64(len(htmlBytes))/1024)
			}
		}
	}

	// PDF configuration for wide horizontal layout
	pdfConfig := DefaultPDFConfig()
	pdfConfig.CompanyName = "Rahul's Database Reports"
	pdfConfig.Title = "SQL Query Execution Report"
	pdfConfig.FontSize = 10
	pdfConfig.TableRowHeight = 20.0
	pdfConfig.MarginX = 20.0
	pdfConfig.MarginY = 20.0
	// Generate wide horizontal PDF
	if rs.generatePDF {
		fmt.Printf("\nGenerating wide horizontal PDF report...\n")

		pdfGen := NewPDFGenerator(pdfConfig)
		pdfBytes, err := pdfGen.GenerateWideHorizontalPDF(results)

		if err != nil {
			log.Printf("Failed to generate PDF: %v", err)
		} else {
			pdfOutputFile := "query_results_wide_horizontal.pdf"
			if err := savePDFToFile(pdfBytes, pdfOutputFile); err != nil {
				log.Printf("Failed to save PDF: %v", err)
			} else {
				fmt.Printf("📄 Wide horizontal PDF saved to: %s\n", pdfOutputFile)
				fmt.Printf("   📏 Use horizontal scroll in PDF viewer to see all columns\n")

				if absPath, err := filepath.Abs(pdfOutputFile); err == nil {
					fmt.Printf("   📍 Full path: %s\n", absPath)
				}
				fmt.Printf("   📊 File size: %.2f KB\n", float64(len(pdfBytes))/1024)
			}
		}
	}

	// Execution summary
	fmt.Printf("\n✅ Execution completed!\n")
	fmt.Printf("   - Executed %d queries\n", len(results))

	successCount := 0
	for _, result := range results {
		if result.Status == "success" {
			successCount++
		}
	}
	fmt.Printf("   - %d successful, %d failed\n", successCount, len(results)-successCount)

}

// HealthCheckHandler provides a simple health check endpoint
func (rs *ReportService) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "report-generator",
	}
	// w.WriteHeader(http.StatusOK) not needed since w.Write already do this if it's not happening.
	w.Write([]byte("OK"))
	json.NewEncoder(w).Encode(response)
}

// validateRequest validates the incoming request
func (rs *ReportService) validateRequest(req *ReportRequest) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}
