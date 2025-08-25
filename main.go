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

const (
	pageWidth  = 841.995 // Landscape A4 width (was height in portrait)
	pageHeight = 595.35  // Landscape A4 height (was width in portrait)
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

// Update DefaultPDFConfig for landscape and many columns
func DefaultPDFConfig() PDFConfig {
	return PDFConfig{
		Title:          "SQL Query Results",
		Author:         "SQL Executor",
		Subject:        "Database Query Report",
		CompanyName:    "Your Company",
		HeaderColor:    []uint8{52, 73, 94},
		TableRowHeight: 20.0, // Reduced from 25.0
		FontSize:       8,    // Reduced from 10
		MarginX:        20.0, // Reduced from 30.0
		MarginY:        20.0, // Reduced from 30.0
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

// drawTableRows draws all the table data rows - FIXED VERSION WITH PAGE NUMBERS
func (pg *PDFGenerator) drawTableRows(data *QueryData, colWidths []float64, startY float64) {
	if len(data.Rows) == 0 {
		return
	}

	rowHeight := pg.config.TableRowHeight
	currentY := startY
	pageNum := 1 // Track page numbers

	bottomMargin := pg.config.MarginY
	maxYPosition := pageHeight - bottomMargin - 20 // Use new pageHeight constant

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

// Update addPageNumber function
func (pg *PDFGenerator) addPageNumber(pageNum int) {
	pg.pdf.SetFont("arial", "", 8)
	pg.pdf.SetTextColor(150, 150, 150)
	pg.pdf.SetXY(pg.config.MarginX, pageHeight-20) // Use new pageHeight
	pg.pdf.Cell(nil, fmt.Sprintf("Page %d", pageNum))
}

// Add this new method for two-level pagination
func (pg *PDFGenerator) addSplitTableWithFullPagination(data *QueryData, startY float64, maxColumnsPerPage int) {
	if data == nil || len(data.Columns) == 0 || len(data.Rows) == 0 {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(100, 100, 100)
		pg.pdf.SetXY(pg.config.MarginX, startY)
		pg.pdf.Cell(nil, "📭 No data returned.")
		return
	}

	// If columns fit in one page, use regular table with pagination
	if len(data.Columns) <= maxColumnsPerPage {
		pg.addTableWithRowControl(data, startY, false) // Enable row pagination
		return
	}

	// Calculate pagination parameters
	totalColumns := len(data.Columns)
	totalRows := len(data.Rows)
	totalColumnChunks := (totalColumns + maxColumnsPerPage - 1) / maxColumnsPerPage

	// Calculate how many rows fit per page (considering headers and margins)
	availableHeight := pageHeight - 100 - (2 * pg.config.MarginY) // Space for headers/footers
	maxRowsPerPage := int(availableHeight / pg.config.TableRowHeight)

	// Ensure minimum rows per page
	if maxRowsPerPage < 5 {
		maxRowsPerPage = 5
	}

	totalRowBatches := (totalRows + maxRowsPerPage - 1) / maxRowsPerPage

	fmt.Printf("🔄 Pagination Strategy:\n")
	fmt.Printf("   📊 Total data: %d columns × %d rows\n", totalColumns, totalRows)
	fmt.Printf("   📄 Column chunks: %d (max %d cols per chunk)\n", totalColumnChunks, maxColumnsPerPage)
	fmt.Printf("   📑 Row batches: %d (max %d rows per batch)\n", totalRowBatches, maxRowsPerPage)
	fmt.Printf("   📋 Total pages: ~%d\n\n", totalColumnChunks*totalRowBatches)

	isFirstOverallPage := true

	// OUTER LOOP: Row batches
	for rowBatchIndex := 0; rowBatchIndex < totalRowBatches; rowBatchIndex++ {
		startRow := rowBatchIndex * maxRowsPerPage
		endRow := startRow + maxRowsPerPage
		if endRow > totalRows {
			endRow = totalRows
		}

		fmt.Printf("🔢 Processing Row Batch %d: Rows %d-%d\n",
			rowBatchIndex+1, startRow+1, endRow)

		// Get row batch
		rowBatch := data.Rows[startRow:endRow]

		// INNER LOOP: Column chunks for this row batch
		for columnChunkIndex := 0; columnChunkIndex < totalColumnChunks; columnChunkIndex++ {
			startCol := columnChunkIndex * maxColumnsPerPage
			endCol := startCol + maxColumnsPerPage
			if endCol > totalColumns {
				endCol = totalColumns
			}

			// fmt.Printf("   📄 Column Chunk %d: Columns %d-%d (%s to %s)\n",
			// 	columnChunkIndex+1, startCol+1, endCol,
			// 	data.Columns[startCol], data.Columns[endCol-1])

			// Create subset of columns for this chunk
			chunkColumns := make([]string, endCol-startCol)
			copy(chunkColumns, data.Columns[startCol:endCol])

			// Create subset of rows with only the columns for this chunk
			chunkRows := make([]map[string]interface{}, len(rowBatch))
			for i, originalRow := range rowBatch {
				chunkRow := make(map[string]interface{})
				for _, colName := range chunkColumns {
					chunkRow[colName] = originalRow[colName]
				}
				chunkRows[i] = chunkRow
			}

			// Create the subset data
			subsetData := &QueryData{
				Columns: chunkColumns,
				Rows:    chunkRows,
			}

			// Add new page for all chunks except the very first one
			if !isFirstOverallPage {
				pg.pdf.AddPage()
			}
			isFirstOverallPage = false

			// Add page header with batch and chunk information
			currentY := pg.config.MarginY

			// Main header
			pg.pdf.SetFont("arial", "", 14)
			pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
			pg.pdf.SetXY(pg.config.MarginX, currentY)

			if totalRowBatches == 1 {
				// Single row batch - just show column info
				pg.pdf.Cell(nil, fmt.Sprintf("Columns %d-%d of %d", startCol+1, endCol, totalColumns))
			} else {
				// Multiple row batches - show both row and column info
				pg.pdf.Cell(nil, fmt.Sprintf("Rows %d-%d | Columns %d-%d of %d",
					startRow+1, endRow, startCol+1, endCol, totalColumns))
			}
			currentY += 25

			// Sub header
			pg.pdf.SetFont("arial", "", 10)
			pg.pdf.SetTextColor(100, 100, 100)
			pg.pdf.SetXY(pg.config.MarginX, currentY)

			if totalRowBatches == 1 {
				pg.pdf.Cell(nil, fmt.Sprintf("Column Chunk %d of %d | From: %s → To: %s",
					columnChunkIndex+1, totalColumnChunks, chunkColumns[0], chunkColumns[len(chunkColumns)-1]))
			} else {
				pg.pdf.Cell(nil, fmt.Sprintf("Row Batch %d of %d | Column Chunk %d of %d | %s → %s",
					rowBatchIndex+1, totalRowBatches, columnChunkIndex+1, totalColumnChunks,
					chunkColumns[0], chunkColumns[len(chunkColumns)-1]))
			}
			currentY += 20

			// Add the table for this chunk - DISABLE internal row pagination
			pg.addTableWithRowControl(subsetData, currentY, true) // Disable internal pagination
		}
	}

	// Add final summary page
	pg.pdf.AddPage()
	pg.addDataSummaryPage(data, totalColumnChunks, totalRowBatches)
}

// Add summary page method
func (pg *PDFGenerator) addDataSummaryPage(data *QueryData, totalColumnChunks, totalRowBatches int) {
	currentY := pg.config.MarginY + 50

	// Title
	pg.pdf.SetFont("arial", "", 18)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, "📊 Data Summary")
	currentY += 40

	// Statistics box
	boxHeight := 120.0
	pg.pdf.SetFillColor(248, 249, 250)
	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY, 400, boxHeight, "F")
	pg.pdf.SetStrokeColor(200, 200, 200)
	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY, 400, boxHeight, "D")

	pg.pdf.SetFont("arial", "", 12)
	pg.pdf.SetTextColor(0, 0, 0)

	pg.pdf.SetXY(pg.config.MarginX+15, currentY+20)
	pg.pdf.Cell(nil, fmt.Sprintf("📋 Total Columns: %d", len(data.Columns)))

	pg.pdf.SetXY(pg.config.MarginX+15, currentY+40)
	pg.pdf.Cell(nil, fmt.Sprintf("📑 Total Rows: %d", len(data.Rows)))

	pg.pdf.SetXY(pg.config.MarginX+15, currentY+60)
	pg.pdf.Cell(nil, fmt.Sprintf("📄 Column Chunks: %d", totalColumnChunks))

	pg.pdf.SetXY(pg.config.MarginX+15, currentY+80)
	pg.pdf.Cell(nil, fmt.Sprintf("🔢 Row Batches: %d", totalRowBatches))
}

// Update the main addSplitTable method to use the new pagination
func (pg *PDFGenerator) addSplitTable(data *QueryData, startY float64, maxColumnsPerPage int) {
	// Use the new two-level pagination method
	pg.addSplitTableWithFullPagination(data, startY, maxColumnsPerPage)
}

// Add this method to split large tables across multiple pages
// addSplitTable splits large tables across multiple pages horizontally
// func (pg *PDFGenerator) addSplitTable(data *QueryData, startY float64, maxColumnsPerPage int) {
// 	if data == nil || len(data.Columns) == 0 || len(data.Rows) == 0 {
// 		pg.pdf.SetFont("arial", "", 10)
// 		pg.pdf.SetTextColor(100, 100, 100)
// 		pg.pdf.SetXY(pg.config.MarginX, startY)
// 		pg.pdf.Cell(nil, "📭 No data returned.")
// 		return
// 	}

// 	// If columns fit in one page, use regular table with pagination
// 	if len(data.Columns) <= maxColumnsPerPage {
// 		pg.addTableWithRowControl(data, startY, false) // Enable row pagination
// 		return
// 	}

// 	// Calculate total chunks needed
// 	totalColumns := len(data.Columns)
// 	totalChunks := (totalColumns + maxColumnsPerPage - 1) / maxColumnsPerPage

// 	fmt.Printf("🔄 Splitting %d columns into %d chunks of max %d columns each\n",
// 		totalColumns, totalChunks, maxColumnsPerPage)

// 	for chunkIndex := 0; chunkIndex < totalChunks; chunkIndex++ {
// 		startCol := chunkIndex * maxColumnsPerPage
// 		endCol := startCol + maxColumnsPerPage
// 		if endCol > totalColumns {
// 			endCol = totalColumns
// 		}

// 		fmt.Printf("   📄 Chunk %d: Columns %d-%d (%s to %s)\n",
// 			chunkIndex+1, startCol+1, endCol,
// 			data.Columns[startCol], data.Columns[endCol-1])

// 		// Create subset of columns for this chunk
// 		chunkColumns := make([]string, endCol-startCol)
// 		copy(chunkColumns, data.Columns[startCol:endCol])

// 		// Create subset of rows with only the columns for this chunk
// 		chunkRows := make([]map[string]interface{}, len(data.Rows))
// 		for i, originalRow := range data.Rows {
// 			chunkRow := make(map[string]interface{})
// 			for _, colName := range chunkColumns {
// 				chunkRow[colName] = originalRow[colName]
// 			}
// 			chunkRows[i] = chunkRow
// 		}

// 		// Create the subset data
// 		subsetData := &QueryData{
// 			Columns: chunkColumns,
// 			Rows:    chunkRows,
// 		}

// 		// Add new page for chunks after the first one
// 		if chunkIndex > 0 {
// 			pg.pdf.AddPage()

// 			// Add chunk information header
// 			currentY := pg.config.MarginY

// 			pg.pdf.SetFont("arial", "", 14)
// 			pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
// 			pg.pdf.SetXY(pg.config.MarginX, currentY)
// 			pg.pdf.Cell(nil, fmt.Sprintf("Columns %d-%d of %d", startCol+1, endCol, totalColumns))
// 			currentY += 25

// 			pg.pdf.SetFont("arial", "", 10)
// 			pg.pdf.SetTextColor(100, 100, 100)
// 			pg.pdf.SetXY(pg.config.MarginX, currentY)
// 			pg.pdf.Cell(nil, fmt.Sprintf("Chunk %d of %d | From: %s → To: %s",
// 				chunkIndex+1, totalChunks, chunkColumns[0], chunkColumns[len(chunkColumns)-1]))
// 			currentY += 20

// 			// Add the table for this chunk - DISABLE row pagination within chunks
// 			pg.addTableWithRowControl(subsetData, currentY, true) // Disable row pagination
// 		} else {
// 			// First chunk - add on current page with info
// 			pg.pdf.SetFont("arial", "", 10)
// 			pg.pdf.SetTextColor(100, 100, 100)
// 			pg.pdf.SetXY(pg.config.MarginX, startY-15)
// 			pg.pdf.Cell(nil, fmt.Sprintf("📊 Showing columns 1-%d of %d total | From: %s → To: %s",
// 				endCol, totalColumns, chunkColumns[0], chunkColumns[len(chunkColumns)-1]))

// 			// DISABLE row pagination for the first chunk too
// 			pg.addTableWithRowControl(subsetData, startY, true) // Disable row pagination
// 		}
// 	}
// }

// debugColumnSplit - temporary function to verify column splitting
func debugColumnSplit(data *QueryData, maxColumnsPerPage int) {
	fmt.Printf("\n=== DEBUG: Column Splitting ===\n")
	fmt.Printf("Total columns: %d\n", len(data.Columns))
	fmt.Printf("Max per page: %d\n", maxColumnsPerPage)

	totalChunks := (len(data.Columns) + maxColumnsPerPage - 1) / maxColumnsPerPage
	fmt.Printf("Total chunks: %d\n", totalChunks)

	for chunkIndex := 0; chunkIndex < totalChunks; chunkIndex++ {
		startCol := chunkIndex * maxColumnsPerPage
		endCol := startCol + maxColumnsPerPage
		if endCol > len(data.Columns) {
			endCol = len(data.Columns)
		}

		fmt.Printf("\nChunk %d:\n", chunkIndex+1)
		fmt.Printf("  Range: %d-%d\n", startCol+1, endCol)
		fmt.Printf("  First column: %s\n", data.Columns[startCol])
		fmt.Printf("  Last column: %s\n", data.Columns[endCol-1])
		fmt.Printf("  Columns in chunk: %v\n", data.Columns[startCol:endCol][:3]) // Show first 3
	}
	fmt.Printf("================================\n\n")
}

// GenerateQueryResultsPDF creates a PDF from query results - UPDATED VERSION.
func (pg *PDFGenerator) GenerateQueryResultsPDF(results []QueryResult) ([]byte, error) {
	pg.pdf = &gopdf.GoPdf{}
	// Change to landscape orientation
	pg.pdf.Start(gopdf.Config{
		PageSize: *gopdf.PageSizeA4Landscape, // This is the key change
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

// Add this helper method to measure actual text width
func (pg *PDFGenerator) measureTextWidth(text string) float64 {
	// gopdf has a GetStringWidth method to measure actual text width
	width, err := pg.pdf.MeasureTextWidth(text)
	if err != nil {
		// Fallback to rough estimation if measurement fails
		return float64(len(text)) * float64(pg.config.FontSize) * 0.6
	}
	return width
}

// Add this method to properly truncate text based on actual width
func (pg *PDFGenerator) truncateTextToFit(text string, maxWidth float64) string {
	if text == "" {
		return text
	}

	// Check if full text fits
	fullWidth := pg.measureTextWidth(text)
	if fullWidth <= maxWidth {
		return text
	}

	// Binary search for optimal length
	ellipsis := "..."
	ellipsisWidth := pg.measureTextWidth(ellipsis)
	availableWidth := maxWidth - ellipsisWidth

	if availableWidth <= 0 {
		return ellipsis
	}

	// Use binary search to find the maximum characters that fit
	left, right := 0, len(text)
	bestFit := ""

	for left <= right {
		mid := (left + right) / 2
		testText := text[:mid]
		testWidth := pg.measureTextWidth(testText)

		if testWidth <= availableWidth {
			bestFit = testText
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	if bestFit == "" {
		return ellipsis
	}

	return bestFit + ellipsis
}

// Updated drawTableRowsWithControl method with proper text truncation
func (pg *PDFGenerator) drawTableRowsWithControl(data *QueryData, colWidths []float64, startY float64, disableRowPagination bool) {
	if len(data.Rows) == 0 {
		return
	}

	rowHeight := pg.config.TableRowHeight
	currentY := startY
	pageNum := 1

	bottomMargin := pg.config.MarginY
	maxYPosition := pageHeight - bottomMargin - 20

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
		// Page break logic (unchanged)
		if !disableRowPagination && currentY+rowHeight > maxYPosition {
			pg.addPageNumber(pageNum)
			pageNum++
			pg.pdf.AddPage()
			currentY = pg.config.MarginY
			currentY = pg.drawTableHeaders(data.Columns, colWidths, currentY)
			pg.pdf.SetTextColor(0, 0, 0)
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

			// IMPROVED: Use proper text width measurement for truncation
			cellPadding := 6.0 // Left + right padding
			availableWidth := colWidths[i] - cellPadding

			// Truncate text to fit available width properly
			displayValue := pg.truncateTextToFit(cellValue, availableWidth)

			// Position text with padding
			pg.pdf.SetXY(currentX+3, currentY+8) // 3pt left padding
			pg.pdf.Cell(nil, displayValue)

			currentX += colWidths[i]
		}

		currentY += rowHeight
	}

	// Draw final vertical borders for the table
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX, currentY)
	pg.pdf.Line(pg.config.MarginX+totalWidth, startY, pg.config.MarginX+totalWidth, currentY)

	// Add summary (unchanged)
	if !disableRowPagination && currentY+30 > maxYPosition {
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

	if !disableRowPagination {
		pg.addPageNumber(pageNum)
	}
}

// Also update the header drawing method
func (pg *PDFGenerator) drawTableHeaders(columns []string, colWidths []float64, startY float64) float64 {
	headerHeight := pg.config.TableRowHeight

	// Set header style
	pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetTextColor(255, 255, 255)
	fontSize := pg.config.FontSize
	if len(columns) > 50 {
		fontSize = 7
	}
	pg.pdf.SetFont("arial", "", fontSize+1)

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
		if i > 0 {
			pg.pdf.Line(currentX, startY, currentX, startY+headerHeight)
		}

		// IMPROVED: Header text with proper width measurement
		headerPadding := 4.0
		availableWidth := colWidths[i] - headerPadding

		// Truncate header text properly
		displayCol := pg.truncateTextToFit(col, availableWidth)

		pg.pdf.SetXY(currentX+2, startY+5)
		pg.pdf.Cell(nil, displayCol)
		currentX += colWidths[i]
	}

	// Draw horizontal borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX+totalWidth, startY)
	pg.pdf.Line(pg.config.MarginX, startY+headerHeight, pg.config.MarginX+totalWidth, startY+headerHeight)

	return startY + headerHeight
}

// Option 1: Add a parameter to control row pagination
func (pg *PDFGenerator) addTableWithRowControl(data *QueryData, startY float64, disableRowPagination bool) {
	if len(data.Columns) == 0 || len(data.Rows) == 0 {
		return
	}

	// Calculate optimal column widths
	colWidths := pg.calculateColumnWidths(data)
	currentY := startY

	// Draw table headers
	currentY = pg.drawTableHeaders(data.Columns, colWidths, currentY)

	// Draw table rows with optional pagination control
	pg.drawTableRowsWithControl(data, colWidths, currentY, disableRowPagination)
}

// addQueryResultPage adds a page for each query result - IMPROVED VERSION.
func (pg *PDFGenerator) addQueryResultPage(result QueryResult, queryNum int) {
	pg.pdf.AddPage()

	currentY := pg.config.MarginY

	// Query header with better styling
	pg.pdf.SetFont("arial", "", 16)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	//pg.pdf.Cell(nil, fmt.Sprintf("Query %d Results", queryNum))
	currentY += 30

	// Status line with icons
	pg.pdf.SetFont("arial", "", 11)
	pg.pdf.SetTextColor(80, 80, 80)
	pg.pdf.SetXY(pg.config.MarginX, currentY)

	// Query text in a box
	if pg.config.ShowQuery {
		pg.pdf.SetFont("arial", "", 9)
		pg.pdf.SetTextColor(0, 0, 0)

		// Draw query box background
		queryBoxHeight := 25.0
		pg.pdf.SetFillColor(250, 250, 250)
		pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY,
			535-pg.config.MarginX, queryBoxHeight, "F")

		// // Draw query box border
		// pg.pdf.SetStrokeColor(200, 200, 200)
		// pg.pdf.SetLineWidth(0.5)
		// pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY,
		// 	535-pg.config.MarginX, queryBoxHeight, "D")

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

	// *** ADD DEBUG OUTPUT ***
	fmt.Printf("\n🔍 Processing Query %d with %d columns\n", queryNum, len(result.Data.Columns))
	if len(result.Data.Columns) > 0 {
		fmt.Printf("   First column: %s\n", result.Data.Columns[0])
		fmt.Printf("   Last column: %s\n", result.Data.Columns[len(result.Data.Columns)-1])
	}

	// WITH THIS:
	// Add split table for many columns (153 columns)
	maxColumnsPerPage := 16 // Adjust based on your needs
	debugColumnSplit(result.Data, maxColumnsPerPage)
	pg.addSplitTable(result.Data, currentY, maxColumnsPerPage)
}

// Updated calculateColumnWidths to handle 153 columns
func (pg *PDFGenerator) calculateColumnWidths(data *QueryData) []float64 {
	// Use landscape page width
	usableWidth := pageWidth - (2 * pg.config.MarginX) - 10 // ~800pt available

	numCols := len(data.Columns)
	if numCols == 0 {
		return []float64{}
	}

	// For many columns, use smaller minimum width
	minWidth := 35.0 // Reduced from 60.0
	maxWidth := 80.0 // Reduced from 150.0

	colWidths := make([]float64, numCols)

	// Calculate content-based widths
	for i, col := range data.Columns {
		// Start with header width (smaller font consideration)
		headerWidth := float64(len(col)*5) + 8 // Reduced multiplier

		// Check content width (sample first 3 rows for performance)
		maxContentWidth := headerWidth
		sampleSize := 3 // Reduced from 5 for performance
		if len(data.Rows) < sampleSize {
			sampleSize = len(data.Rows)
		}

		for j := 0; j < sampleSize; j++ {
			if val := data.Rows[j][col]; val != nil {
				contentStr := fmt.Sprintf("%v", val)
				contentWidth := float64(len(contentStr)*4) + 8 // Reduced multiplier
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

	// Scale down to fit available width
	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	if totalWidth > usableWidth {
		scale := usableWidth / totalWidth
		for i := range colWidths {
			colWidths[i] *= scale
			// Ensure still above minimum after scaling
			if colWidths[i] < minWidth/2 { // Allow even smaller after scaling
				colWidths[i] = minWidth / 2
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
	//pg.pdf.Cell(nil, fmt.Sprintf("📊 Total Queries: %d", queryCount))

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
	// PDF options with landscape-optimized config
	generatePDF := true
	pdfOutputFile := "query_results_landscape_split.pdf"
	pdfConfig := DefaultPDFConfig()
	pdfConfig.CompanyName = "Rahul's Database Reports"
	pdfConfig.Title = "SQL Query Execution Report (153 Columns)"
	pdfConfig.FontSize = 7          // Small font for readability
	pdfConfig.TableRowHeight = 15.0 // Compact rows
	pdfConfig.MarginX = 5.0         // Minimal margins
	pdfConfig.MarginY = 15.0

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
