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

// addTable creates a properly formatted table for the query data.
func (pg *PDFGenerator) addTable(data *QueryData, startY float64) {
	if len(data.Columns) == 0 || len(data.Rows) == 0 {
		return
	}

	// Calculate optimal column widths based on content
	colWidths := pg.calculateColumnWidths(data)
	currentY := startY

	// Draw table headers
	currentY = pg.drawTableHeaders(data.Columns, colWidths, currentY)

	// Draw table rows
	pg.drawTableRows(data, colWidths, currentY)
}

// drawTableHeaders draws the table header row.
func (pg *PDFGenerator) drawTableHeaders(columns []string, colWidths []float64, startY float64) float64 {
	headerHeight := pg.config.TableRowHeight

	// Set header style
	pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetTextColor(255, 255, 255)
	pg.pdf.SetFont("arial", "", pg.config.FontSize+1) // Slightly larger for headers

	// Draw header background
	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}
	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, startY, totalWidth, headerHeight, "F")

	// Draw header borders
	pg.pdf.SetStrokeColor(100, 100, 100)
	pg.pdf.SetLineWidth(0.5)

	currentX := pg.config.MarginX
	for i, col := range columns {
		// Vertical border
		if i > 0 {
			pg.pdf.Line(currentX, startY, currentX, startY+headerHeight)
		}

		// Header text
		pg.pdf.SetXY(currentX+3, startY+8) // Padding

		// Truncate long column names
		colName := col
		maxChars := int(colWidths[i]/6) - 2 // Approximate characters that fit
		if len(colName) > maxChars && maxChars > 3 {
			colName = colName[:maxChars-3] + "..."
		}

		pg.pdf.Cell(nil, colName)
		currentX += colWidths[i]
	}

	// Draw horizontal borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX+totalWidth, startY)                           // Top
	pg.pdf.Line(pg.config.MarginX, startY+headerHeight, pg.config.MarginX+totalWidth, startY+headerHeight) // Bottom

	return startY + headerHeight
}

// drawTableRows draws all the table data rows - FIXED VERSION WITH PAGE NUMBERS
func (pg *PDFGenerator) drawTableRows(data *QueryData, colWidths []float64, startY float64) {
	if len(data.Rows) == 0 {
		return
	}

	rowHeight := pg.config.TableRowHeight
	currentY := startY
	pageNum := 1 // Track page numbers

	// A4 page dimensions: 595.35 x 841.995 points
	const pageHeight = 841.995
	bottomMargin := pg.config.MarginY
	maxYPosition := pageHeight - bottomMargin - 20 // Extra padding for safety

	// Set row style
	pg.pdf.SetTextColor(0, 0, 0)
	pg.pdf.SetFont("arial", "", pg.config.FontSize)
	pg.pdf.SetStrokeColor(200, 200, 200)
	pg.pdf.SetLineWidth(0.3)

	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	for rowIdx, row := range data.Rows {
		// Check if current row would exceed page boundaries
		if currentY+rowHeight > maxYPosition {
			// Add page number to current page before moving to next
			pg.addPageNumber(pageNum)
			pageNum++

			// Add new page
			pg.pdf.AddPage()
			currentY = pg.config.MarginY

			// Redraw headers on new page
			currentY = pg.drawTableHeaders(data.Columns, colWidths, currentY)
			pg.pdf.SetTextColor(0, 0, 0) // Reset to black text after headers
		}

		// Alternate row colors
		if rowIdx%2 == 1 {
			pg.pdf.SetFillColor(248, 249, 250)
			pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY, totalWidth, rowHeight, "F")
		}

		// Draw row borders
		pg.pdf.Line(pg.config.MarginX, currentY+rowHeight, pg.config.MarginX+totalWidth, currentY+rowHeight)

		// Draw cell content
		currentX := pg.config.MarginX
		for i, col := range data.Columns {
			// Vertical border
			if i > 0 {
				pg.pdf.Line(currentX, currentY, currentX, currentY+rowHeight)
			}

			// Cell content
			cellValue := "NULL"
			if val := row[col]; val != nil {
				cellValue = fmt.Sprintf("%v", val)
			}

			// Truncate text to fit column
			maxChars := int(colWidths[i]/6) - 2
			if len(cellValue) > maxChars && maxChars > 3 {
				cellValue = cellValue[:maxChars-3] + "..."
			}

			// Position text
			pg.pdf.SetXY(currentX+3, currentY+8) // Padding
			pg.pdf.Cell(nil, cellValue)

			currentX += colWidths[i]
		}

		// Move to next row position
		currentY += rowHeight
	}

	// Draw final vertical borders for the table
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX, currentY)                       // Left border
	pg.pdf.Line(pg.config.MarginX+totalWidth, startY, pg.config.MarginX+totalWidth, currentY) // Right border

	// Add summary
	if currentY+30 > maxYPosition {
		// Add page number to current page before moving to next
		pg.addPageNumber(pageNum)
		pageNum++

		pg.pdf.AddPage()
		currentY = pg.config.MarginY
	} else {
		currentY += 15
	}

	pg.pdf.SetFont("arial", "", 9)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("📊 Total rows: %d", len(data.Rows)))

	// Add page number to final page
	pg.addPageNumber(pageNum)
}

