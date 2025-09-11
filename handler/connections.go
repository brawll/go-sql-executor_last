package handler

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-sql-executor/models"

	_ "github.com/denisenkom/go-mssqldb" // MSSQL driver
	_ "github.com/go-sql-driver/mysql"   // MySQL driver
	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver
)

type DatabaseConnectionManager models.DatabaseConnectionManager
type ConnectionHandler models.ConnectionHandler

// Capture schema information
func (dcm *DatabaseConnectionManager) CaptureSchema(dbType, hostname string, port int, username, password, dbName string) (*models.SchemaInfo, error) {
	var connStr string
	var driverName string

	switch dbType {
	case "PostgreSQL":
		driverName = "postgres"
		connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			hostname, port, username, password, dbName)
	case "MySQL":
		driverName = "mysql"
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, hostname, port, dbName)
	case "MSSQL":
		driverName = "sqlserver"
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s",
			hostname, port, dbName, username, password)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	testDB, err := sql.Open(driverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
	}
	defer testDB.Close()

	schemaInfo := &models.SchemaInfo{Tables: make(map[string]models.TableInfo)}

	switch dbType {
	case "PostgreSQL":
		return dcm.capturePostgreSQLSchema(testDB, schemaInfo)
	case "MySQL":
		return dcm.captureMySQLSchema(testDB, dbName, schemaInfo)
	case "MSSQL":
		return dcm.captureMSSQLSchema(testDB, schemaInfo)
	}

	return schemaInfo, nil
}

