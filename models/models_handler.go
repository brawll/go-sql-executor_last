package models

import (
	"database/sql"
	"time"
)

// ReportRequest represents the incoming request structure
type ReportRequest struct {
	// Connection and schema information
	ConnectionID   string   `json:"connection_id"`
	SchemaName     string   `json:"schema_name"`
	TableName      string   `json:"table_name"`
	SelectedFields []string `json:"selected_fields"`

	// Filtering and aggregation
	Filters      interface{} `json:"filters,omitempty"`
	Aggregations interface{} `json:"aggregations,omitempty"`

	// Report metadata
	ReportTitle    string      `json:"report_title"`
	ReportFilename string      `json:"report_filename"`
	CategoryName   string      `json:"category_name"`
	TotalColumns   interface{} `json:"total_columns,omitempty"`

	// Template configuration
	TemplateID     *string                `json:"template_id,omitempty"`
	TemplateConfig map[string]interface{} `json:"template_config,omitempty"`

	// Legacy support
	Query string `json:"query,omitempty"`
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
	TempDir      string
	Db           *sql.DB
	GenerateHTML bool
	GeneratePDF  bool
	ExportCSV    bool
	ExportExcel  bool
}

// connections Database Models
type UserDBConnection struct {
	ID             string    `json:"id" db:"id"`
	ConnectionName string    `json:"connection_name" db:"connection_name"`
	Username       string    `json:"username" db:"username"`
	Password       string    `json:"password" db:"password"`
	Hostname       string    `json:"hostname" db:"hostname"`
	Port           int       `json:"port" db:"port"`
	DBName         string    `json:"db_name" db:"db_name"`
	DBType         string    `json:"db_type" db:"db_type"`
	Status         string    `json:"status" db:"status"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type StoredSchema struct {
	ID           string                 `json:"id" db:"id"`
	ConnectionID string                 `json:"connection_id" db:"connection_id"`
	SourceDBName string                 `json:"source_db_name" db:"source_db_name"`
	CapturedAt   time.Time              `json:"captured_at" db:"captured_at"`
	SchemaJSON   map[string]interface{} `json:"schema_json" db:"schema_json"`
}

// Request/Response Models
type ConnectionCreateRequest struct {
	ConnectionName string `json:"connection_name" validate:"required"`
	Username       string `json:"username" validate:"required"`
	Password       string `json:"password" validate:"required"`
	Hostname       string `json:"hostname" validate:"required"`
	Port           int    `json:"port" validate:"required"`
	DBType         string `json:"db_type" validate:"required"`
	DBName         string `json:"db_name" validate:"required"`
	Status         string `json:"status"`
}

type ConnectionResponse struct {
	ID             string    `json:"id"`
	ConnectionName string    `json:"connection_name"`
	Username       string    `json:"username"`
	Hostname       string    `json:"hostname"`
	Port           int       `json:"port"`
	DBType         string    `json:"db_type"`
	DBName         string    `json:"db_name"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type ErrorResponseConnection struct {
	Detail string `json:"detail"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

// Schema structures
type ColumnInfo struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Nullable     string      `json:"nullable"`
	Default      interface{} `json:"default"`
	MaxLength    interface{} `json:"max_length"`
	IsPrimaryKey bool        `json:"is_primary_key"`
}

type TableInfo struct {
	Type    string       `json:"type"`
	Columns []ColumnInfo `json:"columns"`
}

type SchemaInfo struct {
	Tables map[string]TableInfo `json:"tables"`
}

// Database Connection Manager
type DatabaseConnectionManager struct {
	Db *sql.DB
}

// Connection Handler
type ConnectionHandler struct {
	Dcm *DatabaseConnectionManager
	Db  *sql.DB
}
