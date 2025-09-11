package exporters

import (
	"bytes"
	"fmt"

	"go-sql-executor/models"
	"html/template"
	"os"
	"time"
)

// DefaultHTMLConfig returns default HTML configuration.
func DefaultHTMLConfig() models.HTMLConfig {
	return models.HTMLConfig{
		Title:       "SQL Query Results",
		CompanyName: "Your Company",

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
func GenerateHTML(config models.HTMLConfig, results []models.QueryResult) ([]byte, error) {
	// Create template with the config and results data
	data := struct {
		models.HTMLConfig
		Results     []models.QueryResult
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
func SaveHTMLToFile(htmlBytes []byte, filename string) error {
	return os.WriteFile(filename, htmlBytes, 0644)
}