func (dcm *DatabaseConnectionManager) capturePostgreSQLSchema(db *sql.DB, schemaInfo *models.SchemaInfo) (*models.SchemaInfo, error) {
	// Get tables
	tablesQuery := `
		SELECT t.table_name, t.table_type
		FROM information_schema.tables t
		WHERE t.table_schema = 'public'
		ORDER BY t.table_name;
	`

	rows, err := db.Query(tablesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

	// Get columns for each table
	for _, table := range tables {
		columnsQuery := `
			SELECT 
				c.column_name,
				c.data_type,
				c.is_nullable,
				c.column_default,
				c.character_maximum_length,
				CASE 
					WHEN pk.column_name IS NOT NULL THEN true 
					ELSE false 
				END as is_primary_key
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT kc.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kc 
					ON kc.constraint_name = tc.constraint_name
				WHERE tc.constraint_type = 'PRIMARY KEY'
				AND kc.table_name = $1
			) pk ON pk.column_name = c.column_name
			WHERE c.table_schema = 'public'
			AND c.table_name = $1
			ORDER BY c.ordinal_position;
		`

		colRows, err := db.Query(columnsQuery, table.Name)
		if err != nil {
			return nil, err
		}

		var columns []models.ColumnInfo
		for colRows.Next() {
			var col models.ColumnInfo
			var maxLength sql.NullInt64
			var defaultVal sql.NullString

			err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &col.IsPrimaryKey)
			if err != nil {
				colRows.Close()
				return nil, err
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
	}

	return schemaInfo, nil
}

func (dcm *DatabaseConnectionManager) captureMySQLSchema(db *sql.DB, dbName string, schemaInfo *models.SchemaInfo) (*models.SchemaInfo, error) {
	// Get tables
	tablesQuery := `
		SELECT table_name, table_type
		FROM information_schema.tables 
		WHERE table_schema = ?
		ORDER BY table_name;
	`

	rows, err := db.Query(tablesQuery, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

	// Get columns for each table
	for _, table := range tables {
		columnsQuery := `
			SELECT 
				c.column_name,
				c.data_type,
				c.is_nullable,
				c.column_default,
				c.character_maximum_length,
				CASE 
					WHEN k.column_name IS NOT NULL THEN true 
					ELSE false 
				END as is_primary_key
			FROM information_schema.columns c
			LEFT JOIN information_schema.key_column_usage k
				ON k.table_schema = c.table_schema
				AND k.table_name = c.table_name
				AND k.column_name = c.column_name
				AND k.constraint_name = 'PRIMARY'
			WHERE c.table_schema = ?
			AND c.table_name = ?
			ORDER BY c.ordinal_position;
		`

		colRows, err := db.Query(columnsQuery, dbName, table.Name)
		if err != nil {
			return nil, err
		}

		var columns []models.ColumnInfo
		for colRows.Next() {
			var col models.ColumnInfo
			var maxLength sql.NullInt64
			var defaultVal sql.NullString

			err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &col.IsPrimaryKey)
			if err != nil {
				colRows.Close()
				return nil, err
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
	}

	return schemaInfo, nil
}

func (dcm *DatabaseConnectionManager) captureMSSQLSchema(db *sql.DB, schemaInfo *models.SchemaInfo) (*models.SchemaInfo, error) {
	// Get tables
	tablesQuery := `
		SELECT TABLE_NAME, TABLE_TYPE
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY TABLE_NAME;
	`

	rows, err := db.Query(tablesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

	// Get columns for each table
	for _, table := range tables {
		columnsQuery := `
			SELECT 
				COLUMN_NAME,
				DATA_TYPE,
				IS_NULLABLE,
				COLUMN_DEFAULT,
				CHARACTER_MAXIMUM_LENGTH,
				CASE 
					WHEN COLUMNPROPERTY(OBJECT_ID(TABLE_SCHEMA + '.' + TABLE_NAME), COLUMN_NAME, 'IsIdentity') = 1 
					THEN 1
					ELSE 0
				END as is_primary_key
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION;
		`

		colRows, err := db.Query(columnsQuery, table.Name)
		if err != nil {
			return nil, err
		}

		var columns []models.ColumnInfo
		for colRows.Next() {
			var col models.ColumnInfo
			var maxLength sql.NullInt64
			var defaultVal sql.NullString
			var isPK int

			err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &maxLength, &isPK)
			if err != nil {
				colRows.Close()
				return nil, err
			}

			if defaultVal.Valid {
				col.Default = defaultVal.String
			}
			if maxLength.Valid {
				col.MaxLength = maxLength.Int64
			}
			col.IsPrimaryKey = isPK == 1

			columns = append(columns, col)
		}
		colRows.Close()

		schemaInfo.Tables[table.Name] = models.TableInfo{
			Type:    table.Type,
			Columns: columns,
		}
	}

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

func (dcm *DatabaseConnectionManager) VerifyConnection(dbType, hostname string, port int, username, password, dbName string) error {
	var connStr string
	var driverName string

	switch dbType {
	case "PostgreSQL":
		driverName = "postgres"
		connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			hostname, port, username, password, dbName)
	case "MySQL":
		driverName = "mysql"
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, hostname, port, dbName)
	case "MSSQL":
		driverName = "sqlserver"
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s",
			hostname, port, dbName, username, password)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	testDB, err := sql.Open(driverName, connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %v", err)
	}
	defer testDB.Close()

	if err := testDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	return nil
}

// Create connection endpoint
func (ch *ConnectionHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var req models.ConnectionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"detail":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Set default status if not provided
	if req.Status == "" {
		req.Status = "active"
	}

	// Verify database connection
	dc := (*DatabaseConnectionManager)(ch.Dcm) // Type Assertion of model type to the alias local type.
	if err := dc.VerifyConnection(req.DBType, req.Hostname, req.Port, req.Username, req.Password, req.DBName); err != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to establish database connection: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResp)
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

	_, err := ch.Db.Exec(insertQuery, connectionID, req.ConnectionName, req.Username, encodedPassword,
		req.Hostname, req.Port, req.DBName, req.DBType, req.Status, createdAt)
	if err != nil {
		errorResp := models.ErrorResponseConnection{Detail: fmt.Sprintf("Failed to create connection: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Capture schema
	dc2 := (*DatabaseConnectionManager)(ch.Dcm)
	schemaInfo, err := dc2.CaptureSchema(req.DBType, req.Hostname, req.Port, req.Username, req.Password, req.DBName)
	if err != nil {
		log.Printf("Warning: Failed to capture schema: %v", err)
	} else {
		// Store schema
		schemaJSON, _ := json.Marshal(schemaInfo)
		schemaID := uuid.New().String()

		schemaInsertQuery := `
			INSERT INTO copied_schema.stored_schemas 
			(id, connection_id, source_db_name, captured_at, schema_json)
			VALUES ($1, $2, $3, $4, $5)
		`

		_, err = ch.Db.Exec(schemaInsertQuery, schemaID, connectionID, req.DBName, createdAt, string(schemaJSON))
		if err != nil {
			log.Printf("Warning: Failed to store schema: %v", err)
		}
	}

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Get all connections endpoint
func (ch *ConnectionHandler) GetConnections(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, connection_name, username, hostname, port, db_type, db_name, status, created_at
		FROM user_connection.user_db_connections
		ORDER BY created_at DESC
	`

	rows, err := ch.db.Query(query)
	if err != nil {
		errorResp := ErrorResponseConnection{Detail: fmt.Sprintf("Failed to fetch connections: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}
	defer rows.Close()

	var connections []ConnectionResponse
	for rows.Next() {
		var conn ConnectionResponse
		err := rows.Scan(&conn.ID, &conn.ConnectionName, &conn.Username, &conn.Hostname,
			&conn.Port, &conn.DBType, &conn.DBName, &conn.Status, &conn.CreatedAt)
		if err != nil {
			errorResp := ErrorResponseConnection{Detail: fmt.Sprintf("Failed to scan connection: %v", err)}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(errorResp)
			return
		}
		connections = append(connections, conn)
	}

	if connections == nil {
		connections = []ConnectionResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connections)
}

// Get connection by ID endpoint, we may need it for testing but it's might just be redundant.
// func (ch *ConnectionHandler) GetConnection(w http.ResponseWriter, r *http.Request) {
// 	connectionID := r.PathValue("connection_id")

// 	query := `
// 		SELECT id, connection_name, username, hostname, port, db_type, db_name, status, created_at
// 		FROM user_connection.user_db_connections
// 		WHERE id = $1
// 	`

// 	var conn ConnectionResponse
// 	err := ch.db.QueryRow(query, connectionID).Scan(&conn.ID, &conn.ConnectionName, &conn.Username,
// 		&conn.Hostname, &conn.Port, &conn.DBType, &conn.DBName, &conn.Status, &conn.CreatedAt)

// 	if err == sql.ErrNoRows {
// 		errorResp := ErrorResponseConnection{Detail: "Connection not found"}
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusNotFound)
// 		json.NewEncoder(w).Encode(errorResp)
// 		return
// 	} else if err != nil {
// 		errorResp := ErrorResponseConnection{Detail: fmt.Sprintf("Failed to fetch connection: %v", err)}
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusInternalServerError)
// 		json.NewEncoder(w).Encode(errorResp)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(conn)
// }

// Delete connection endpoint
func (ch *ConnectionHandler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	connectionID := r.PathValue("connection_id")

	// Check if connection exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM user_connection.user_db_connections WHERE id = $1)`
	err := ch.db.QueryRow(checkQuery, connectionID).Scan(&exists)
	if err != nil {
		errorResp := ErrorResponseConnection{Detail: fmt.Sprintf("Failed to check connection: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	if !exists {
		errorResp := ErrorResponseConnection{Detail: "Connection not found"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Delete connection (schema will be deleted by CASCADE)
	deleteQuery := `DELETE FROM user_connection.user_db_connections WHERE id = $1`
	_, err = ch.db.Exec(deleteQuery, connectionID)
	if err != nil {
		errorResp := ErrorResponseConnection{Detail: fmt.Sprintf("Failed to delete connection: %v", err)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	response := SuccessResponse{Message: "Connection deleted successfully"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CORS middleware
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
