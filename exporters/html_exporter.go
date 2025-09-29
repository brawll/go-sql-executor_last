package exporters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"go-sql-executor/models"
	"html/template"
	"os"
	"strings"
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

// PageData represents a single page of table data for pagination
type PageData struct {
	PageNumber int64         `json:"page_number"`
	Data       []interface{} `json:"data"`
	HasNext    bool          `json:"has_next"`
	StartRow   int64         `json:"start_row"`
	EndRow     int64         `json:"end_row"`
}

// PaginatedTemplateData represents the complete data structure for paginated HTML
type PaginatedTemplateData struct {
	models.HTMLConfig
	AllResults         []models.QueryResult
	CurrentResult      *models.QueryResult
	GeneratedAt        string
	PagesData          map[string][]PageData
	TotalRows          int64
	TotalColumns       int
	PageSize           int64
	HasMultipleResults bool
}

// GeneratePaginatedHTML generates optimized HTML with client-side pagination and search for large datasets
func GeneratePaginatedHTML(config models.HTMLConfig, results []models.QueryResult) ([]byte, error) {
	pageSize := int64(100) // Default page size for performance
	pagesData := make(map[string][]PageData)
	var totalRows int64
	hasMultipleResults := len(results) > 1

	for _, result := range results {
		if result.Data != nil && len(result.Data.Rows) > 0 {
			resultID := fmt.Sprintf("result_%s", result.Timestamp)
			pages := createPages(result.Data.Rows, pageSize)
			pagesData[resultID] = pages
			totalRows += int64(len(result.Data.Rows))
		}
	}

	var currentResult *models.QueryResult
	if len(results) > 0 {
		currentResult = &results[0]
	}

	// Convert map to JSON string for template embedding
	pagesDataJSON, err := json.Marshal(pagesData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pages data: %w", err)
	}

	templateData := PaginatedTemplateData{
		HTMLConfig:         config,
		AllResults:         results,
		CurrentResult:      currentResult,
		GeneratedAt:        time.Now().Format("January 2, 2006 at 3:04 PM"),
		PagesData:          pagesData,
		TotalRows:          totalRows,
		PageSize:           pageSize,
		HasMultipleResults: hasMultipleResults,
	}

	// Add the pagesDataJSON to template functions
	tmpl, err := template.New("paginated-report").Funcs(template.FuncMap{
		"add":           func(a, b int) int { return a + b },
		"notnil":        func(v interface{}) bool { return v != nil },
		"pagesDataJSON": func() string { return string(pagesDataJSON) },
		"resultsJSON":   func(results []models.QueryResult) string { d, _ := json.Marshal(results); return string(d) },
		"formatNumber":  func(n interface{}) string { return formatNumber(n) },
		"sub":           func(a, b int) int { return a - b },
		"safe":          func(s string) template.HTML { return template.HTML(s) },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"lenCols": func(cols []string) int { return len(cols) },
	}).Parse(htmlTemplatePaginated)

	if err != nil {
		return nil, fmt.Errorf("failed to parse paginated HTML template: %w", err)
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated HTML template: %w", err)
	}

	return buffer.Bytes(), nil
}

// createPages splits data rows into pages for pagination
func createPages(rows []map[string]interface{}, pageSize int64) []PageData {
	var pages []PageData
	numPages := int64(math.Ceil(float64(len(rows)) / float64(pageSize)))

	for pageNum := int64(0); pageNum < numPages; pageNum++ {
		startIdx := pageNum * pageSize
		endIdx := startIdx + pageSize

		if endIdx > int64(len(rows)) {
			endIdx = int64(len(rows))
		}

		pageData := make([]interface{}, endIdx-startIdx)
		for i := startIdx; i < endIdx; i++ {
			pageData[i-startIdx] = rows[i]
		}

		pages = append(pages, PageData{
			PageNumber: pageNum + 1,
			Data:       pageData,
			HasNext:    pageNum < numPages-1,
			StartRow:   startIdx + 1,
			EndRow:     endIdx,
		})
	}

	return pages
}

