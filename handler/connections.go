package handler

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go-sql-executor/models"
	"go-sql-executor/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DatabaseConnectionManager models.DatabaseConnectionManager
type ConnectionHandler models.ConnectionHandler

// Helper function for consistent error responses
func writeErrorResponse(c *gin.Context, status int, detail string) {
	c.JSON(status, models.ErrorResponseConnection{Detail: detail})
}

// CaptureSchema captures database schema information with performance logging and optimization
func (dcm *DatabaseConnectionManager) CaptureSchema(db *sql.DB, dbType string, dbName string) (*models.SchemaInfo, error) {
	startTime := time.Now()
	log.Printf("🔍 Starting schema capture for %s database: %s", dbType, dbName)

	// Configure minimal connection pool for schema capture (for speed)
	db.SetMaxOpenConns(10)                 // Only 1 connection needed
	db.SetMaxIdleConns(2)                  // No idle connections
	db.SetConnMaxLifetime(time.Minute * 1) // Shorter lifetime
	defer db.Close()

	schemaInfo := &models.SchemaInfo{Tables: make(map[string]models.TableInfo)}

	// Define table query based on database type
	var tablesQuery string
	var tablesArgs []interface{}

	var columnsQuery string
	var columnsArgs func(string) []interface{}

	switch dbType {
	case "PostgreSQL", "Vertica":
		tablesQuery = "SELECT t.table_name, t.table_type FROM information_schema.tables t WHERE t.table_schema = $1 ORDER BY t.table_name"
		tablesArgs = []interface{}{"public"}

		columnsQuery = `SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, c.character_maximum_length,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key
			FROM information_schema.columns c
			LEFT JOIN (SELECT kc.column_name FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kc ON kc.constraint_name = tc.constraint_name
				WHERE tc.constraint_type = 'PRIMARY KEY') pk ON pk.column_name = c.column_name
			WHERE c.table_schema = $1 AND c.table_name = $2 ORDER BY c.ordinal_position`

		columnsArgs = func(tableName string) []interface{} {
			return []interface{}{"public", tableName}
		}
	case "MySQL":
		tablesQuery = "SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name"
		tablesArgs = []interface{}{dbName}

		columnsQuery = `SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, c.character_maximum_length,
			CASE WHEN k.column_name IS NOT NULL THEN true ELSE false END as is_primary_key
			FROM information_schema.columns c
			LEFT JOIN information_schema.key_column_usage k ON k.table_schema = c.table_schema
				AND k.table_name = c.table_name AND k.column_name = c.column_name AND k.constraint_name = 'PRIMARY'
			WHERE c.table_schema = ? AND c.table_name = ? ORDER BY c.ordinal_position`

		columnsArgs = func(tableName string) []interface{} {
			return []interface{}{dbName, tableName}
		}
	case "MSSQL":
		tablesQuery = "SELECT TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW') ORDER BY TABLE_NAME"
		tablesArgs = []interface{}{}

		columnsQuery = `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, CHARACTER_MAXIMUM_LENGTH,
			CASE WHEN COLUMNPROPERTY(OBJECT_ID(TABLE_SCHEMA + '.' + TABLE_NAME), COLUMN_NAME, 'IsIdentity') = 1 THEN 1 ELSE 0 END as is_primary_key
			FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? ORDER BY ORDINAL_POSITION`

		columnsArgs = func(tableName string) []interface{} {
			return []interface{}{tableName}
		}
	case "Oracle":
		// Limit to current schema (user) using USER_* views for performance and simplicity
		tablesQuery = `
			SELECT table_name, 'BASE TABLE' AS table_type FROM user_tables
			UNION ALL
			SELECT view_name AS table_name, 'VIEW' AS table_type FROM user_views
			ORDER BY 1`
		tablesArgs = []interface{}{}

		columnsQuery = `
			SELECT
				c.column_name,
				c.data_type,
				c.nullable,
				c.data_default,
				c.char_col_decl_length,
				CASE
					WHEN EXISTS (
						SELECT 1
						FROM user_constraints uc
						JOIN user_cons_columns ucc ON uc.constraint_name = ucc.constraint_name
						WHERE uc.constraint_type = 'P'
						  AND ucc.table_name = c.table_name
						  AND ucc.column_name = c.column_name
					) THEN 1 ELSE 0
				END AS is_primary_key
			FROM user_tab_columns c
			WHERE c.table_name = :1
			ORDER BY c.column_id`

		columnsArgs = func(tableName string) []interface{} {
			return []interface{}{strings.ToUpper(tableName)}
		}
	}

	// Get tables
	tableQueryTime := time.Now()
	rows, err := db.Query(tablesQuery, tablesArgs...)
	if err != nil {
		log.Printf("❌ Failed to query tables for %s database: %v (took %v)", dbType, err, time.Since(tableQueryTime))
		return nil, err
	}
	defer rows.Close()

	tableScanTime := time.Now()

	var tables []struct {
		Name string
		Type string
	}

	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			return nil, err
		}
		tables = append(tables, struct {
			Name string
			Type string
		}{tableName, tableType})
	}

	log.Printf("📋 Found %d tables in %s database (scan took %v)",
		len(tables), dbType, time.Since(tableScanTime))

	// Get columns for each table
	columnQueryStart := time.Now()
	columnsProcessed := 0
	for _, table := range tables {
		colArgs := columnsArgs(table.Name)
		colRows, err := db.Query(columnsQuery, colArgs...)
		if err != nil {
			return nil, err
		}

		var columns []models.ColumnInfo
		for colRows.Next() {
			col := models.ColumnInfo{}
			var maxLength sql.NullInt64
			var defaultVal sql.NullString
			var pkVal bool

			switch dbType {
			case "MSSQL":
				var isPK int
				if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &isPK); err != nil {
					colRows.Close()
					return nil, err
				}
				col.IsPrimaryKey = isPK == 1
			case "Oracle":
				var isPK int
				if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &isPK); err != nil {
					colRows.Close()
					return nil, err
				}
				col.IsPrimaryKey = isPK == 1
			default:
				if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &pkVal); err != nil {
					colRows.Close()
					return nil, err
				}
				col.IsPrimaryKey = pkVal
			}

			if defaultVal.Valid {
				col.Default = defaultVal.String
			}
			if maxLength.Valid {
				col.MaxLength = maxLength.Int64
			}

			columns = append(columns, col)
		}
		colRows.Close()

		schemaInfo.Tables[table.Name] = models.TableInfo{
			Type:    table.Type,
			Columns: columns,
		}
		columnsProcessed += len(columns)
	}

	log.Printf("✅ Schema capture completed for %s database: %d tables, %d total columns (took %v total, %v for column queries)",
		dbType, len(tables), columnsProcessed, time.Since(startTime), time.Since(columnQueryStart))

	return schemaInfo, nil
}

