package exporters

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"go-sql-executor/models"

	"github.com/signintech/gopdf"
)

// PDFGenerator handles PDF creation for query results.
type PDFGenerator struct {
	pdf    *gopdf.GoPdf
	config models.PDFConfig
}

// DefaultPDFConfig returns default PDF configuration for wide horizontal layout.
func DefaultPDFConfig() models.PDFConfig {
	return models.PDFConfig{
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
func NewPDFGenerator(config models.PDFConfig) *PDFGenerator {
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

// calculateContactWidth estimates the width needed for contact details
func (pg *PDFGenerator) calculateContactWidth() float64 {
	if !pg.config.ContactEnabled {
		return 0
	}

	maxWidth := 0.0
	charWidth := 5.0 // Approximate character width

	// Check each contact field and find the longest one
	if pg.config.ContactName != nil && *pg.config.ContactName != "" {
		width := float64(len(*pg.config.ContactName)) * charWidth
		if width > maxWidth {
			maxWidth = width
		}
	}

	if pg.config.ContactEmail != nil && *pg.config.ContactEmail != "" {
		width := float64(len(*pg.config.ContactEmail)) * charWidth
		if width > maxWidth {
			maxWidth = width
		}
	}

	if pg.config.ContactPhone != nil && *pg.config.ContactPhone != "" {
		width := float64(len(*pg.config.ContactPhone)) * charWidth
		if width > maxWidth {
			maxWidth = width
		}
	}

	// For address, use a fixed reasonable width since it can wrap
	if pg.config.ContactAddress != nil && *pg.config.ContactAddress != "" {
		addressWidth := 140.0 // Smaller fixed width for address
		if addressWidth > maxWidth {
			maxWidth = addressWidth
		}
	}

	// Add minimal padding and set reasonable bounds
	maxWidth += 10 // Reduced padding
	if maxWidth < 100 {
		maxWidth = 100
	}
	if maxWidth > 160 { // Reduced maximum width
		maxWidth = 160
	}

	log.Printf("📐 Calculated contact width: %.1f pts", maxWidth)
	return maxWidth
}

// getHorizontalPosition calculates X position based on alignment
func (pg *PDFGenerator) getHorizontalPosition(position string, leftPos, centerPos, rightPos, elementWidth float64) float64 {
	switch position {
	case "left":
		return leftPos
	case "right":
		// For right alignment, subtract element width from right position
		return rightPos - elementWidth
	case "center":
		return centerPos - (elementWidth / 2)
	default:
		return leftPos
	}
}

// addTemplateHeader adds a customizable header with logo, title, and contact details
func (pg *PDFGenerator) addTemplateHeader(startY, pageWidth float64) float64 {
	log.Printf("pageWidth: %v", pageWidth)
	currentY := startY
	headerHeight := pg.config.HeaderHeight

	log.Printf("🏗️  Building header: pageWidth=%.1f, headerHeight=%.1f", pageWidth, headerHeight)

	// Determine positions - make right position much closer to edge
	leftPos := pg.config.MarginX + 5           // Small padding from margin
	centerPos := pageWidth / 2                 // Simple center calculation
	rightEdge := pageWidth - pg.config.MarginX // Actual right edge considering margin

	log.Printf("📍 Positions: left=%.1f, center=%.1f, rightEdge=%.1f", leftPos, centerPos, rightEdge)

	logoY := currentY + 10
	titleY := currentY + 15
	contactY := currentY + 10

	// Track what's in the center to adjust title positioning
	centerOccupied := false

	// Add logo if configured
	if pg.config.LogoPath != nil {
		logoX := pg.getHorizontalPosition(pg.config.LogoPosition, leftPos, centerPos, rightEdge, pg.config.LogoWidth)
		err := pg.addBase64Logo(logoX, logoY)
		if err != nil {
			log.Printf("Warning: Failed to add logo: %v", err)
		}

		// Mark center as occupied if logo is there
		if pg.config.LogoPosition == "center" {
			centerOccupied = true
		}
	}

	// Add contact details if enabled (do this before title to check center occupation)
	if pg.config.ContactEnabled {
		// Calculate actual contact width needed
		contactWidth := pg.calculateActualContactWidth()
		log.Printf("📐 Calculated actual contact width: %.1f pts", contactWidth)

		// For right positioning, position so the contact block ends at rightEdge - small padding
		var contactX float64
		if pg.config.ContactPosition == "right" {
			contactX = rightEdge - contactWidth - 10 // 10pts padding from right edge
			log.Printf("📍 Right-aligned contact X position: %.1f (rightEdge=%.1f - width=%.1f - padding=10)",
				contactX, rightEdge, contactWidth)
		} else {
			contactX = pg.getHorizontalPosition(pg.config.ContactPosition, leftPos, centerPos, rightEdge, contactWidth)
			log.Printf("📍 %s-aligned contact X position: %.1f", pg.config.ContactPosition, contactX)
		}

		pg.addContactDetails(contactX, contactY)

		// Mark center as occupied if contact is there
		if pg.config.ContactPosition == "center" {
			centerOccupied = true
		}
	}

	// ALWAYS add report title if we have title text
	titleText := ""
	if pg.config.ReportTitleText != nil {
		titleText = *pg.config.ReportTitleText
	} else if pg.config.Title != "" {
		titleText = pg.config.Title
	}

	if titleText != "" {
		if centerOccupied {
			// Move title below the center element
			titleY = logoY + pg.config.LogoHeight + 15
		}
		pg.addReportTitle(titleText, titleY, "center", pageWidth)
		log.Printf("📝 Report title should be added: '%s'", titleText)
	} else {
		log.Printf("⚠️  No report title found: ReportTitleText=%v, Title='%s'", pg.config.ReportTitleText, pg.config.Title)
	}

	return currentY + headerHeight + 10
}

// calculateActualContactWidth calculates the ACTUAL width needed based on content
func (pg *PDFGenerator) calculateActualContactWidth() float64 {
	if !pg.config.ContactEnabled {
		return 0
	}

	maxLineWidth := 0.0
	charWidth := 4.5      // More accurate character width for size 9 font
	maxCharsPerLine := 30 // Maximum characters we want per line

	// Check each contact field
	contactLines := []string{}

	if pg.config.ContactName != nil && *pg.config.ContactName != "" {
		contactLines = append(contactLines, *pg.config.ContactName)
	}

	if pg.config.ContactAddress != nil && *pg.config.ContactAddress != "" {
		// Split address into reasonable lines
		address := *pg.config.ContactAddress
		words := strings.Fields(address)
		currentLine := ""

		for _, word := range words {
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			if len(testLine) <= maxCharsPerLine {
				currentLine = testLine
			} else {
				if currentLine != "" {
					contactLines = append(contactLines, currentLine)
					currentLine = word
				}
			}
		}
		if currentLine != "" {
			contactLines = append(contactLines, currentLine)
		}
	}

	if pg.config.ContactEmail != nil && *pg.config.ContactEmail != "" {
		contactLines = append(contactLines, *pg.config.ContactEmail)
	}

	if pg.config.ContactPhone != nil && *pg.config.ContactPhone != "" {
		contactLines = append(contactLines, *pg.config.ContactPhone)
	}

	// Find the longest line
	for _, line := range contactLines {
		lineWidth := float64(len(line)) * charWidth
		if lineWidth > maxLineWidth {
			maxLineWidth = lineWidth
		}
		log.Printf("📏 Contact line: '%s' = %.1f pts", line, lineWidth)
	}

	// Add padding
	actualWidth := maxLineWidth + 15 // Reasonable padding

	// Set reasonable bounds
	if actualWidth < 80 {
		actualWidth = 80
	}
	if actualWidth > 180 {
		actualWidth = 180
	}

	log.Printf("📐 Final contact width: %.1f pts (longest line: %.1f pts)", actualWidth, maxLineWidth)
	return actualWidth
}

// addBase64Logo handles base64 encoded logo data
func (pg *PDFGenerator) addBase64Logo(x, y float64) error {

	// Parse the data URL: data:image/png;base64,iVBw0...
	dataURL := *pg.config.LogoPath
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format")
	}

	// Extract MIME type to determine file extension
	header := parts[0]

	base64Data := parts[1]

	var fileExt string
	if strings.Contains(header, "image/png") {
		fileExt = ".png"
	} else if strings.Contains(header, "image/jpeg") || strings.Contains(header, "image/jpg") {
		fileExt = ".jpg"
	} else if strings.Contains(header, "image/gif") {
		fileExt = ".gif"
	} else {
		// Default to PNG if format is unclear
		fileExt = ".png"
		log.Printf("Unknown image format in data URL, defaulting to PNG: %s", header)
	}

	// Decode base64 data
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 logo data: %w", err)
	}

	// Create temporary file for the logo
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "pdf_logo_*"+fileExt)
	if err != nil {
		return fmt.Errorf("failed to create temporary logo file: %w", err)
	}
	defer tempFile.Close()

	tempFilePath := tempFile.Name()

	// Write decoded data to temporary file
	if _, err := tempFile.Write(imageData); err != nil {
		os.Remove(tempFilePath) // Clean up on error
		return fmt.Errorf("failed to write logo data to temporary file: %w", err)
	}

	// Close the file so it can be read by the PDF library
	tempFile.Close()

	// Add image to PDF using temporary file
	err = pg.pdf.Image(tempFilePath, x, y, &gopdf.Rect{
		W: pg.config.LogoWidth,
		H: pg.config.LogoHeight,
	})

	// Clean up temporary file
	os.Remove(tempFilePath)

	if err != nil {
		return fmt.Errorf("failed to add base64 logo to PDF: %w", err)
	}

	log.Printf("✅ Base64 logo added successfully at position (%.1f, %.1f)", x, y)
	return nil
}