// formatNumber formats large numbers for display
func formatNumber(n interface{}) string {
	switch v := n.(type) {
	case int:
		return formatInt64(int64(v))
	case int64:
		return formatInt64(v)
	case float64:
		if v == float64(int64(v)) {
			return formatInt64(int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%v", n)
	}
}

// formatInt64 adds thousand separators to large numbers
func formatInt64(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}

	str := strconv.FormatInt(n, 10)
	parts := []string{}
	for len(str) > 3 {
		parts = append([]string{str[len(str)-3:]}, parts...)
		str = str[:len(str)-3]
	}
	if len(str) > 0 {
		parts = append([]string{str}, parts...)
	}
	return strings.Join(parts, ",")
}

// GeneratePerformanceOptimizedHTML is an alias for GeneratePaginatedHTML for backward compatibility
func GeneratePerformanceOptimizedHTML(config models.HTMLConfig, results []models.QueryResult) ([]byte, error) {
	return GeneratePaginatedHTML(config, results)
}

// Performance-optimized HTML template with pagination, search, and lazy loading for large datasets
const htmlTemplatePaginated = `
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
            overflow-x: hidden;
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
            position: relative;
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

        .controls-bar {
            background: {{if eq .Theme "dark"}}#2d2d2d{{else}}white{{end}};
            border-radius: 10px;
            padding: 20px;
            margin-bottom: 20px;
            box-shadow: 0 2px 10px rgba(0,0,0,{{if eq .Theme "dark"}}0.3{{else}}0.1{{end}});
            border: 1px solid {{if eq .Theme "dark"}}#404040{{else}}#e0e0e0{{end}};
            display: flex;
            flex-wrap: wrap;
            gap: 15px;
            align-items: center;
            justify-content: space-between;
        }

        .search-container {
            flex: 1;
            min-width: 250px;
        }

        .search-input {
            width: 100%;
            padding: 12px;
            border: 1px solid {{if eq .Theme "dark"}}#555{{else}}#ddd{{end}};
            border-radius: 8px;
            background: {{if eq .Theme "dark"}}#404040{{else}}#fff{{end}};
            color: {{if eq .Theme "dark"}}#e0e0e0{{else}}#333{{end}};
            font-size: 0.9rem;
        }

        .search-input:focus {
            outline: none;
            border-color: {{.HeaderColor}};
            box-shadow: 0 0 0 2px {{.HeaderColor}}33;
        }

        .controls-group {
            display: flex;
            gap: 10px;
            align-items: center;
        }

        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: all 0.2s ease;
        }

        .btn-primary {
            background: {{.HeaderColor}};
            color: white;
        }

        .btn-primary:hover {
            background: {{.HeaderColor}}dd;
        }

        .btn-secondary {
            background: {{if eq .Theme "dark"}}#555{{else}}#f8f9fa{{end}};
            color: {{if eq .Theme "dark"}}#e0e0e0{{else}}#333{{end}};
            border: 1px solid {{if eq .Theme "dark"}}#777{{else}}#dee2e6{{end}};
        }

        .btn-secondary:hover {
            background: {{if eq .Theme "dark"}}#666{{else}}#e2e6ea{{end}};
        }

        .page-size-selector {
            display: flex;
            align-items: center;
            gap: 5px;
        }

        .page-size-selector select {
            padding: 8px;
            border: 1px solid {{if eq .Theme "dark"}}#555{{else}}#ddd{{end}};
            border-radius: 6px;
            background: {{if eq .Theme "dark"}}#404040{{else}}#fff{{end}};
            color: {{if eq .Theme "dark"}}#e0e0e0{{else}}#333{{end}};
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
            justify-content: space-between;
        }

        .query-header h2 {
            color: {{.HeaderColor}};
            font-size: 1.5rem;
            margin-right: 15px;
        }

        .result-tabs {
            display: flex;
            gap: 10px;
            margin-bottom: 20px;
            flex-wrap: wrap;
        }

        .result-tab {
            padding: 8px 16px;
            background: {{if eq .Theme "dark"}}#404040{{else}}#f8f9fa{{end}};
            border: 1px solid {{if eq .Theme "dark"}}#555{{else}}#dee2e6{{end}};
            border-radius: 6px;
            cursor: pointer;
            font-weight: 500;
            transition: all 0.2s ease;
        }

        .result-tab.active {
            background: {{.HeaderColor}};
            color: white;
            border-color: {{.HeaderColor}};
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
            max-height: 95vh;  /* Increase vh value for more rows */
            overflow-y: auto;
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
            z-index: 20;
            cursor: pointer;
            transition: all 0.2s ease;
        }

        .data-table th:hover {
            background: {{.HeaderColor}}dd;
        }

        .data-table th.sort-asc::after {
            content: ' ~';
            margin-left: 5px;
        }

        .data-table th.sort-desc::after {
            content: ' |';
            margin-left: 5px;
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
            vertical-align: top;
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

        .loading {
            text-align: center;
            padding: 40px;
            color: {{.HeaderColor}};
            font-weight: 500;
        }

        .loading::after {
            content: '';
            display: inline-block;
            width: 20px;
            height: 20px;
            border: 3px solid {{.HeaderColor}}33;
            border-radius: 50%;
            border-top-color: {{.HeaderColor}};
            animation: spin 1s ease-in-out infinite;
            margin-left: 10px;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        .error-message {
            background-color: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
            padding: 15px;
            border-radius: 5px;
            margin: 15px 0;
        }

        .pagination {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 20px;
            flex-wrap: wrap;
            gap: 15px;
        }

        .pagination-info {
            font-size: 0.9rem;
            color: {{if eq .Theme "dark"}}#ccc{{else}}#666{{end}};
        }

        .pagination-controls {
            display: flex;
            align-items: center;
            gap: 10px;
            flex-wrap: wrap;
        }

        .pagination-buttons {
            display: flex;
            gap: 5px;
            align-items: center;
        }

        .page-btn {
            padding: 8px 12px;
            border: 1px solid {{if eq .Theme "dark"}}#555{{else}}#dee2e6{{end}};
            background: {{if eq .Theme "dark"}}#404040{{else}}#fff{{end}};
            color: {{if eq .Theme "dark"}}#e0e0e0{{else}}#333{{end}};
            cursor: pointer;
            border-radius: 4px;
            transition: all 0.2s ease;
            font-size: 0.9rem;
        }

        .page-btn:hover {
            background: {{if eq .Theme "dark"}}#555{{else}}#f8f9fa{{end}};
        }

        .page-btn.active {
            background: {{.HeaderColor}};
            color: white;
            border-color: {{.HeaderColor}};
        }

        .page-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
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

        .scroll-indicator {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: {{.HeaderColor}};
            color: white;
            padding: 10px;
            border-radius: 50%;
            cursor: pointer;
            display: none;
            z-index: 100;
        }

        @media (max-width: 768px) {
            .container {
                padding: 10px;
            }

            .header h1 {
                font-size: 2rem;
            }

            .controls-bar {
                flex-direction: column;
                align-items: stretch;
            }

            .search-container {
                min-width: auto;
            }

            .query-info {
                grid-template-columns: 1fr;
            }

            .pagination {
                flex-direction: column;
                align-items: stretch;
            }

            .pagination-controls {
                flex-direction: column;
                gap: 10px;
            }

            .result-tabs {
                font-size: 0.9rem;
            }

            .result-tab {
                padding: 6px 12px;
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

        .hidden {
            display: none !important;
        }

        .fade-in {
            animation: fadeIn 0.3s ease-in;
        }

        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }

        .performance-notes {
            background: {{if eq .Theme "dark"}}#404040{{else}}#e8f5e8{{end}};
            border: 1px solid {{if eq .Theme "dark"}}#555{{else}}#d4edda{{end}};
            border-radius: 5px;
            padding: 15px;
            margin-bottom: 20px;
            font-size: 0.9rem;
        }

        .performance-notes h4 {
            color: {{.HeaderColor}};
            margin-bottom: 8px;
            font-size: 1rem;
        }

        .cell-tooltip {
            position: relative;
        }

        .cell-tooltip:hover::after {
            content: attr(title);
            position: absolute;
            background: {{if eq .Theme "dark"}}#555{{else}}rgba(0,0,0,0.8){{end}};
            color: white;
            padding: 5px 8px;
            border-radius: 4px;
            font-size: 0.8rem;
            white-space: nowrap;
            z-index: 50;
            bottom: 100%;
            left: 50%;
            transform: translateX(-50%);
            max-width: 300px;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .data-table td.cell-tooltip {
            position: relative;
        }

        .filter-active {
            background: {{.HeaderColor}}22 !important;
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

        <div class="performance-notes">
            <h4>🚀 Performance Optimized View</h4>
            <div>
                • Showing {{.PageSize}} rows per page for optimal performance<br>
                • Total: {{formatNumber .TotalRows}} rows across all results<br>
                • Use search and pagination to navigate efficiently<br>
                • Click column headers to sort data
            </div>
        </div>

        {{if .HasMultipleResults}}
        <div class="controls-bar">
            <div class="result-tabs">
                {{range $index, $result := .AllResults}}
                <div class="result-tab {{if eq $index 0}}active{{end}}" data-result-id="result_{{$result.Timestamp}}">
                    Query {{$index | add 1}}
                    <span class="status-badge {{if eq $result.Status "success"}}status-success{{else}}status-error{{end}}">
                        {{$result.Status}}
                    </span>
                </div>
                {{end}}
            </div>
        </div>
        {{end}}

        <div class="controls-bar">
            <div class="search-container">
                <input type="text" class="search-input" placeholder="🔍 Search all columns..." id="global-search">
            </div>
            <div class="controls-group">
                <div class="page-size-selector">
                    <label for="page-size">Rows per page:</label>
                    <select id="page-size">
                        <option value="25">25</option>
                        <option value="50">50</option>
                        <option value="100" selected>100</option>
                        <option value="250">250</option>
                        <option value="500">500</option>
                    </select>
                </div>
                <button class="btn btn-primary" onclick="exportCurrentPage()">
                    📥 Export Page
                </button>
            </div>
        </div>

        <div id="query-sections">
            {{range $index, $result := .AllResults}}
            <div class="query-section {{if ne $index 0}}hidden{{end}}" data-result-id="result_{{$result.Timestamp}}">
                <div class="query-header">
                    <div>
                        <h2>Query {{add $index 1}} Results</h2>
                        <span class="status-badge {{if eq $result.Status "success"}}status-success{{else}}status-error{{end}}">
                            {{if eq $result.Status "success"}}✅{{else}}❌{{end}} {{$result.Status}}
                        </span>
                    </div>
                </div>

                <div class="query-info">
                    <div class="info-item">
                        <div class="info-label">Executed At</div>
                        <div>{{$result.Timestamp}}</div>
                    </div>
                    {{if $result.Data}}
                    <div class="info-item">
                        <div class="info-label">Columns</div>
                        <div>{{lenCols $result.Data.Columns}}</div>
                    </div>
                    <div class="info-item">
                        <div class="info-label">Total Rows</div>
                        <div>{{formatNumber (len $result.Data.Rows)}}</div>
                    </div>
                    {{end}}
                    <div class="info-item">
                        <div class="info-label">Duration</div>
                        <div>{{$result.Duration}}</div>
                    </div>
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
                <div class="pagination">
                    <div class="pagination-info" id="pagination-info-{{$index}}">
                        Showing rows 1-100 of {{formatNumber (len $result.Data.Rows)}}
                    </div>
                    <div class="pagination-controls">
                        <button class="page-btn" id="first-btn-{{$index}}" onclick="goToFirstPage('{{$result.Timestamp}}')" disabled>⏮️ First</button>
                        <button class="page-btn" id="prev-btn-{{$index}}" onclick="goToPreviousPage('{{$result.Timestamp}}')" disabled>◀️ Prev</button>
                        <div class="pagination-buttons" id="page-buttons-{{$index}}"></div>
                        <button class="page-btn" id="next-btn-{{$index}}" onclick="goToNextPage('{{$result.Timestamp}}')">Next ▶️</button>
                        <button class="page-btn" id="last-btn-{{$index}}" onclick="goToLastPage('{{$result.Timestamp}}')">Last ⏭️</button>
                    </div>
                </div>

                <div class="scroll-hint">
                    📏 Scroll horizontally to see all {{lenCols $result.Data.Columns}} columns • Use pagination for large datasets
                </div>

                <div class="table-container" id="table-container-{{$index}}">
                    <table class="data-table" id="data-table-{{$index}}">
                        <thead id="table-header-{{$index}}">
                            <tr>
                                {{range $colIndex, $col := $result.Data.Columns}}
                                <th onclick="sortTable('{{$index}}', '{{$col}}', this)" data-column="{{$col}}">
                                    {{$col}}
                                </th>
                                {{end}}
                            </tr>
                        </thead>
                        <tbody id="table-body-{{$index}}" class="fade-in">
                            <!-- Rows will be loaded here -->
                        </tbody>
                    </table>
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
    </div>

    <div class="scroll-indicator" id="scroll-indicator" onclick="scrollToTop()">
        ↑
    </div>

    <script>
        const pagesData = JSON.parse('{{pagesDataJSON | safe}}');
        const allResults = JSON.parse('{{resultsJSON .AllResults | safe}}');
        let currentPages = {};
        let currentSort = {};
        let globalSearchTerm = '';
        const pageSizeSelector = document.getElementById('page-size');

        // Initialize on page load
        document.addEventListener('DOMContentLoaded', function() {
            initializeResults();
            updateScrollIndicator();

            // Global search
            document.getElementById('global-search').addEventListener('input', function(e) {
                globalSearchTerm = e.target.value.toLowerCase();
                refreshCurrentView();
            });

            // Page size change
            pageSizeSelector.addEventListener('change', function(e) {
                changePageSize(parseInt(e.target.value));
            });

            // Scroll indicator
            window.addEventListener('scroll', updateScrollIndicator);
        });

        function initializeResults() {
    const querySections = document.querySelectorAll('#query-sections > .query-section');
    querySections.forEach((section, index) => {
        const resultId = section.dataset.resultId;
        if (pagesData[resultId] && pagesData[resultId].length > 0) {
            console.log('Loading result:', resultId, 'Pages:', pagesData[resultId].length);
            currentPages[resultId] = 0;
            loadPage(allResults[index].timestamp, 0); // Pass timestamp, but loadPage will convert to index
            updatePagination(index, 0); // Pass the array index directly
        } else {
            console.warn('No pages data found for result:', resultId, '- Falling back to direct rendering');
            // Fallback code...
        }
    });

    // Result tab switching
    document.querySelectorAll('.result-tab').forEach(tab => {
        tab.addEventListener('click', function() {
            switchResult(this.dataset.resultId);
        });
    });
}

        function switchResult(resultId) {
            document.querySelectorAll('.result-tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.query-section').forEach(s => s.classList.add('hidden'));

            document.querySelector('[data-result-id="' + resultId + '"]').classList.remove('hidden');
            document.querySelector('[data-result-id="' + resultId + '"]').classList.add('active');

            const resultIndex = resultId.split('_')[1];
            if (!currentPages[resultId]) {
                currentPages[resultId] = 0;
                loadPage(resultIndex, 0);
                updatePagination(resultIndex, 0);
            }
        }

        function goToNextPage(timestamp) {
            const resultId = 'result_' + timestamp;
            const currentPage = currentPages[resultId] || 0;
            goToPage(currentPage + 1, timestamp);
        }

        function goToPreviousPage(timestamp) {
            const resultId = 'result_' + timestamp;
            const currentPage = currentPages[resultId] || 0;
            if (currentPage > 0) {
                goToPage(currentPage - 1, timestamp);
            }
        }

        function goToFirstPage(timestamp) {
            goToPage(0, timestamp);
        }

        function loadPage(resultTimestamp, pageIndex) {
            const resultId = 'result_' + resultTimestamp;
            console.log('loadPage called for resultId:', resultId, 'pageIndex:', pageIndex);

            // Convert timestamp to array index
            const arrayIndex = getResultIndex(resultTimestamp);
            if (arrayIndex === -1) {
                console.error('Result not found for timestamp:', resultTimestamp);
                return;
            }

            // First try to load from pre-paginated data
            if (pagesData[resultId] && pagesData[resultId][pageIndex]) {
                console.log('Using pre-paginated data for page', pageIndex);
                const pageData = pagesData[resultId][pageIndex];
                if (pageData.Data && pageData.Data.length > 0) {
                    console.log('Page data found:', pageData.Data.length, 'rows');
                    renderTable(arrayIndex, pageData.Data);
                    return;
                }
            }

            // Fallback to dynamic pagination if pre-paginated data not available
            const result = allResults[arrayIndex];
            if (!result || !result.data) return;

            console.log('Using fallback dynamic pagination for resultTimestamp:', resultTimestamp);

            const tableBody = document.getElementById('table-body-' + arrayIndex);
            if (tableBody) {
                tableBody.innerHTML = '<tr><td colspan="' + result.data.columns.length + '" class="loading">Loading...</td></tr>';
            }

            setTimeout(() => {
                const rowsToShow = getFilteredRows(result, pageIndex);
                console.log('Fallback loaded', rowsToShow.length, 'rows for page', pageIndex);
                renderTable(arrayIndex, rowsToShow);
            }, 10);
        }

        function getFilteredRows(result, pageIndex) {
            // Handle case where result or result.data may be undefined (use lowercase)
            if (!result || !result.data || !result.data.rows) {
                console.warn('getFilteredRows: Invalid result data for pageIndex:', pageIndex);
                return [];
            }

            let rows = [...result.data.rows];  // Changed from result.Data.Rows to result.data.rows

            // Apply search filter
            if (globalSearchTerm) {
                rows = rows.filter(row => {
                    return result.data.columns.some(col =>  // Changed from result.Data.Columns to result.data.columns
                        String(row[col] || '').toLowerCase().includes(globalSearchTerm)
                    );
                });
            }

            // Apply sorting
            const resultId = 'result_' + result.Timestamp;
            if (currentSort[resultId]) {
                const sortData = currentSort[resultId];
                const column = sortData.column;
                const direction = sortData.direction;
                rows.sort((a, b) => {
                    const aVal = a[column];
                    const bVal = b[column];
                    let comparison = 0;

                    if (typeof aVal === 'string' && typeof bVal === 'string') {
                        comparison = aVal.localeCompare(bVal);
                    } else if (typeof aVal === 'number' && typeof bVal === 'number') {
                        comparison = aVal - bVal;
                    } else {
                        comparison = String(aVal || '').localeCompare(String(bVal || ''));
                    }

                    return direction === 'asc' ? comparison : -comparison;
                });
            }

            const pageSize = parseInt(pageSizeSelector.value || 100);
            const start = pageIndex * pageSize;
            const end = start + pageSize;
            return rows.slice(start, end);
        }

        function renderTable(arrayIndex, rows) {
            const result = allResults[arrayIndex]; // Use array index directly
            
            const tableBody = document.getElementById('table-body-' + arrayIndex); // Use arrayIndex
            tableBody.innerHTML = '';

            if (rows.length === 0) {
                const colCount = result && result.data && result.data.columns ? result.data.columns.length : 153;
                tableBody.innerHTML = '<tr><td colspan="' + colCount + '" class="no-data">No matching rows found</td></tr>';
                return;
            }

            console.log('renderTable: Rendering', rows.length, 'rows for arrayIndex:', arrayIndex);

            // Handle case where result or result.data may be undefined
            if (!result || !result.data || !result.data.columns) {
                console.warn('renderTable: Missing result data for arrayIndex:', arrayIndex, result);
                tableBody.innerHTML = '<tr><td colspan="153" class="no-data">Error: Missing data</td></tr>';
                return;
            }

            rows.forEach((row, rowIndex) => {
                const tr = document.createElement('tr');
                result.data.columns.forEach(col => {
                    const td = document.createElement('td');
                    const value = row[col];
                    const text = value === null || value === undefined ? 'NULL' : String(value);
                    td.textContent = text;
                    td.title = text.length > 50 ? text : '';
                    tr.appendChild(td);
                });
                tableBody.appendChild(tr);
            });

            console.log('renderTable: Successfully rendered table for arrayIndex:', arrayIndex);
        }

        function goToPage(pageIndex, timestamp) {
            console.log('goToPage called with pageIndex:', pageIndex, 'timestamp:', timestamp);
            
            const arrayIndex = getResultIndex(timestamp);
            if (arrayIndex === -1) {
                console.error('Result not found for timestamp:', timestamp);
                return;
            }
            
            const result = allResults[arrayIndex];
            if (!result || !result.data) {
                console.error('Invalid result data for timestamp:', timestamp);
                return;
            }
            
            // Calculate total pages to validate pageIndex
            const totalRows = result.data.rows.length;
            const pageSize = parseInt(pageSizeSelector.value || 100);
            const totalPages = Math.ceil(totalRows / pageSize);
            
            console.log('Pagination info:', {
                pageIndex,
                totalPages,
                totalRows,
                pageSize
            });
            
            // Validate page index
            if (pageIndex < 0 || pageIndex >= totalPages) {
                console.warn('Invalid page index:', pageIndex, 'totalPages:', totalPages);
                return;
            }
            
            // Update current page BEFORE loading
            const resultId = 'result_' + timestamp;
            currentPages[resultId] = pageIndex;
            console.log('Updated currentPages[' + resultId + '] to:', pageIndex);
            
            const filteredRows = getFilteredRows(result, pageIndex);
            console.log('Filtered rows for page:', filteredRows.length);

            // Load page and update pagination with the correct pageIndex
            loadPage(timestamp, pageIndex);
            updatePagination(arrayIndex, pageIndex); // Make sure we pass the correct pageIndex
        }

        function goToLastPage(timestamp) {
            console.log('goToLastPage called with timestamp:', timestamp);
            
            const arrayIndex = getResultIndex(timestamp);
            if (arrayIndex === -1) {
                console.error('Result not found for timestamp:', timestamp);
                return;
            }
            
            const result = allResults[arrayIndex];
            if (!result || !result.data) {
                console.error('Invalid result data for timestamp:', timestamp);
                return;
            }
            
            const totalRows = result.data.rows.length;
            const pageSize = parseInt(pageSizeSelector.value || 100);
            const lastPage = Math.max(0, Math.ceil(totalRows / pageSize) - 1);
            
            console.log('Going to last page:', lastPage);
            goToPage(lastPage, timestamp);
        }


        function updatePagination(arrayIndex, pageIndex) {
            console.log('updatePagination called with arrayIndex:', arrayIndex, 'pageIndex:', pageIndex);
            
            const result = allResults[arrayIndex]; // Use array index directly

            // Validate data before proceeding
            if (!result || !result.data || !result.data.rows) {
                console.warn('updatePagination: Invalid result data for arrayIndex:', arrayIndex);
                console.warn('Found result:', result);
                return;
            }

            // Calculate total pages based on ALL rows, not filtered rows for pagination purposes
            let totalRows = result.data.rows.length;
            
            // Apply search filter to get filtered count
            if (globalSearchTerm) {
                const filteredRows = result.data.rows.filter(row => {
                    return result.data.columns.some(col =>
                        String(row[col] || '').toLowerCase().includes(globalSearchTerm)
                    );
                });
                totalRows = filteredRows.length;
            }

            const pageSize = parseInt(pageSizeSelector.value || 100);

            // Handle zero page size case
            if (pageSize <= 0) {
                console.warn('updatePagination: Invalid page size:', pageSize);
                return;
            }

            const totalPages = Math.ceil(totalRows / pageSize);
            
            console.log('updatePagination debug:', {
                arrayIndex, 
                pageIndex, 
                totalRows, 
                pageSize, 
                totalPages,
                currentPage: pageIndex
            });

            const startRow = pageIndex * pageSize + 1;
            const endRow = Math.min((pageIndex + 1) * pageSize, totalRows);

            // Safe element access with null checks using arrayIndex
            const paginationInfo = document.getElementById('pagination-info-' + arrayIndex);
            if (paginationInfo) {
                paginationInfo.textContent =
                    'Showing rows ' + startRow.toLocaleString() + '-' +
                    endRow.toLocaleString() + ' of ' + totalRows.toLocaleString();
            } else {
                console.warn('Pagination info element not found for arrayIndex:', arrayIndex);
            }

            // Safe button access with null checks using arrayIndex
            const firstBtn = document.getElementById('first-btn-' + arrayIndex);
            if (firstBtn) {
                const shouldDisable = pageIndex === 0;
                firstBtn.disabled = shouldDisable;
                console.log('First button - pageIndex:', pageIndex, 'disabled:', shouldDisable);
            }

            const prevBtn = document.getElementById('prev-btn-' + arrayIndex);
            if (prevBtn) {
                const shouldDisable = pageIndex === 0;
                prevBtn.disabled = shouldDisable;
                console.log('Prev button - pageIndex:', pageIndex, 'disabled:', shouldDisable);
            }

            const nextBtn = document.getElementById('next-btn-' + arrayIndex);
            if (nextBtn) {
                const shouldDisable = pageIndex >= totalPages - 1;
                nextBtn.disabled = shouldDisable;
                console.log('Next button - pageIndex:', pageIndex, 'totalPages:', totalPages, 'disabled:', shouldDisable);
            }

            const lastBtn = document.getElementById('last-btn-' + arrayIndex);
            if (lastBtn) {
                const shouldDisable = pageIndex >= totalPages - 1;
                lastBtn.disabled = shouldDisable;
                console.log('Last button - pageIndex:', pageIndex, 'totalPages:', totalPages, 'disabled:', shouldDisable);
            }

            // Generate page number buttons using arrayIndex
            generatePageButtons(arrayIndex, pageIndex, totalPages);
        }

        function generatePageButtons(arrayIndex, currentPage, totalPages) {
            const pageButtons = document.getElementById('page-buttons-' + arrayIndex);
            if (!pageButtons) {
                console.warn('Page buttons container not found for arrayIndex:', arrayIndex);
                return;
            }
            
            pageButtons.innerHTML = '';

            const maxVisiblePages = 5;
            let start = Math.max(0, currentPage - Math.floor(maxVisiblePages / 2));
            let end = start + maxVisiblePages;

            if (end > totalPages) {
                end = totalPages;
                start = Math.max(0, end - maxVisiblePages);
            }

            for (let i = start; i < end; i++) {
                const btn = document.createElement('button');
                btn.className = 'page-btn' + (i === currentPage ? ' active' : '');
                btn.textContent = i + 1;
                btn.onclick = () => goToPage(i, allResults[arrayIndex].timestamp);
                pageButtons.appendChild(btn);
            }
        }

        function sortTable(resultIndex, column, headerElement) {
            const resultId = 'result_' + resultIndex;
            const currentSortForResult = currentSort[resultId];

            let direction = 'asc';
            if (currentSortForResult && currentSortForResult.column === column) {
                direction = currentSortForResult.direction === 'asc' ? 'desc' : 'asc';
            }

            currentSort[resultId] = { column, direction };

            // Update header styles
            const headerSelector = '#table-header-' + resultIndex + ' th';
            document.querySelectorAll(headerSelector).forEach(th => {
                th.classList.remove('sort-asc', 'sort-desc');
            });

            const currentHeaderSelector = '#table-header-' + resultIndex + ' th[data-column="' + column + '"]';
            const currentHeader = document.querySelector(currentHeaderSelector);
            if (currentHeader) {
                currentHeader.classList.add(direction === 'asc' ? 'sort-asc' : 'sort-desc');
            }

            // Refresh current page with new sort
            const currentPage = currentPages[resultId] || 0;
            loadPage(resultIndex, currentPage);
        }

        function changePageSize(newSize) {
            const currentResultId = getActiveResultId();
            if (currentResultId) {
                currentPages[currentResultId] = 0;
                const resultIndex = currentResultId.split('_')[1];
                loadPage(resultIndex, 0);
                updatePagination(resultIndex, 0);
            }
        }

        function refreshCurrentView() {
            const currentResultId = getActiveResultId();
            if (currentResultId) {
                const resultIndex = currentResultId.split('_')[1];
                const currentPage = currentPages[currentResultId] || 0;
                loadPage(resultIndex, currentPage);
                updatePagination(resultIndex, currentPage);
            }
        }

        function getActiveResultId() {
            const activeSection = document.querySelector('#query-sections > .query-section:not(.hidden)');
            return activeSection ? activeSection.dataset.resultId : null;
        }

        function updateScrollIndicator() {
            const scrollIndicator = document.getElementById('scroll-indicator');
            if (window.pageYOffset > 200) {
                scrollIndicator.style.display = 'block';
            } else {
                scrollIndicator.style.display = 'none';
            }
        }

        function scrollToTop() {
            window.scrollTo({
                top: 0,
                behavior: 'smooth'
            });
        }

        function getResultIndex(timestamp) {
            return allResults.findIndex(r => r.timestamp === timestamp);
        }

        function exportCurrentPage() {
            const currentResultId = getActiveResultId();
            if (!currentResultId) return;

            const resultIndex = currentResultId.split('_')[1];
            const result = allResults.find(r => r.timestamp === resultIndex);
            if (!result || !result.data) return;  // Use lowercase data

            const currentPage = currentPages[currentResultId] || 0;
            const rowsToShow = getFilteredRows(result, currentPage);

            // Create CSV content
            let csvContent = result.data.columns.join(',') + '\\n';

                rowsToShow.forEach(row => {
                    const values = result.data.columns.map(col => {
                    const value = row[col];
                    let strValue = value === null || value === undefined ? '' : String(value);
                    if (strValue.includes(',') || strValue.includes('"') || strValue.includes('\\n')) {
                        strValue = '"' + strValue.replace(/"/g, '""') + '"';
                    }
                    return strValue;
                });
                csvContent += values.join(',') + '\\n';
            });

            // Download CSV
            const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
            const link = document.createElement('a');
            const url = URL.createObjectURL(blob);
            link.setAttribute('href', url);
            link.setAttribute('download', 'query_results_page_' + (currentPage + 1) + '.csv');
            link.style.visibility = 'hidden';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        }
    </script>
</body>
</html>
`

// Add the HTML template constant for backward compatibility
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
                            <td>{{if notnil (index $row $colName)}}{{index $row $colName}}{{else}}NULL{{end}}</td>
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

// Original GenerateHTML function (still available for backward compatibility)
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
		"add":    func(a, b int) int { return a + b },
		"notnil": func(v interface{}) bool { return v != nil },
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
