package handler

import (
	"database/sql"
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

// OpenAPI-compliant response structures
type ReportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	// Data    *QueryResult     `json:"data,omitempty"`
	Files   []FileInfo       `json:"files,omitempty"`
	Summary ExecutionSummary `json:"summary"`
}

type FileInfo struct {
	Type      string `json:"type"`
	Filename  string `json:"filename"`
	Path      string `json:"path,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Columns   int    `json:"columns,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

type ExecutionSummary struct {
	TotalQueries       int    `json:"total_queries"`
	SuccessfulQueries  int    `json:"successful_queries"`
	FailedQueries      int    `json:"failed_queries"`
	TotalExecutionTime string `json:"total_execution_time"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Database  string `json:"database"`
}

type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp"`
}

type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
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
