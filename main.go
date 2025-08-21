package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/signintech/gopdf"
)

// DBConfig holds the configuration for a database connection.
type DBConfig struct {
	DriverName     string
	DataSourceName string
}

// QueryData holds the actual data from a query result.
type QueryData struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

// QueryResult holds the result of a single query execution.
type QueryResult struct {
	Query     string     `json:"query"`
	Data      *QueryData `json:"data,omitempty"`
	Error     string     `json:"error,omitempty"`
	Status    string     `json:"status"`
	Duration  string     `json:"duration,omitempty"`
	Timestamp string     `json:"timestamp"`
}

// OutputFormat represents different ways to display results.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
	FormatPDF   OutputFormat = "pdf"
)

// PDFConfig holds PDF generation configuration.
type PDFConfig struct {
	Title          string
	Author         string
	Subject        string
	CompanyName    string
	HeaderColor    []uint8 // RGB values
	TableRowHeight float64
	FontSize       int
	MarginX        float64
	MarginY        float64
	ShowTimestamp  bool
	ShowQuery      bool
}

// PDFGenerator handles PDF creation for query results.
type PDFGenerator struct {
	pdf    *gopdf.GoPdf
	config PDFConfig
}

// DefaultPDFConfig returns a sensible default configuration.
func DefaultPDFConfig() PDFConfig {
	return PDFConfig{
		Title:          "SQL Query Results",
		Author:         "SQL Executor",
		Subject:        "Database Query Report",
		CompanyName:    "Your Company",
		HeaderColor:    []uint8{52, 73, 94},
		TableRowHeight: 25.0,
		FontSize:       10,
		MarginX:        30.0,
		MarginY:        30.0,
		ShowTimestamp:  true,
		ShowQuery:      true,
	}
}

// NewPDFGenerator creates a new PDF generator instance.
func NewPDFGenerator(config PDFConfig) *PDFGenerator {
	return &PDFGenerator{
		config: config,
	}
}

// setupFonts initializes fonts for the PDF.
func (pg *PDFGenerator) setupFonts() error {
	// Try different font approaches in order of preference

	// Method 1: Windows system fonts
	if err := pg.tryWindowsFonts(); err == nil {
		return nil
	}

	// Method 2: macOS system fonts
	if err := pg.tryMacOSFonts(); err == nil {
		return nil
	}

	// Method 3: Linux system fonts
	if err := pg.tryLinuxFonts(); err == nil {
		return nil
	}

	// Method 4: Download and use embedded font
	return pg.setupDownloadedFont()
}

// tryWindowsFonts attempts to load Windows system fonts.
func (pg *PDFGenerator) tryWindowsFonts() error {
	fontPaths := []string{
		"C:/Windows/Fonts/arial.ttf",
		"C:/Windows/Fonts/calibri.ttf",
		"C:/Windows/Fonts/tahoma.ttf",
		"C:/Windows/Fonts/verdana.ttf",
	}

	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			return pg.pdf.AddTTFFont("arial", path)
		}
	}

	return fmt.Errorf("no Windows fonts found")
}

// tryMacOSFonts attempts to load macOS system fonts.
func (pg *PDFGenerator) tryMacOSFonts() error {
	fontPaths := []string{
		"/System/Library/Fonts/Arial.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
		"/Library/Fonts/Arial.ttf",
	}

	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			return pg.pdf.AddTTFFont("arial", path)
		}
	}

	return fmt.Errorf("no macOS fonts found")
}

// tryLinuxFonts attempts to load Linux system fonts.
func (pg *PDFGenerator) tryLinuxFonts() error {
	fontPaths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/TTF/arial.ttf",
		"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
	}

	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			return pg.pdf.AddTTFFont("arial", path)
		}
	}

	return fmt.Errorf("no Linux fonts found")
}

// setupDownloadedFont downloads and uses a free font.
func (pg *PDFGenerator) setupDownloadedFont() error {
	if err := os.MkdirAll("fonts", 0755); err != nil {
		return fmt.Errorf("failed to create fonts directory: %w", err)
	}

	fontPath := "fonts/opensans.ttf"

	// Check if font already exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fmt.Println("Downloading Open Sans font...")
		if err := downloadFont(fontPath); err != nil {
			return fmt.Errorf("failed to download font: %w", err)
		}
	}

	return pg.pdf.AddTTFFont("arial", fontPath)
}