// addReportTitle adds the report title with custom styling
func (pg *PDFGenerator) addReportTitle(title string, y float64, position string, pageWidth float64) {
	if title == "" {
		log.Printf("❌ Report title is empty, skipping")
		return
	}

	log.Printf("📝 Adding report title: '%s' at Y=%.1f", title, y)

	pg.pdf.SetFont("arial", "", 16) // Good size for title
	pg.pdf.SetTextColor(pg.config.ReportTitleColor[0], pg.config.ReportTitleColor[1], pg.config.ReportTitleColor[2])

	var x float64
	// More accurate text width calculation
	charWidth := 9.0 // Approximate character width for size 16 font
	textWidth := float64(len(title)) * charWidth

	switch position {
	case "left":
		x = pg.config.MarginX + 10
	case "right":
		x = pageWidth - pg.config.MarginX - textWidth - 10
	default:
		// Center the title
		x = (pageWidth - textWidth) / 2
	}

	log.Printf("📍 Title position calculated: X=%.1f, Y=%.1f", x, y)

	pg.pdf.SetXY(x, y)
	pg.pdf.Cell(nil, title)

	log.Printf("✅ Report title added successfully: '%s' at position (%.1f, %.1f)", title, x, y)
}

// addContactDetails adds contact information with proper line breaks
func (pg *PDFGenerator) addContactDetails(x, y float64) {
	if !pg.config.ContactEnabled {
		log.Printf("❌ Contact details disabled")
		return
	}

	log.Printf("📞 Adding contact details at X=%.1f, Y=%.1f", x, y)

	pg.pdf.SetFont("arial", "", 10) // Smaller font for contact details
	pg.pdf.SetTextColor(pg.config.ContactTextColor[0], pg.config.ContactTextColor[1], pg.config.ContactTextColor[2])

	currentY := y
	lineHeight := 11.0 // Reduced line height
	maxWidth := 140.0  // Reduced maximum width for contact details

	// Contact name
	if pg.config.ContactName != nil && *pg.config.ContactName != "" {
		log.Printf("  📝 Contact name: '%s' at X=%.1f, Y=%.1f", *pg.config.ContactName, x, currentY)
		currentY = pg.addMultilineText(x, currentY, *pg.config.ContactName, maxWidth, lineHeight)
		currentY += 2 // Small gap between fields
	}

	// Contact address (can be long and multiline)
	if pg.config.ContactAddress != nil && *pg.config.ContactAddress != "" {
		log.Printf("  📍 Contact address: '%s' at X=%.1f, Y=%.1f", *pg.config.ContactAddress, x, currentY)
		currentY = pg.addMultilineText(x, currentY, *pg.config.ContactAddress, maxWidth, lineHeight)
		currentY += 2
	}

	// Contact email
	if pg.config.ContactEmail != nil && *pg.config.ContactEmail != "" {
		log.Printf("  📧 Contact email: '%s' at X=%.1f, Y=%.1f", *pg.config.ContactEmail, x, currentY)
		currentY = pg.addMultilineText(x, currentY, *pg.config.ContactEmail, maxWidth, lineHeight)
		currentY += 2
	}

	// Contact phone
	if pg.config.ContactPhone != nil && *pg.config.ContactPhone != "" {
		log.Printf("  📱 Contact phone: '%s' at X=%.1f, Y=%.1f", *pg.config.ContactPhone, x, currentY)
		pg.pdf.SetXY(x, currentY)
		pg.pdf.Cell(nil, *pg.config.ContactPhone)
	}

	log.Printf("✅ Contact details positioning complete")
}