func NewDatabaseConnectionManager(db *sql.DB) *models.DatabaseConnectionManager {
	return &models.DatabaseConnectionManager{Db: db}
}

func NewConnectionHandler(db *sql.DB) *ConnectionHandler {
	return &ConnectionHandler{
		Dcm: NewDatabaseConnectionManager(db),
		Db:  db,
	}
}

// Create connection endpoint
func (ch *ConnectionHandler) CreateConnection(c *gin.Context) {
	var req models.ConnectionCreateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(400, models.ErrorResponseConnection{Detail: "Invalid JSON"})
		return
	}

	// Set default status if not provided
	if req.Status == "" {
		req.Status = "active"
	}

	validatedDB, err := utils.VerifyConnection(req.DBType, req.Hostname, req.Port, req.Username, req.Password, req.DBName)
	if err != nil {
		writeErrorResponse(c, 400, fmt.Sprintf("Failed to establish database connection: %v", err))
		return
	}

	// Encode password
	encodedPassword := base64.StdEncoding.EncodeToString([]byte(req.Password))

	// Generate UUID
	connectionID := uuid.New().String()

	// Insert connection
	insertQuery := `
		INSERT INTO user_connection.user_db_connections
		(id, connection_name, username, password, hostname, port, db_name, db_type, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	createdAt := time.Now()

	_, err2 := ch.Db.Exec(insertQuery, connectionID, req.ConnectionName, req.Username, encodedPassword,
		req.Hostname, req.Port, req.DBName, req.DBType, req.Status, createdAt)
	if err2 != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to add new connection: %v", err)}
		c.JSON(500, errorResp)
		return
	}

	// Capture schema asynchronously (non-blocking)
	go func(schemaDB *sql.DB) {
		dc := (*DatabaseConnectionManager)(ch.Dcm)
		schemaInfo, captureErr := dc.CaptureSchema(schemaDB, req.DBType, req.DBName)
		if captureErr != nil {
			log.Printf("Warning: Failed to capture schema: %v", captureErr)
		} else {
			// Store schema
			schemaJSON, _ := json.Marshal(schemaInfo)
			schemaID := uuid.New().String()

			schemaInsertQuery := `
				INSERT INTO copied_schema.stored_schemas
				(id, connection_id, source_db_name, captured_at, schema_json)
				VALUES ($1, $2, $3, $4, $5)
			`

			_, insertErr := ch.Db.Exec(schemaInsertQuery, schemaID, connectionID, req.DBName, createdAt, string(schemaJSON))
			if insertErr != nil {
				log.Printf("Warning: Failed to store schema: %v", insertErr)
			}
		}
	}(validatedDB)

	// Return response
	response := models.ConnectionResponse{
		ID:             connectionID,
		ConnectionName: req.ConnectionName,
		Username:       req.Username,
		Hostname:       req.Hostname,
		Port:           req.Port,
		DBType:         req.DBType,
		DBName:         req.DBName,
		Status:         req.Status,
		CreatedAt:      createdAt,
	}

	c.JSON(200, response)
}

// Get all connections endpoint
func (ch *ConnectionHandler) GetConnections(c *gin.Context) {
	query := `
		SELECT id, connection_name, username, hostname, port, db_type, db_name, status, created_at
		FROM user_connection.user_db_connections
		ORDER BY created_at DESC
	`

	rows, err := ch.Db.Query(query)
	if err != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to fetch connections: %v", err)}
		c.JSON(500, errorResp)
		return
	}
	defer rows.Close()

	var connections []models.ConnectionResponse
	for rows.Next() {
		var conn models.ConnectionResponse
		err := rows.Scan(&conn.ID, &conn.ConnectionName, &conn.Username, &conn.Hostname,
			&conn.Port, &conn.DBType, &conn.DBName, &conn.Status, &conn.CreatedAt)
		if err != nil {
			errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to scan connection: %v", err)}
			c.JSON(500, errorResp)
			return
		}
		connections = append(connections, conn)
	}

	if connections == nil {
		connections = []models.ConnectionResponse{}
	}

	c.JSON(200, connections)
}

// Delete connection endpoint
func (ch *ConnectionHandler) DeleteConnection(c *gin.Context) {
	connectionID := c.Param("connection_id")

	// Check if connection exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM user_connection.user_db_connections WHERE id = $1)`
	err := ch.Db.QueryRow(checkQuery, connectionID).Scan(&exists)
	if err != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to check connection: %v", err)}
		c.JSON(500, errorResp)
		return
	}

	if !exists {
		errorResp := models.ErrorResponseConnection{Detail: "Connection not found"}
		c.JSON(404, errorResp)
		return
	}

	// Delete connection (schema will be deleted by CASCADE)
	deleteQuery := `DELETE FROM user_connection.user_db_connections WHERE id = $1`
	_, err = ch.Db.Exec(deleteQuery, connectionID)
	if err != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to delete connection: %v", err)}
		c.JSON(500, errorResp)
		return
	}

	response := models.SuccessResponse{Message: "Connection deleted successfully"}
	c.JSON(200, response)
}

// CORS middleware for Gin
func CorsMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	})
}
