package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/xuri/excelize/v2"
)

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

// savePDFToFile saves the PDF bytes to a file.
func savePDFToFile(pdfBytes []byte, filename string) error {
	return os.WriteFile(filename, pdfBytes, 0644)
}

//go:embed openapi.yaml
var openAPISpec []byte // This is a compiler directive, not a function call

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

	// Serve OpenAPI spec
	http.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(openAPISpec)
	})

	// Serve Swagger UI
	http.HandleFunc("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/openapi.yaml"),
	))

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
	log.Println("  GET  /docs/ - Interactive API documentation (Swagger UI)")
	log.Println("  GET  /openapi.yaml - OpenAPI specification")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}