// downloadFont downloads Open Sans font from GitHub.
func downloadFont(fontPath string) error {
	url := "https://github.com/google/fonts/raw/main/apache/opensans/OpenSans-Regular.ttf"

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download font: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download font: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(fontPath)
	if err != nil {
		return fmt.Errorf("failed to create font file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save font file: %w", err)
	}

	fmt.Printf("✅ Font downloaded successfully to %s\n", fontPath)
	return nil
}

// GenerateQueryResultsPDF creates a PDF from query results.
func (pg *PDFGenerator) GenerateQueryResultsPDF(results []QueryResult) ([]byte, error) {
	pg.pdf = &gopdf.GoPdf{}
	pg.pdf.Start(gopdf.Config{
		PageSize: *gopdf.PageSizeA4,
		Unit:     gopdf.UnitPT,
	})

	// Set PDF metadata
	pg.pdf.SetInfo(gopdf.PdfInfo{
		Title:    pg.config.Title,
		Author:   pg.config.Author,
		Subject:  pg.config.Subject,
		Creator:  "SQL Executor",
		Producer: "GoPDF",
	})

	// Setup fonts
	if err := pg.setupFonts(); err != nil {
		return nil, fmt.Errorf("failed to setup fonts: %w", err)
	}

	// Generate title page
	pg.addTitlePage(len(results))

	// Add each query result
	for i, result := range results {
		pg.addQueryResultPage(result, i+1)
	}

	// Convert to bytes
	var buffer bytes.Buffer
	err := pg.pdf.Write(&buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to write PDF: %w", err)
	}

	return buffer.Bytes(), nil
}

// addTitlePage creates a title page for the PDF.
func (pg *PDFGenerator) addTitlePage(queryCount int) {
	pg.pdf.AddPage()
	pg.pdf.SetFont("arial", "", 24)

	// Title
	pg.pdf.SetXY(pg.config.MarginX, 100)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.Cell(nil, pg.config.Title)

	// Company name
	pg.pdf.SetFont("arial", "", 16)
	pg.pdf.SetXY(pg.config.MarginX, 140)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.Cell(nil, pg.config.CompanyName)

	// Report info
	pg.pdf.SetFont("arial", "", 12)
	pg.pdf.SetTextColor(0, 0, 0)

	pg.pdf.SetXY(pg.config.MarginX, 200)
	pg.pdf.Cell(nil, fmt.Sprintf("Total Queries: %d", queryCount))

	if pg.config.ShowTimestamp {
		pg.pdf.SetXY(pg.config.MarginX, 220)
		pg.pdf.Cell(nil, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	}

	// Add a horizontal line
	pg.pdf.SetLineWidth(1)
	pg.pdf.SetStrokeColor(200, 200, 200)
	pg.pdf.Line(pg.config.MarginX, 250, 565, 250)
}

// addQueryResultPage adds a page for each query result.
func (pg *PDFGenerator) addQueryResultPage(result QueryResult, queryNum int) {
	pg.pdf.AddPage()

	currentY := pg.config.MarginY

	// Query header
	pg.pdf.SetFont("arial", "", 14)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Query %d Results", queryNum))
	currentY += 25

	// Timestamp and status
	pg.pdf.SetFont("arial", "", 10)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Executed: %s | Status: %s | Duration: %s",
		result.Timestamp, result.Status, result.Duration))
	currentY += 20

	// Query text
	if pg.config.ShowQuery && result.Status == "success" {
		pg.pdf.SetFont("arial", "", 9)
		pg.pdf.SetTextColor(0, 0, 0)
		pg.pdf.SetXY(pg.config.MarginX, currentY)

		queryText := result.Query
		if len(queryText) > 80 {
			queryText = queryText[:80] + "..."
		}
		pg.pdf.Cell(nil, fmt.Sprintf("SQL: %s", queryText))
		currentY += 20
	}

	// Handle errors
	if result.Error != "" {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(220, 20, 20)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, fmt.Sprintf("Error: %s", result.Error))
		return
	}

	// Handle empty results
	if result.Data == nil || len(result.Data.Rows) == 0 {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(100, 100, 100)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, "No data returned.")
		return
	}

	// Add table
	pg.simpleAddTable(result.Data, currentY+10)
}