// addMultilineText handles long text by breaking it into multiple lines
func (pg *PDFGenerator) addMultilineText(x, y float64, text string, maxWidth, lineHeight float64) float64 {
	if text == "" {
		return y
	}

	log.Printf("📝 Adding multiline text at X=%.1f, Y=%.1f: '%s' (maxWidth=%.1f)", x, y, text, maxWidth)

	// Estimate characters per line based on font size and available width
	charWidth := 4.5 // More accurate character width for size 9 font
	charsPerLine := int(maxWidth / charWidth)

	if charsPerLine < 10 {
		charsPerLine = 10 // Minimum characters per line
	}

	words := strings.Fields(text)
	var lines []string
	var currentLine string

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) <= charsPerLine {
			currentLine = testLine
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				// Word is too long, break it
				for len(word) > charsPerLine {
					lines = append(lines, word[:charsPerLine])
					word = word[charsPerLine:]
				}
				currentLine = word
			}
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Draw each line
	currentY := y
	for i, line := range lines {
		log.Printf("  📄 Line %d at X=%.1f, Y=%.1f: '%s'", i+1, x, currentY, line)
		pg.pdf.SetXY(x, currentY)
		pg.pdf.Cell(nil, line)
		currentY += lineHeight
	}

	return currentY
}

// addQueryResultWide adds a query result to the wide format page.
func (pg *PDFGenerator) addQueryResultWide(result models.QueryResult, queryNum int, startY, pageWidth float64) float64 {
	currentY := startY

	// Only show query header if no template header is configured
	if pg.config.ReportTitleText == nil {
		// Query header
		pg.pdf.SetFont("arial", "", 16)
		pg.pdf.SetTextColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
		pg.pdf.SetXY(pg.config.MarginX, currentY)
		pg.pdf.Cell(nil, fmt.Sprintf("Query %d Results", queryNum))
		currentY += 25
	}

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

	// Add template header first
	currentY := pg.config.MarginY
	if pg.config.LogoPath != nil || pg.config.ContactEnabled || pg.config.ReportTitleText != nil {
		currentY = pg.addTemplateHeader(currentY, totalWidth)
	}

	// Add all query results on the single optimized page

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

// drawWideTableRows draws all table rows for wide layout with proper text wrapping.
func (pg *PDFGenerator) drawWideTableRows(data *models.QueryData, colWidths []float64, startY float64) float64 {
	if len(data.Rows) == 0 {
		return startY
	}

	currentY := startY
	lineHeight := float64(pg.config.FontSize + 2)

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
		rowStartY := currentY
		rowHeightUsed := pg.config.TableRowHeight

		// Calculate required row height based on text wrapping
		for i, col := range data.Columns {
			cellValue := "NULL"
			if val := row[col]; val != nil {
				cellValue = fmt.Sprintf("%v", val)
			}

			availWidth := colWidths[i] - 10
			charWidth := 6.0
			if availWidth > charWidth {
				charsPerLine := availWidth / charWidth
				numLines := (len(cellValue) / int(charsPerLine)) + 1
				cellHeight := float64(numLines) * lineHeight
				if cellHeight > rowHeightUsed {
					rowHeightUsed = cellHeight
				}
			}
		}

		// Draw row background
		if rowIdx%2 == 1 {
			pg.pdf.SetFillColor(248, 249, 250)
			pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, rowStartY, totalWidth, rowHeightUsed, "F")
		}

		// Draw row borders
		pg.pdf.Line(pg.config.MarginX, rowStartY+rowHeightUsed, pg.config.MarginX+totalWidth, rowStartY+rowHeightUsed)

		// Draw vertical borders
		currentX := pg.config.MarginX
		for i := 0; i < len(data.Columns); i++ {
			if i > 0 {
				pg.pdf.Line(currentX, rowStartY, currentX, rowStartY+rowHeightUsed)
			}
			currentX += colWidths[i]
		}

		// Draw cell content with MultiCell for proper wrapping
		currentX = pg.config.MarginX
		for i, col := range data.Columns {
			cellValue := "NULL"
			if val := row[col]; val != nil {
				cellValue = fmt.Sprintf("%v", val)
			}

			pg.pdf.SetXY(currentX+5, rowStartY+6)
			pg.pdf.MultiCell(&gopdf.Rect{W: colWidths[i] - 10, H: rowHeightUsed}, cellValue)
			currentX += colWidths[i]
		}

		currentY += rowHeightUsed
	}

	// Final borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX, currentY)
	pg.pdf.Line(pg.config.MarginX+totalWidth, startY, pg.config.MarginX+totalWidth, currentY)

	return currentY
}

