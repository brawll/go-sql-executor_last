package exporters

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"go-sql-executor/models"

	"github.com/signintech/gopdf"
)

// PDFGenerator handles PDF creation for query results.
type PDFGenerator struct {
	pdf    *gopdf.GoPdf
	config PDFConfig
}

// DefaultPDFConfig returns default PDF configuration for wide horizontal layout.
func DefaultPDFConfig() PDFConfig {
	return PDFConfig{
		Title:          "SQL Query Results",
		Author:         "SQL Executor",
		Subject:        "Database Query Report",
		CompanyName:    "Your Company",
		HeaderColor:    []uint8{52, 73, 94},
		TableRowHeight: 20.0,
		FontSize:       10,
		MarginX:        20.0,
		MarginY:        20.0,
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
	if err := pg.tryWindowsFonts(); err == nil {
		return nil
	}
	if err := pg.tryMacOSFonts(); err == nil {
		return nil
	}
	if err := pg.tryLinuxFonts(); err == nil {
		return nil
	}
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

// savePDFToFile saves the PDF bytes to a file.
func SavePDFToFile(pdfBytes []byte, filename string) error {
	return os.WriteFile(filename, pdfBytes, 0644)
}

// addQueryResultWide adds a query result to the wide format page.
func (pg *PDFGenerator) addQueryResultWide(result models.QueryResult, queryNum int, startY, pageWidth float64) float64 {
	currentY := startY

	// Query header
	pg.pdf.SetFont("arial", "", 16)
	pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetXY(pg.config.MarginX, currentY)
	pg.pdf.Cell(nil, fmt.Sprintf("Query %d Results", queryNum))
	currentY += 25

	// Status line
	pg.pdf.SetFont("arial", "", 11)
	pg.pdf.SetTextColor(80, 80, 80)
	pg.pdf.SetXY(pg.config.MarginX, currentY)

	pg.pdf.Cell(nil, fmt.Sprintf("Status: %s | Time: %s",
		result.Status, result.Timestamp))
	currentY += 20

	// Show query if enabled
	if pg.config.ShowQuery {
		pg.pdf.SetFont("arial", "", 9)
		pg.pdf.SetTextColor(100, 100, 100)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		queryText := result.Query
		if len(queryText) > 150 {
			queryText = queryText[:147] + "..."
		}
		pg.pdf.Cell(nil, fmt.Sprintf("SQL: %s", queryText))
		currentY += 20
	}

	// Handle errors
	if result.Error != "" {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(220, 20, 20)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, fmt.Sprintf("❌ Error: %s", result.Error))
		return currentY + 20
	}

	// Handle empty results
	if result.Data == nil || len(result.Data.Rows) == 0 {
		pg.pdf.SetFont("arial", "", 10)
		pg.pdf.SetTextColor(100, 100, 100)
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, "📭 No data returned.")
		return currentY + 20
	}

	// Add the wide table
	currentY = pg.addWideTable(result.Data, currentY, pageWidth)

	return currentY
}

// GenerateWideHorizontalPDF -  with minimal wasted space
func (pg *PDFGenerator) GenerateWideHorizontalPDF(results []models.QueryResult) ([]byte, error) {
	// Find the result with the most data to calculate actual dimensions
	var largestResult *models.QueryResult
	maxRows := 0

	for _, result := range results {
		if result.Data != nil && len(result.Data.Rows) > maxRows {
			maxRows = len(result.Data.Rows)
			largestResult = &result
		}
	}

	if largestResult == nil || largestResult.Data == nil {
		// Fallback for no data
		totalWidth := 800.0
		totalHeight := 600.0

		pg.pdf = &gopdf.GoPdf{}
		pg.pdf.Start(gopdf.Config{
			PageSize: gopdf.Rect{W: totalWidth, H: totalHeight},
			Unit:     gopdf.UnitPT,
		})

		if err := pg.setupFonts(); err != nil {
			return nil, fmt.Errorf("failed to setup fonts: %w", err)
		}

		pg.pdf.AddPage()

		var buffer bytes.Buffer
		_, err := pg.pdf.WriteTo(&buffer)
		return buffer.Bytes(), err
	}

	// Calculate ACTUAL column widths for the largest dataset
	actualColWidths := pg.calculateWideColumnWidths(largestResult.Data, 0) // Pass 0 since we don't want to constrain

	// Calculate ACTUAL table width
	actualTableWidth := 0.0
	for _, width := range actualColWidths {
		actualTableWidth += width
	}

	// Page dimensions based on ACTUAL content
	headerHeight := 80.0
	rowHeight := pg.config.TableRowHeight
	extraMargin := 40.0 // Minimal extra space

	// Calculate page size based on actual content
	totalWidth := actualTableWidth + (2 * pg.config.MarginX) + extraMargin
	totalHeight := headerHeight + float64(maxRows)*rowHeight + (2 * pg.config.MarginY) + 100

	// Ensure reasonable minimums
	if totalWidth < 600 {
		totalWidth = 600
	}
	if totalHeight < 400 {
		totalHeight = 400
	}

	fmt.Printf("📏 Creating optimized wide page:\n")
	fmt.Printf("   📊 Actual table width: %.0f pts\n", actualTableWidth)
	fmt.Printf("   📄 Page dimensions: %.0f × %.0f pts (%.1f × %.1f inches)\n",
		totalWidth, totalHeight, totalWidth/72, totalHeight/72)
	fmt.Printf("   📐 Columns: %d, Rows: %d\n", len(largestResult.Data.Columns), maxRows)

	// Create PDF with optimized page size
	pg.pdf = &gopdf.GoPdf{}
	pg.pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: totalWidth, H: totalHeight},
		Unit:     gopdf.UnitPT,
	})

	// Set PDF metadata
	pg.pdf.SetInfo(gopdf.PdfInfo{
		Title:    pg.config.Title + " (Optimized Wide Format)",
		Author:   pg.config.Author,
		Subject:  pg.config.Subject + " - Compact Full Content",
		Creator:  "SQL Executor v2.0 - Optimized Wide Mode",
		Producer: "GoPDF Enhanced Wide",
	})

	// Setup fonts
	if err := pg.setupFonts(); err != nil {
		return nil, fmt.Errorf("failed to setup fonts: %w", err)
	}

	pg.pdf.AddPage()

	// Add all query results on the single optimized page
	currentY := pg.config.MarginY
	for i, result := range results {
		currentY = pg.addQueryResultWide(result, i+1, currentY, totalWidth)
		currentY += 30 // Reasonable space between queries
	}

	// Convert to bytes
	var buffer bytes.Buffer
	_, err := pg.pdf.WriteTo(&buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to write PDF: %w", err)
	}

	return buffer.Bytes(), nil
}