// addTable creates a table for the query data.
func (pg *PDFGenerator) addTable(data *QueryData, startY float64) {
	if len(data.Columns) == 0 || len(data.Rows) == 0 {
		return
	}

	pageWidth := 595.0
	availableWidth := pageWidth - (2 * pg.config.MarginX)
	colWidth := availableWidth / float64(len(data.Columns))

	maxColWidth := 120.0
	if colWidth > maxColWidth {
		colWidth = maxColWidth
	}

	currentY := startY

	// Table headers
	pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetTextColor(255, 255, 255)
	pg.pdf.SetFont("arial", "", pg.config.FontSize)

	for i, col := range data.Columns {
		x := pg.config.MarginX + (float64(i) * colWidth)
		pg.pdf.SetXY(x, currentY)

		pg.pdf.RectFromUpperLeftWithStyle(x, currentY, colWidth, pg.config.TableRowHeight, "F")
		pg.pdf.SetXY(x+2, currentY+7)

		colName := col
		if len(colName) > 15 {
			colName = colName[:12] + "..."
		}
		pg.pdf.Cell(nil, colName)
	}

	currentY += pg.config.TableRowHeight

	// Table rows
	pg.pdf.SetTextColor(0, 0, 0)
	rowCount := 0
	maxRowsPerPage := 25

	for _, row := range data.Rows {
		if rowCount >= maxRowsPerPage {
			pg.pdf.AddPage()
			currentY = pg.config.MarginY
			rowCount = 0

			// Re-add headers
			pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
			pg.pdf.SetTextColor(255, 255, 255)

			for i, col := range data.Columns {
				x := pg.config.MarginX + (float64(i) * colWidth)
				pg.pdf.SetXY(x, currentY)
				pg.pdf.RectFromUpperLeftWithStyle(x, currentY, colWidth, pg.config.TableRowHeight, "F")
				pg.pdf.SetXY(x+2, currentY+7)

				colName := col
				if len(colName) > 15 {
					colName = colName[:12] + "..."
				}
				pg.pdf.Cell(nil, colName)
			}
			currentY += pg.config.TableRowHeight
			pg.pdf.SetTextColor(0, 0, 0)
		}

		// Alternate row colors
		if rowCount%2 == 1 {
			pg.pdf.SetFillColor(248, 249, 250)
			for i := range data.Columns {
				x := pg.config.MarginX + (float64(i) * colWidth)
				pg.pdf.RectFromUpperLeftWithStyle(x, currentY, colWidth, pg.config.TableRowHeight, "F")
			}
		}

		// Add row data
		for i, col := range data.Columns {
			x := pg.config.MarginX + (float64(i) * colWidth)
			pg.pdf.SetXY(x+2, currentY+7)

			cellValue := ""
			if val := row[col]; val != nil {
				cellValue = fmt.Sprintf("%v", val)
			} else {
				cellValue = "NULL"
			}

			if len(cellValue) > 20 {
				cellValue = cellValue[:17] + "..."
			}

			pg.pdf.Cell(nil, cellValue)
		}

		currentY += pg.config.TableRowHeight
		rowCount++
	}

	// Add row count summary
	currentY += 10
	pg.pdf.SetFont("arial", "", 9)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Total rows: %d", len(data.Rows)))
}

// readQueriesFromFile reads SQL queries from a given file.
func readQueriesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open query file: %w", err)
	}
	defer file.Close()

	var queries []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "--") {
			queries = append(queries, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading query file: %w", err)
	}

	return queries, nil
}