// Add page numbers
func (pg *PDFGenerator) addPageNumber(pageNum int) {
	// A4 page height in points
	const pageHeight = 841.995
	pg.pdf.SetFont("arial", "", 8)
	pg.pdf.SetTextColor(150, 150, 150)
	pg.pdf.SetXY(pg.config.MarginX, pageHeight-20)
	pg.pdf.Cell(nil, fmt.Sprintf("Page %d", pageNum))
}

// GenerateQueryResultsPDF creates a PDF from query results - UPDATED VERSION.
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
		Creator:  "SQL Executor v2.0",
		Producer: "GoPDF Enhanced",
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

// addQueryResultPage adds a page for each query result - IMPROVED VERSION.
func (pg *PDFGenerator) addQueryResultPage(result QueryResult, queryNum int) {
	pg.pdf.AddPage()

	currentY := pg.config.MarginY

	// Query header with better styling
	pg.pdf.SetFont("arial", "", 16)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Query %d Results", queryNum))
	currentY += 30

	// Status line with icons
	pg.pdf.SetFont("arial", "", 11)
	pg.pdf.SetTextColor(80, 80, 80)
	pg.pdf.SetXY(pg.config.MarginX, currentY)

	statusIcon := "✅"
	if result.Status != "success" {
		statusIcon = "❌"
	}

	statusText := fmt.Sprintf("%s Executed: %s | Status: %s | Duration: %s",
		statusIcon, result.Timestamp, result.Status, result.Duration)
	pg.pdf.Cell(nil, statusText)
	currentY += 25

	// Query text in a box
	if pg.config.ShowQuery {
		pg.pdf.SetFont("arial", "", 9)
		pg.pdf.SetTextColor(0, 0, 0)

		// Draw query box background
		queryBoxHeight := 25.0
		pg.pdf.SetFillColor(250, 250, 250)
		pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY,
			535-pg.config.MarginX, queryBoxHeight, "F")

		// Draw query box border
		pg.pdf.SetStrokeColor(200, 200, 200)
		pg.pdf.SetLineWidth(0.5)
		pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY,
			535-pg.config.MarginX, queryBoxHeight, "D")

		// Query text
		pg.pdf.SetXY(pg.config.MarginX+5, currentY+8)
		queryText := result.Query
		maxQueryLen := 100
		if len(queryText) > maxQueryLen {
			queryText = queryText[:maxQueryLen-3] + "..."
		}
		pg.pdf.Cell(nil, fmt.Sprintf("SQL: %s", queryText))
		currentY += queryBoxHeight + 15
	}

	// Handle errors with better formatting
	if result.Error != "" {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(220, 20, 20)

		// Error box
		errorBoxHeight := 30.0
		pg.pdf.SetFillColor(255, 245, 245)
		pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY,
			535-pg.config.MarginX, errorBoxHeight, "F")

		pg.pdf.SetXY(pg.config.MarginX+5, currentY+10)
		errorText := result.Error
		if len(errorText) > 80 {
			errorText = errorText[:77] + "..."
		}
		pg.pdf.Cell(nil, fmt.Sprintf("❌ Error: %s", errorText))
		return
	}

	// Handle empty results
	if result.Data == nil || len(result.Data.Rows) == 0 {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(100, 100, 100)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, "📭 No data returned.")
		return
	}

	// Add properly formatted table
	pg.addTable(result.Data, currentY)
}

