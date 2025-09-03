package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/signintech/gopdf"
	"github.com/xuri/excelize/v2"
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

// Add this after your existing structs
// HTMLConfig holds HTML generation configuration.
type HTMLConfig struct {
	Title         string
	CompanyName   string
	HeaderColor   string // CSS color
	ShowTimestamp bool
	ShowQuery     bool
	Theme         string // "light" or "dark"
}

// DefaultHTMLConfig returns default HTML configuration.
func DefaultHTMLConfig() HTMLConfig {
	return HTMLConfig{
		Title:         "SQL Query Results",
		CompanyName:   "Your Company",
		HeaderColor:   "#34495e",
		ShowTimestamp: true,
		ShowQuery:     true,
		Theme:         "light",
	}
}

// Add the HTML template constant
const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background-color: {{if eq .Theme "dark"}}#1a1a1a{{else}}#f8f9fa{{end}};
            color: {{if eq .Theme "dark"}}#e0e0e0{{else}}#333{{end}};
            line-height: 1.6;
        }
        
        .container {
            max-width: 100%;
            margin: 0 auto;
            padding: 20px;
        }
        
        .header {
            background: linear-gradient(135deg, {{.HeaderColor}}, {{.HeaderColor}}dd);
            color: white;
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        
        .header h1 {
            font-size: 2.5rem;
            margin-bottom: 10px;
            font-weight: 700;
        }
        
        .header .company {
            font-size: 1.2rem;
            opacity: 0.9;
        }
        
        .header .meta {
            margin-top: 15px;
            font-size: 0.95rem;
            opacity: 0.8;
        }
        
        .query-section {
            background: {{if eq .Theme "dark"}}#2d2d2d{{else}}white{{end}};
            border-radius: 10px;
            padding: 25px;
            margin-bottom: 30px;
            box-shadow: 0 2px 10px rgba(0,0,0,{{if eq .Theme "dark"}}0.3{{else}}0.1{{end}});
            border: 1px solid {{if eq .Theme "dark"}}#404040{{else}}#e0e0e0{{end}};
        }
        
        .query-header {
            display: flex;
            align-items: center;
            margin-bottom: 20px;
        }
        
        .query-header h2 {
            color: {{.HeaderColor}};
            font-size: 1.5rem;
            margin-right: 15px;
        }
        
        .status-badge {
            padding: 5px 12px;
            border-radius: 20px;
            font-size: 0.85rem;
            font-weight: 600;
        }
        
        .status-success {
            background-color: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        
        .status-error {
            background-color: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        
        .query-info {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
            font-size: 0.9rem;
        }
        
        .info-item {
            background: {{if eq .Theme "dark"}}#3a3a3a{{else}}#f8f9fa{{end}};
            padding: 10px 15px;
            border-radius: 5px;
            border-left: 4px solid {{.HeaderColor}};
        }
        
        .info-label {
            font-weight: 600;
            color: {{.HeaderColor}};
        }
        
        .table-container {
            overflow-x: auto;
            border: 1px solid {{if eq .Theme "dark"}}#404040{{else}}#ddd{{end}};
            border-radius: 8px;
            background: {{if eq .Theme "dark"}}#2a2a2a{{else}}white{{end}};
        }
        
        .data-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.9rem;
            min-width: 100%;
        }
        
        .data-table th {
            background: {{.HeaderColor}};
            color: white;
            padding: 12px 8px;
            text-align: left;
            font-weight: 600;
            white-space: nowrap;
            border-right: 1px solid rgba(255,255,255,0.2);
            position: sticky;
            top: 0;
            z-index: 10;
        }
        
        .data-table th:last-child {
            border-right: none;
        }
        
        .data-table td {
            padding: 10px 8px;
            border-bottom: 1px solid {{if eq .Theme "dark"}}#404040{{else}}#eee{{end}};
            border-right: 1px solid {{if eq .Theme "dark"}}#404040{{else}}#eee{{end}};
            white-space: nowrap;
            max-width: 200px;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        
        .data-table td:last-child {
            border-right: none;
        }
        
        .data-table tbody tr:hover {
            background-color: {{if eq .Theme "dark"}}#404040{{else}}#f8f9fa{{end}};
        }
        
        .data-table tbody tr:nth-child(even) {
            background-color: {{if eq .Theme "dark"}}#333{{else}}#f9f9f9{{end}};
        }
        
        .data-table tbody tr:nth-child(even):hover {
            background-color: {{if eq .Theme "dark"}}#454545{{else}}#f0f0f0{{end}};
        }
        
        .no-data {
            text-align: center;
            padding: 40px;
            color: {{if eq .Theme "dark"}}#888{{else}}#666{{end}};
            font-style: italic;
        }
        
        .error-message {
            background-color: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
            padding: 15px;
            border-radius: 5px;
            margin: 15px 0;
        }
        
        .scroll-hint {
            background: {{if eq .Theme "dark"}}#404040{{else}}#e3f2fd{{end}};
            color: {{if eq .Theme "dark"}}#ccc{{else}}#1976d2{{end}};
            padding: 10px;
            border-radius: 5px;
            margin-bottom: 10px;
            font-size: 0.9rem;
            text-align: center;
        }
        
        @media (max-width: 768px) {
            .container {
                padding: 10px;
            }
            
            .header h1 {
                font-size: 2rem;
            }
            
            .query-info {
                grid-template-columns: 1fr;
            }
        }
        
        .table-container::-webkit-scrollbar {
            height: 8px;
        }
        
        .table-container::-webkit-scrollbar-track {
            background: {{if eq .Theme "dark"}}#1a1a1a{{else}}#f1f1f1{{end}};
        }
        
        .table-container::-webkit-scrollbar-thumb {
            background: {{.HeaderColor}};
            border-radius: 4px;
        }
        
        .table-container::-webkit-scrollbar-thumb:hover {
            background: {{.HeaderColor}}cc;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            <div class="company">{{.CompanyName}}</div>
            {{if .ShowTimestamp}}
            <div class="meta">Generated on {{.GeneratedAt}}</div>
            {{end}}
        </div>
        
        {{range $index, $result := .Results}}
        <div class="query-section">
            <div class="query-header">
                <h2>Query {{add $index 1}} Results</h2>
                <span class="status-badge {{if eq $result.Status "success"}}status-success{{else}}status-error{{end}}">
                    {{if eq $result.Status "success"}}✅{{else}}❌{{end}} {{$result.Status}}
                </span>
            </div>
            
            <div class="query-info">
                <div class="info-item">
                    <div class="info-label">Timestamp</div>
                    <div>{{$result.Timestamp}}</div>
                </div>
                {{if $result.Data}}
                <div class="info-item">
                    <div class="info-label">Columns</div>
                    <div>{{len $result.Data.Columns}}</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Rows</div>
                    <div>{{len $result.Data.Rows}}</div>
                </div>
                {{end}}
            </div>
            
            {{if $result.Error}}
            <div class="error-message">
                <strong>Error:</strong> {{$result.Error}}
            </div>
            {{else if not $result.Data}}
            <div class="no-data">No data returned</div>
            {{else if eq (len $result.Data.Rows) 0}}
            <div class="no-data">No rows returned</div>
            {{else}}
            <div class="scroll-hint">
                📏 Scroll horizontally to see all {{len $result.Data.Columns}} columns
            </div>
            <div class="table-container">
                <table class="data-table">
                    <thead>
                        <tr>
                            {{range $result.Data.Columns}}
                            <th>{{.}}</th>
                            {{end}}
                        </tr>
                    </thead>
                    <tbody>
                        {{range $rowIndex, $row := $result.Data.Rows}}
                        <tr>
                            {{range $colName := $result.Data.Columns}}
                            <td>{{if index $row $colName}}{{index $row $colName}}{{else}}NULL{{end}}</td>
                            {{end}}
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>
`

// Simple HTML generation function
func GenerateHTML(config HTMLConfig, results []QueryResult) ([]byte, error) {
	// Create template with the config and results data
	data := struct {
		HTMLConfig
		Results     []QueryResult
		GeneratedAt string
	}{
		HTMLConfig:  config,
		Results:     results,
		GeneratedAt: time.Now().Format("January 2, 2006 at 3:04 PM"),
	}

	// Parse and execute template
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buffer.Bytes(), nil
}

// saveHTMLToFile saves the HTML bytes to a file.
func saveHTMLToFile(htmlBytes []byte, filename string) error {
	return os.WriteFile(filename, htmlBytes, 0644)
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

type CSVExporter struct {
	IncludeQueryInfo bool
	Separator        rune
}

// NewCSVExporter creates a new CSV exporter with default settings
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{
		IncludeQueryInfo: true,
		Separator:        ',',
	}
}

// ExportToCSV exports query results to a CSV file
func (ce *CSVExporter) ExportToCSV(results []QueryResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if ce.Separator != ',' {
		writer.Comma = ce.Separator
	}
	defer writer.Flush()

	for queryIndex, result := range results {
		// Add query information header
		if ce.IncludeQueryInfo {
			if err := ce.writeQueryHeader(writer, result); err != nil {
				return fmt.Errorf("failed to write query header: %w", err)
			}
		}

		// Handle errors
		if result.Error != "" {
			if err := writer.Write([]string{"ERROR", result.Error}); err != nil {
				return fmt.Errorf("failed to write error: %w", err)
			}
			continue
		}

		// Handle empty results
		if result.Data == nil || len(result.Data.Rows) == 0 {
			if err := writer.Write([]string{"INFO", "No data returned"}); err != nil {
				return fmt.Errorf("failed to write no data message: %w", err)
			}
			continue
		}

		// Write column headers
		if err := writer.Write(result.Data.Columns); err != nil {
			return fmt.Errorf("failed to write column headers: %w", err)
		}

		// Write data rows
		for _, row := range result.Data.Rows {
			csvRow := make([]string, len(result.Data.Columns))
			for i, col := range result.Data.Columns {
				if val := row[col]; val != nil {
					csvRow[i] = fmt.Sprintf("%v", val)
				} else {
					csvRow[i] = ""
				}
			}
			if err := writer.Write(csvRow); err != nil {
				return fmt.Errorf("failed to write data row: %w", err)
			}
		}

		// Add separator between queries if there are multiple
		if len(results) > 1 && queryIndex < len(results)-1 {
			if err := writer.Write([]string{}); err != nil {
				return fmt.Errorf("failed to write separator: %w", err)
			}
		}
	}

	return nil
}

// writeQueryHeader writes query information to CSV
func (ce *CSVExporter) writeQueryHeader(writer *csv.Writer, result QueryResult) error {
	// Query text
	if err := writer.Write([]string{"SQL", result.Query}); err != nil {
		return err
	}

	// Empty line for separation
	return writer.Write([]string{})
}

// Simple Excel export function
func ExportToExcel(result QueryResult, filename string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"

	// Handle errors
	if result.Error != "" {
		f.SetCellValue(sheetName, "A1", "ERROR")
		f.SetCellValue(sheetName, "B1", result.Error)
		return f.SaveAs(filename)
	}

	// Handle empty results
	if result.Data == nil || len(result.Data.Rows) == 0 {
		f.SetCellValue(sheetName, "A1", "No data returned")
		return f.SaveAs(filename)
	}

	// Write headers (row 1)
	for i, col := range result.Data.Columns {
		cell := fmt.Sprintf("%s1", columnName(i))
		f.SetCellValue(sheetName, cell, col)
	}

	// Write data (starting from row 2)
	for rowIdx, row := range result.Data.Rows {
		for colIdx, col := range result.Data.Columns {
			cell := fmt.Sprintf("%s%d", columnName(colIdx), rowIdx+2)

			if val := row[col]; val != nil {
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", val))
			}
		}
	}

	return f.SaveAs(filename)
}

// Helper function to convert column index to Excel column name
func columnName(index int) string {
	result := ""
	for index >= 0 {
		result = string(rune('A'+index%26)) + result
		index = index/26 - 1
	}
	return result
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

// addQueryResultWide adds a query result to the wide format page.
func (pg *PDFGenerator) addQueryResultWide(result QueryResult, queryNum int, startY, pageWidth float64) float64 {
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

// GenerateWideHorizontalPDF - FIXED VERSION with minimal wasted space
func (pg *PDFGenerator) GenerateWideHorizontalPDF(results []QueryResult) ([]byte, error) {
	// Find the result with the most data to calculate actual dimensions
	var largestResult *QueryResult
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
		err := pg.pdf.Write(&buffer)
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
	err := pg.pdf.Write(&buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to write PDF: %w", err)
	}

	return buffer.Bytes(), nil
}

// calculateWideColumnWidths - OPTIMIZED VERSION for exact width calculation
func (pg *PDFGenerator) calculateWideColumnWidths(data *QueryData, pageWidth float64) []float64 {
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

// addWideTable - OPTIMIZED VERSION with precise width usage
func (pg *PDFGenerator) addWideTable(data *QueryData, startY, pageWidth float64) float64 {
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
func (pg *PDFGenerator) drawWideTableRows(data *QueryData, colWidths []float64, startY float64) float64 {
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

		fmt.Printf("Columns: %d, Rows: %d\n", len(result.Data.Columns), len(result.Data.Rows))
	}
}

// savePDFToFile saves the PDF bytes to a file.
func savePDFToFile(pdfBytes []byte, filename string) error {
	return os.WriteFile(filename, pdfBytes, 0644)
}

func main() {

	generatePDF := true
	exportCSV := true
	exportExcel := true
	generateHTML := true

	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, relying on OS environment.")
	}

	// LOAD CONFIGURATION from environment variables
	selectedDB := os.Getenv("DB_DRIVER")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	sslmode := os.Getenv("SSL_MODE")

	// Database configuration
	dbConfigs := map[string]DBConfig{
		"postgres": {
			DriverName: "postgres",
			DataSourceName: fmt.Sprintf("host=%s port=%s "+
				"user=%s password=%s "+
				"dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPassword, dbName, sslmode),
		},
		"mysql": {
			DriverName:     "mysql",
			DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb",
		},
	}

	config, exists := dbConfigs[selectedDB]
	if !exists {
		log.Fatalf("Database '%s' not configured", selectedDB)
	}

	// Database Connection
	dsn := config.DataSourceName
	db, err := sql.Open(selectedDB, dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Pinging the database to verify the connection is alive/established
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	fmt.Printf("Successfully connected to %s! database\n", selectedDB)

	// Initialize service
	reportService := NewReportService(db, generateHTML, generatePDF, exportCSV, exportExcel)

	// Setup routes
	http.HandleFunc("/health", reportService.HealthCheckHandler)
	http.HandleFunc("/api/v1/generate-report", reportService.GenerateReportHandler)

	// Configure server
	server := &http.Server{
		Addr:         ":8081",
		Handler:      nil,               // Use default ServeMux
		ReadTimeout:  30 * time.Second,  // Prevent slow client attacks
		WriteTimeout: 300 * time.Second, // Allow time for large file generation
		IdleTimeout:  60 * time.Second,
		//	MaxHeaderBytes: 1 << 20, // 1MB max header size
	}

	log.Println("Report generation service starting on :8081")
	log.Println("Available endpoints:")
	log.Println("  POST /api/v1/generate-report - Generate reports")
	log.Println("  GET  /health - Health check")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}