// executeQuery executes a single SQL query and stores the complete result.
func executeQuery(
	wg *sync.WaitGroup,
	db *sql.DB,
	query string,
	resultChan chan<- QueryResult,
) {
	defer wg.Done()

	start := time.Now()
	timestamp := start.Format("2006-01-02 15:04:05")

	rows, err := db.Query(query)
	if err != nil {
		resultChan <- QueryResult{
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
		resultChan <- QueryResult{
			Query:     query,
			Error:     fmt.Sprintf("failed to get columns: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}
		return
	}

	var queryData QueryData
	queryData.Columns = columns
	queryData.Rows = make([]map[string]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			resultChan <- QueryResult{
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
		resultChan <- QueryResult{
			Query:     query,
			Error:     fmt.Sprintf("error iterating rows: %v", err),
			Status:    "error",
			Duration:  time.Since(start).String(),
			Timestamp: timestamp,
		}
		return
	}

	resultChan <- QueryResult{
		Query:     query,
		Data:      &queryData,
		Status:    "success",
		Duration:  time.Since(start).String(),
		Timestamp: timestamp,
	}
}

// displayResults shows the query results in the specified format.
func displayResults(results []QueryResult, format OutputFormat) {
	if format == FormatPDF {
		return
	}

	fmt.Printf("\n--- Query Results (%s format) ---\n", format)

	for i, result := range results {
		fmt.Printf("\n=== Query %d ===\n", i+1)
		fmt.Printf("SQL: %s\n", result.Query)
		fmt.Printf("Status: %s\n", result.Status)
		fmt.Printf("Duration: %s\n", result.Duration)

		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
			continue
		}

		if result.Data == nil || len(result.Data.Rows) == 0 {
			fmt.Println("No data returned.")
			continue
		}

		switch format {
		case FormatJSON:
			displayJSON(result.Data)
		case FormatCSV:
			displayCSV(result.Data)
		case FormatTable:
			displayTable(result.Data)
		default:
			displayTable(result.Data)
		}
	}
}

// displayJSON shows results in JSON format.
func displayJSON(data *QueryData) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

// displayCSV shows results in CSV format.
func displayCSV(data *QueryData) {
	fmt.Println(strings.Join(data.Columns, ","))
	for _, row := range data.Rows {
		values := make([]string, len(data.Columns))
		for i, col := range data.Columns {
			if val := row[col]; val != nil {
				values[i] = fmt.Sprintf("%v", val)
			} else {
				values[i] = ""
			}
		}
		fmt.Println(strings.Join(values, ","))
	}
}

// displayTable shows results in a formatted table.
func displayTable(data *QueryData) {
	if len(data.Rows) == 0 {
		fmt.Println("No rows returned.")
		return
	}

	colWidths := make([]int, len(data.Columns))
	for i, col := range data.Columns {
		colWidths[i] = len(col)
	}

	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := ""
			if row[col] != nil {
				val = fmt.Sprintf("%v", row[col])
			}
			if len(val) > colWidths[i] {
				colWidths[i] = len(val)
			}
		}
	}

	for i, col := range data.Columns {
		fmt.Printf("%-*s", colWidths[i]+2, col)
	}
	fmt.Println()

	for _, width := range colWidths {
		fmt.Printf("%s", strings.Repeat("-", width+2))
	}
	fmt.Println()

	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := ""
			if row[col] != nil {
				val = fmt.Sprintf("%v", row[col])
			}
			fmt.Printf("%-*s", colWidths[i]+2, val)
		}
		fmt.Println()
	}

	fmt.Printf("\nTotal rows: %d\n", len(data.Rows))
}