// calculateColumnWidths determines optimal column widths - IMPROVED.
func (pg *PDFGenerator) calculateColumnWidths(data *QueryData) []float64 {
	pageWidth := 595.0
	usableWidth := pageWidth - (2 * pg.config.MarginX) - 10 // Safety margin

	numCols := len(data.Columns)
	if numCols == 0 {
		return []float64{}
	}

	// Start with equal distribution
	//baseWidth := usableWidth / float64(numCols)
	colWidths := make([]float64, numCols)

	// Minimum and maximum column widths
	minWidth := 60.0
	maxWidth := 150.0

	// Calculate content-based widths
	for i, col := range data.Columns {
		// Start with header width
		headerWidth := float64(len(col)*7) + 10 // 7 pts per char + padding

		// Check content width (sample first 5 rows)
		maxContentWidth := headerWidth
		sampleSize := 5
		if len(data.Rows) < sampleSize {
			sampleSize = len(data.Rows)
		}

		for j := 0; j < sampleSize; j++ {
			if val := data.Rows[j][col]; val != nil {
				contentStr := fmt.Sprintf("%v", val)
				contentWidth := float64(len(contentStr)*6) + 10 // 6 pts per char + padding
				if contentWidth > maxContentWidth {
					maxContentWidth = contentWidth
				}
			}
		}

		// Apply constraints
		if maxContentWidth < minWidth {
			colWidths[i] = minWidth
		} else if maxContentWidth > maxWidth {
			colWidths[i] = maxWidth
		} else {
			colWidths[i] = maxContentWidth
		}
	}

	// If total width exceeds available space, scale down
	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	if totalWidth > usableWidth {
		scale := usableWidth / totalWidth
		for i := range colWidths {
			colWidths[i] *= scale
			// Ensure still above minimum after scaling
			if colWidths[i] < minWidth {
				colWidths[i] = minWidth
			}
		}
	}

	return colWidths
}

// addTitlePage creates an improved title page.
func (pg *PDFGenerator) addTitlePage(queryCount int) {
	pg.pdf.AddPage()

	// Main title
	pg.pdf.SetFont("arial", "", 28)
	pg.pdf.SetXY(pg.config.MarginX, 80)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.Cell(nil, pg.config.Title)

	// Subtitle/Company
	pg.pdf.SetFont("arial", "", 18)
	pg.pdf.SetXY(pg.config.MarginX, 120)
	pg.pdf.SetTextColor(100, 100, 100)
	pg.pdf.Cell(nil, pg.config.CompanyName)

	// Report details box
	boxY := 180.0
	boxHeight := 100.0
	pg.pdf.SetFillColor(248, 249, 250)
	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, boxY, 400, boxHeight, "F")
	pg.pdf.SetStrokeColor(200, 200, 200)
	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, boxY, 400, boxHeight, "D")

	// Report info
	pg.pdf.SetFont("arial", "", 12)
	pg.pdf.SetTextColor(0, 0, 0)

	pg.pdf.SetXY(pg.config.MarginX+15, boxY+20)
	pg.pdf.Cell(nil, fmt.Sprintf("📊 Total Queries: %d", queryCount))

	if pg.config.ShowTimestamp {
		pg.pdf.SetXY(pg.config.MarginX+15, boxY+40)
		pg.pdf.Cell(nil, fmt.Sprintf("🕒 Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	}

	pg.pdf.SetXY(pg.config.MarginX+15, boxY+60)
	pg.pdf.Cell(nil, "🎯 Report Type: SQL Query Execution")

	// Footer line
	pg.pdf.SetLineWidth(2)
	pg.pdf.SetStrokeColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.Line(pg.config.MarginX, 320, 565, 320)
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

func displayResults(results []QueryResult) {
	fmt.Printf("\n--- Query Results  ---\n")

	for i, result := range results {
		fmt.Printf("\n=== Query %d ===\n", i+1)
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

	}
}

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

// savePDFToFile saves the PDF bytes to a file.
func savePDFToFile(pdfBytes []byte, filename string) error {
	return os.WriteFile(filename, pdfBytes, 0644)
}

func main() {
	// --- Configuration --- //
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

	// === CONFIGURATION OPTIONS === //
	selectedDB := "postgres"
	saveToFile := false
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

	// --- Database Connection --- //
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

	displayResults(results)

	// Save JSON results
	if saveToFile {
		if err := saveResultsToFile(results, outputFile); err != nil {
			log.Printf("Failed to save results to file: %v", err)
		} else {
			fmt.Printf("\nJSON results saved to: %s\n", outputFile)
		}
	}

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