// drawWideTableHeaders draws table headers for wide layout with proper text wrapping.
func (pg *PDFGenerator) drawWideTableHeaders(columns []string, colWidths []float64, startY float64) float64 {
	headerBaseHeight := pg.config.TableRowHeight + 8 // Base height for better readability
	lineHeight := float64(pg.config.FontSize + 2)    // Same as font size + padding

	// Calculate required header height based on text wrapping
	headerHeightUsed := headerBaseHeight
	for i, col := range columns {
		availWidth := colWidths[i] - 10
		charWidth := 6.0
		if availWidth > charWidth {
			charsPerLine := availWidth / charWidth
			numLines := (len(col) / int(charsPerLine)) + 1
			headerHeight := float64(numLines)*lineHeight + 8 // Include padding
			if headerHeight > headerHeightUsed {
				headerHeightUsed = headerHeight
			}
		}
	}

	// Header background
	pg.pdf.SetFillColor(pg.config.HeaderColor[0], pg.config.HeaderColor[1], pg.config.HeaderColor[2])
	pg.pdf.SetTextColor(255, 255, 255)
	pg.pdf.SetFont("arial", "", pg.config.FontSize+2)

	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	pg.pdf.RectFromUpperLeftWithStyle(pg.config.MarginX, startY, totalWidth, headerHeightUsed, "F")

	// Header borders
	pg.pdf.SetStrokeColor(100, 100, 100)
	pg.pdf.SetLineWidth(0.5)

	currentX := pg.config.MarginX
	for i := 0; i < len(columns); i++ {
		if i > 0 {
			pg.pdf.Line(currentX, startY, currentX, startY+headerHeightUsed)
		}
		currentX += colWidths[i]
	}

	// Draw header text with MultiCell for proper wrapping
	currentX = pg.config.MarginX
	for i, col := range columns {
		pg.pdf.SetXY(currentX+5, startY+10) // Increased padding
		pg.pdf.MultiCell(&gopdf.Rect{W: colWidths[i] - 10, H: headerHeightUsed - 10}, col)
		currentX += colWidths[i]
	}

	// Horizontal borders
	pg.pdf.Line(pg.config.MarginX, startY, pg.config.MarginX+totalWidth, startY)
	pg.pdf.Line(pg.config.MarginX, startY+headerHeightUsed, pg.config.MarginX+totalWidth, startY+headerHeightUsed)

	return startY + headerHeightUsed
}