// saveResultsToFile saves all results to a JSON file.
func saveResultsToFile(results []QueryResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

// simpleAddTable - Basic but reliable table rendering.
func (pg *PDFGenerator) simpleAddTable(data *QueryData, startY float64) {
	if len(data.Columns) == 0 || len(data.Rows) == 0 {
		return
	}

	lineHeight := 20.0
	currentY := startY
	leftMargin := pg.config.MarginX

	// Simple headers
	pg.pdf.SetFont("arial", "", 12)
	pg.pdf.SetTextColor(0, 0, 0)
	pg.pdf.SetXY(leftMargin, currentY)

	headerText := strings.Join(data.Columns, " | ")
	pg.pdf.Cell(nil, headerText)
	currentY += lineHeight

	// Separator line
	pg.pdf.SetXY(leftMargin, currentY)
	pg.pdf.Cell(nil, strings.Repeat("-", len(headerText)))
	currentY += lineHeight

	// Data rows
	pg.pdf.SetFont("arial", "", 10)

	for i, row := range data.Rows {
		// Page break check
		if currentY > 750 {
			pg.pdf.AddPage()
			currentY = pg.config.MarginY
		}

		// Build row text
		var rowValues []string
		for _, col := range data.Columns {
			val := "NULL"
			if row[col] != nil {
				val = fmt.Sprintf("%v", row[col])
				// Truncate long values
				if len(val) > 25 {
					val = val[:22] + "..."
				}
			}
			rowValues = append(rowValues, val)
		}

		pg.pdf.SetXY(leftMargin, currentY)
		pg.pdf.Cell(nil, strings.Join(rowValues, " | "))
		currentY += lineHeight

		// Add some spacing every 5 rows
		if (i+1)%5 == 0 {
			currentY += 5
		}
	}

	// Summary
	currentY += 15
	pg.pdf.SetFont("arial", "", 9)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.SetXY(leftMargin, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Total rows: %d", len(data.Rows)))
}

// savePDFToFile saves the PDF bytes to a file.
func savePDFToFile(pdfBytes []byte, filename string) error {
	return os.WriteFile(filename, pdfBytes, 0644)
}

func main() {
	// --- Configuration ---
	dbConfigs := map[string]DBConfig{
		"postgres": {
			DriverName: "postgres",
			DataSourceName: "host=localhost port=5432 " +
				"user=postgres password=RahulM6? " +
				"dbname=sql-executor sslmode=disable",
		},
		"mysql": {
			DriverName:     "mysql",
			DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb",
		},
	}

	// === CONFIGURATION OPTIONS ===
	selectedDB := "postgres"
	outputFormat := FormatTable
	saveToFile := true
	outputFile := "query_results.json"

	// PDF options
	generatePDF := true
	pdfOutputFile := "query_results.pdf"
	pdfConfig := DefaultPDFConfig()
	pdfConfig.CompanyName = "Rahul's Database Reports"
	pdfConfig.Title = "SQL Query Execution Report"

	config, ok := dbConfigs[selectedDB]
	if !ok {
		log.Fatalf("Database '%s' not configured", selectedDB)
	}

	// --- Database Connection ---
	db, err := sql.Open(config.DriverName, config.DataSourceName)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	fmt.Printf("Successfully connected to %s!\n", selectedDB)

	// --- Query Execution ---
	queries, err := readQueriesFromFile("queries.txt")
	if err != nil {
		log.Fatalf("Failed to read queries: %v", err)
	}
	if len(queries) == 0 {
		log.Fatalf("No queries found in queries.txt")
	}
	fmt.Printf("Found %d queries to execute.\n", len(queries))

	var wg sync.WaitGroup
	resultChan := make(chan QueryResult, len(queries))

	for _, query := range queries {
		wg.Add(1)
		go executeQuery(&wg, db, query, resultChan)
	}

	wg.Wait()
	close(resultChan)

	// Collect results
	var results []QueryResult
	for result := range resultChan {
		results = append(results, result)
	}

	// Display results
	displayResults(results, outputFormat)

	// Save JSON results
	if saveToFile {
		if err := saveResultsToFile(results, outputFile); err != nil {
			log.Printf("Failed to save results to file: %v", err)
		} else {
			fmt.Printf("\nJSON results saved to: %s\n", outputFile)
		}
	}

	// Generate PDF
	if generatePDF {
		fmt.Printf("\nGenerating PDF report...\n")

		pdfGen := NewPDFGenerator(pdfConfig)
		pdfBytes, err := pdfGen.GenerateQueryResultsPDF(results)
		if err != nil {
			log.Printf("Failed to generate PDF: %v", err)
			fmt.Println("\n📋 PDF Generation Failed - Here's how to fix it:")
			fmt.Println("🔧 Quick Fix Options:")
			fmt.Println("   1. Run the font downloader (see below)")
			fmt.Println("   2. Or manually download a TTF font to fonts/opensans.ttf")
			fmt.Println("\n💡 To auto-download font, run this in a separate terminal:")
			fmt.Println("   go run download_font.go")
		} else {
			if err := savePDFToFile(pdfBytes, pdfOutputFile); err != nil {
				log.Printf("Failed to save PDF: %v", err)
			} else {
				fmt.Printf("📄 PDF report saved to: %s\n", pdfOutputFile)

				if absPath, err := filepath.Abs(pdfOutputFile); err == nil {
					fmt.Printf("   Full path: %s\n", absPath)
				}

				fmt.Printf("   File size: %.2f KB\n", float64(len(pdfBytes))/1024)
			}
		}
	}

	fmt.Printf("\n✅ Execution completed!\n")
	fmt.Printf("   - Executed %d queries\n", len(results))

	successCount := 0
	for _, result := range results {
		if result.Status == "success" {
			successCount++
		}
	}
	fmt.Printf("   - %d successful, %d failed\n", successCount, len(results)-successCount)

	if saveToFile {
		fmt.Printf("   - JSON saved: %s\n", outputFile)
	}
	if generatePDF {
		fmt.Printf("   - PDF generation: %s\n",
			map[bool]string{true: "✅ Success", false: "❌ Failed"}[err == nil])
	}
}