// calculateWideColumnWidths - for exact width calculation
func (pg *PDFGenerator) calculateWideColumnWidths(data *models.QueryData, pageWidth float64) []float64 {
	numCols := len(data.Columns)
	if numCols == 0 {
		return []float64{}
	}

	colWidths := make([]float64, numCols)
	minWidth := 60.0  // Reasonable minimum
	maxWidth := 300.0 // Reasonable maximum to prevent excessive width

	// Calculate content-based widths precisely
	for i, col := range data.Columns {
		// Header width - precise calculation
		headerWidth := float64(len(col)*6) + 12 // 6pts per char + padding

		// Check ALL rows for maximum content width
		maxContentWidth := headerWidth

		for _, row := range data.Rows {
			if val := row[col]; val != nil {
				contentStr := fmt.Sprintf("%v", val)
				// Precise character width calculation
				contentWidth := float64(len(contentStr)*6) + 12 // 6pts per char + padding
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

	return colWidths
}

// addWideTable - with precise width usage
func (pg *PDFGenerator) addWideTable(data *models.QueryData, startY, pageWidth float64) float64 {
	if len(data.Columns) == 0 || len(data.Rows) == 0 {
		return startY
	}

	// Calculate column widths - don't constrain by pageWidth for wide format
	colWidths := pg.calculateWideColumnWidths(data, 0)
	currentY := startY

	// Calculate actual table width
	actualTableWidth := 0.0
	for _, width := range colWidths {
		actualTableWidth += width
	}

	fmt.Printf("📊 Table: %d columns × %d rows | Actual width: %.0f pts\n",
		len(data.Columns), len(data.Rows), actualTableWidth)

	// Draw table headers
	currentY = pg.drawWideTableHeaders(data.Columns, colWidths, currentY)

	// Draw all rows
	currentY = pg.drawWideTableRows(data, colWidths, currentY)

	return currentY
}

// drawWideTableRows draws all table rows for wide layout - NO TRUNCATION VERSION.
func (pg *PDFGenerator) drawWideTableRows(data *models.QueryData, colWidths []float64, startY float64) float64 {
	if len(data.Rows) == 0 {
		return startY
	}

	rowHeight := pg.config.TableRowHeight
	currentY := startY

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
		// Alternate row colors
		if rowIdx%2 == 1 {
			pg.pdf.SetFillColor(248, 249, 250)
			pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, currentY, totalWidth, rowHeight, "F")
		}

		// Row border
		pg.pdf.Line(pg.config.MarginX, currentY+rowHeight, pg.config.MarginX+totalWidth, currentY+rowHeight)

		// Cell content
		currentX := pg.config.MarginX
		for i, col := range data.Columns {
			if i > 0 {
				pg.pdf.Line(currentX, currentY, currentX, currentY+rowHeight)
			}

			// Cell value
			cellValue := "NULL"
			if val := row[col]; val != nil {
				cellValue = fmt.Sprintf("%v", val)
			}

			// NO TRUNCATION - Display full content
			// Just add the text with padding, no length checking
			pg.pdf.SetXY(currentX+5, currentY+6) // Increased left padding
			pg.pdf.Cell(nil, cellValue)          // Full content, no truncation

			currentX += colWidths[i]
		}

		currentY += rowHeight
	}

	// Final borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX, currentY)
	pg.pdf.Line(pg.config.MarginX+totalWidth, startY, pg.config.MarginX+totalWidth, currentY)

	return currentY
}

// drawWideTableHeaders draws table headers for wide layout - NO TRUNCATION VERSION.
func (pg *PDFGenerator) drawWideTableHeaders(columns []string, colWidths []float64, startY float64) float64 {
	headerHeight := pg.config.TableRowHeight + 8 // Increased height for better readability

	// Header background
	pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetTextColor(255, 255, 255)
	pg.pdf.SetFont("arial", "", pg.config.FontSize+2)

	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, startY, totalWidth, headerHeight, "F")

	// Header borders
	pg.pdf.SetStrokeColor(100, 100, 100)
	pg.pdf.SetLineWidth(0.5)

	currentX := pg.config.MarginX
	for i, col := range columns {
		if i > 0 {
			pg.pdf.Line(currentX, startY, currentX, startY+headerHeight)
		}

		// Header text - NO TRUNCATION
		pg.pdf.SetXY(currentX+5, startY+10) // Increased padding
		pg.pdf.Cell(nil, col)               // Full column name, no truncation
		currentX += colWidths[i]
	}

	// Horizontal borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX+totalWidth, startY)
	pg.pdf.Line(pg.config.MarginX, startY+headerHeight, pg.config.MarginX+totalWidth, startY+headerHeight)

	return startY + headerHeight
}
