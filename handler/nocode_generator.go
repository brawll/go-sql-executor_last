package handler

import (
	"database/sql"
	"fmt"
	"go-sql-executor/utils"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type TableInfo struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type TableResponse struct {
	Success        bool        `json:"success"`
	Tables         []TableInfo `json:"tables,omitempty"`
	TotalCount     int         `json:"total_count,omitempty"`
	ConnectionName string      `json:"connection_name,omitempty"`
	DBType         string      `json:"db_type,omitempty"`
	Message        string      `json:"message,omitempty"`
}

type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable string  `json:"nullable"`
	Default  *string `json:"default"`
	IsDate   bool    `json:"is_date"`
}

type ColumnResponse struct {
	Success        bool         `json:"success"`
	ConnectionName string       `json:"connection_name"`
	DBType         string       `json:"db_type"`
	Schema         string       `json:"schema"`
	Table          string       `json:"table"`
	Columns        []ColumnInfo `json:"columns"`
	HasDateColumns bool         `json:"has_date_columns"`
}

type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type ReportRequest struct {
	ConnectionID   string   `json:"connection_id"`
	SchemaName     string   `json:"schema_name"`
	TableName      string   `json:"table_name"`
	SelectedFields []string `json:"selected_fields"`
	Filters        []Filter `json:"filters,omitempty"`
	SortBy         string   `json:"sort_by,omitempty"`
	SortOrder      string   `json:"sort_order,omitempty"`
	TemplateID     string   `json:"template_id,omitempty"`
}

type NoCodeGenerator struct {
	Db *sql.DB
}

func NewNoCodeGenerator(db *sql.DB) *NoCodeGenerator {
	return &NoCodeGenerator{Db: db}
}

// getTablesForDBType retrieves tables based on database type
func getTablesForDBType(conn *sql.DB, dbType string) ([]TableInfo, error) {
	var query string

	switch dbType {
	case "PostgreSQL":
		query = `
			SELECT
				table_schema,
				table_name
			FROM information_schema.tables
			WHERE table_schema NOT IN (
				'pg_catalog',
				'information_schema',
				'pg_toast'
			)
			AND table_type = 'BASE TABLE'
			ORDER BY table_schema, table_name;
		`
	case "Vertica":
		query = `
			SELECT
				table_schema,
				table_name
			FROM information_schema.tables
			WHERE table_schema NOT IN (
				'v_catalog',
				'information_schema'
			)
			AND table_type = 'BASE TABLE'
			ORDER BY table_schema, table_name;
		`
	case "MySQL":
		query = `
			SELECT
				table_schema as schema_name,
				table_name
			FROM information_schema.tables
			WHERE table_schema NOT IN (
				'mysql',
				'information_schema',
				'performance_schema',
				'sys'
			)
			AND table_type = 'BASE TABLE'
			ORDER BY table_schema, table_name;
		`
	case "MSSQL":
		query = `
			SELECT
				table_schema,
				table_name
			FROM information_schema.tables
			WHERE table_schema NOT IN (
				'sys',
				'information_schema'
			)
			AND table_type = 'BASE TABLE'
			ORDER BY table_schema, table_name;
		`
	case "Oracle":
		// List tables across non-system schemas
		query = `
			SELECT owner AS table_schema, table_name
			FROM all_tables
			WHERE owner NOT IN (
				'SYS','SYSTEM','XDB','MDSYS','CTXSYS','ORDSYS','OUTLN',
				'OLAPSYS','DBSNMP','APPQOSSYS','ORDDATA','AUDSYS','OJVMSYS',
				'GSMADMIN_INTERNAL'
			)
			ORDER BY owner, table_name
		`
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		err := rows.Scan(&table.Schema, &table.Table)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table row: %v", err)
		}
		tables = append(tables, table)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over table rows: %v", err)
	}

	return tables, nil
}

// GetTables - Renders tables for a given connection ID
func (ncg *NoCodeGenerator) GetTables(c *gin.Context) {
	connectionID := c.Param("connection_id")

	// Get user database connection details
	userDBConn, err := utils.GetUserDBConnection(connectionID, ncg.Db)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to get connection details: %v", err)})
		return
	}

	// Create database connection
	conn, err := utils.VerifyConnection(userDBConn.DBType, userDBConn.Hostname, userDBConn.Port,
		userDBConn.Username, userDBConn.Password, userDBConn.DBName)
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Database connection failed: %v", err)})
		return
	}
	defer conn.Close()

	// Query tables
	tables, err := getTablesForDBType(conn, userDBConn.DBType)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Error fetching tables: %v", err)})
		return
	}

	// Return response based on whether tables were found
	if len(tables) == 0 {
		response := TableResponse{
			Success:        false,
			Tables:         []TableInfo{},
			Message:        "No tables found in database",
			ConnectionName: userDBConn.ConnectionName,
			DBType:         userDBConn.DBType,
		}
		c.JSON(200, response)
		return
	}

	response := TableResponse{
		Success:        true,
		Tables:         tables,
		TotalCount:     len(tables),
		ConnectionName: userDBConn.ConnectionName,
		DBType:         userDBConn.DBType,
	}
	c.JSON(200, response)
}

// getColumnsForDBType retrieves columns and their metadata of a specific table based on database type
func getColumnsForDBType(conn *sql.DB, dbType, schema, tableName string) ([]ColumnInfo, error) {
	var query string
	var args []interface{}

	switch dbType {
	case "PostgreSQL":
		query = `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default
			FROM information_schema.columns
			WHERE table_schema = $1
			AND table_name = $2
			ORDER BY ordinal_position;
		`
		args = []interface{}{schema, tableName}
	case "MySQL":
		query = `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default
			FROM information_schema.columns
			WHERE table_schema = ?
			AND table_name = ?
			ORDER BY ordinal_position;
		`
		args = []interface{}{schema, tableName}
	case "MSSQL":
		query = `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default
			FROM information_schema.columns
			WHERE table_schema = 'dbo'
			AND table_name = ?
			ORDER BY ordinal_position;
		`
		args = []interface{}{tableName}
	case "Oracle":
		query = `
			SELECT
				column_name,
				data_type,
				nullable,
				data_default
			FROM all_tab_columns
			WHERE owner = :1
			AND table_name = :2
			ORDER BY column_id
		`
		args = []interface{}{strings.ToUpper(schema), strings.ToUpper(tableName)}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var defaultValue sql.NullString
		err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultValue)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column row: %v", err)
		}
		if defaultValue.Valid {
			col.Default = &defaultValue.String
		} else {
			col.Default = nil
		}
		// Check if it's a date type (handles Oracle TIMESTAMP variations as well)
		typeLower := strings.ToLower(col.Type)
		col.IsDate = typeLower == "date" ||
			typeLower == "datetime" ||
			strings.Contains(typeLower, "timestamp")
		columns = append(columns, col)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over column rows: %v", err)
	}

	return columns, nil
}

// GetColumnsPublic - Get all columns for a specific table in public schema
func (ncg *NoCodeGenerator) GetColumnsPublic(c *gin.Context) {
	dbConnID := c.Param("connection_id")
	schema := c.Param("schema_name")
	tableName := c.Param("table_name")

	userDBConn, err := utils.GetUserDBConnection(dbConnID, ncg.Db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to verify Database connection id: %v", err)})
		return
	}

	connStr, driverName := utils.GetConnectionInfo(userDBConn.DBType, userDBConn.Hostname, userDBConn.Port,
		userDBConn.Username, userDBConn.Password, userDBConn.DBName)

	db, err := sql.Open(driverName, connStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to open db connection: %v", err)})
		return
	}
	defer db.Close()

	// Query columns
	columns, err := getColumnsForDBType(db, userDBConn.DBType, schema, tableName)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Error fetching columns: %v", err)})
		return
	}

	// Check if table has any date columns
	hasDateColumns := false
	for _, col := range columns {
		if col.IsDate {
			hasDateColumns = true
			break
		}
	}

	response := ColumnResponse{
		Success:        true,
		ConnectionName: userDBConn.ConnectionName,
		DBType:         userDBConn.DBType,
		Schema:         schema,
		Table:          tableName,
		Columns:        columns,
		HasDateColumns: hasDateColumns,
	}
	c.JSON(200, response)
}

func (ncg *NoCodeGenerator) ReportPreview(c *gin.Context) {
	var req ReportRequest
	var page, pageSize int
	var err error

	// Bind JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get page and page_size from query params, defaulting to 1 and 50
	page = 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = 10
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 1000 {
			pageSize = parsed
		}
	}

	userConnection, err := utils.GetUserDBConnection(req.ConnectionID, ncg.Db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get connection details"})
		return
	}

	conn, err := utils.VerifyConnection(userConnection.DBType, userConnection.Hostname, userConnection.Port,
		userConnection.Username, userConnection.Password, userConnection.DBName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error:": "Database connection failed"})
		return
	}
	defer conn.Close()

	// Get column type information for selected fields (performance optimized)
	columnTypes, err := utils.GetColumnsType(conn, userConnection.DBType, req.SchemaName, req.TableName, req.SelectedFields)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching column types: %v", err)})
		return
	}

	// Build the main SQL query (no filtering since frontend doesn't use it)
	baseQuery := buildBaseQuery(userConnection.DBType, req.SchemaName, req.TableName, req.SelectedFields)

	// Add sorting if specified
	if req.SortBy != "" {
		baseQuery += fmt.Sprintf(" ORDER BY %s %s", req.SortBy, req.SortOrder)
	}

	// Build count query (no parameters since no filters)
	var countQuery string
	if userConnection.DBType == "Oracle" {
		countQuery = fmt.Sprintf("SELECT COUNT(*) FROM (%s) record_count", baseQuery)
	} else {
		countQuery = fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS record_count", baseQuery)
	}

	// Get total records for pagination info
	var totalRecords int
	err = conn.QueryRow(countQuery).Scan(&totalRecords)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error counting records: %v", err)})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalRecords) / float64(pageSize)))

	// Add pagination to the base query
	switch userConnection.DBType {
	case "MSSQL":
		offset := (page - 1) * pageSize
		baseQuery += fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, pageSize)
	case "Oracle":
		offset := (page - 1) * pageSize
		// Wrap to avoid ORDER BY requirement for FETCH in Oracle
		baseQuery = fmt.Sprintf("SELECT * FROM (%s) subq OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", baseQuery, offset, pageSize)
	default:
		offset := (page - 1) * pageSize
		baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, offset)
	}

	// Execute main query (no parameters since no filters)
	rows, err := conn.Query(baseQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error executing query: %v", err)})
		return
	}
	defer rows.Close()

	// Get columns in SQL result order, then potentially reorder for display
	originalColumns, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting columns: %v", err)})
		return
	}

	// Columns for response (may be reordered)
	responseColumns := originalColumns
	if req.SortBy != "" {
		responseColumns = reorderColumnsForSortField(originalColumns, req.SortBy)
	}

	// Scan results using original SQL column order
	results := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(originalColumns))
		scanArgs := make([]interface{}, len(originalColumns))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		err := rows.Scan(scanArgs...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error scanning row: %v", err)})
			return
		}

		// Create rowMap using RESPONSE column order (not SQL order)
		rowMap := make(map[string]interface{})
		for _, responseCol := range responseColumns {
			// Find the original SQL index for this response column
			originalIdx := -1
			for origIdx, origCol := range originalColumns {
				if origCol == responseCol {
					originalIdx = origIdx
					break
				}
			}

			if originalIdx >= 0 {
				if b, ok := values[originalIdx].([]byte); ok {
					rowMap[responseCol] = string(b)
				} else {
					rowMap[responseCol] = values[originalIdx]
				}
			}
		}
		results = append(results, rowMap)
	}

	// Calculate totals for numeric columns (no filtering)
	totals := make(map[string]interface{})

	numericColumns := []string{}
	for _, colType := range columnTypes {
		if isNumericType(colType.Type) {
			numericColumns = append(numericColumns, colType.Name)
		}
	}

	if len(numericColumns) > 0 {
		totalsQuery := fmt.Sprintf("SELECT %s FROM %s",
			buildNumericTotalsSQL(userConnection.DBType, req.SchemaName, req.TableName, numericColumns),
			quoteTableName(userConnection.DBType, req.SchemaName, req.TableName))

		totalsRow, err := conn.Query(totalsQuery)
		if err == nil && totalsRow != nil {
			defer totalsRow.Close()

			if totalsRow.Next() {
				totalsValues := make([]interface{}, len(numericColumns))
				scanArgs := make([]interface{}, len(totalsValues))
				for i := range totalsValues {
					scanArgs[i] = &totalsValues[i]
				}

				err := totalsRow.Scan(scanArgs...)
				if err != nil {
					// Silently handle totals error
				} else {
					for i, colName := range numericColumns {
						totals[colName+"_total"] = totalsValues[i]
					}
				}
			}
		}
	}

	// Build response
	response := gin.H{
		"success":       true,
		"data":          results,
		"total_records": totalRecords,
		"total_pages":   totalPages,
		"current_page":  page,
		"page_size":     pageSize,
		"columns":       responseColumns,
		"column_types":  columnTypeMap(columnTypes),
		"totals":        totals,
		"connection_info": gin.H{
			"connection_name": userConnection.ConnectionName,
			"db_type":         userConnection.DBType,
		},
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions for building queries

func buildBaseQuery(dbType, schemaName, tableName string, selectedFields []string) string {
	tableRef := quoteTableName(dbType, schemaName, tableName)

	if len(selectedFields) == 0 {
		return fmt.Sprintf("SELECT * FROM %s", tableRef)
	}

	fieldsRefs := make([]string, len(selectedFields))
	for i, field := range selectedFields {
		fieldsRefs[i] = quoteFieldName(dbType, field)
	}

	return fmt.Sprintf("SELECT %s FROM %s", strings.Join(fieldsRefs, ", "), tableRef)
}

func quoteTableName(dbType, schemaName, tableName string) string {
	switch dbType {
	case "PostgreSQL":
		return fmt.Sprintf("\"%s\".\"%s\"", schemaName, tableName)
	case "MySQL":
		return fmt.Sprintf("`%s`.`%s`", schemaName, tableName)
	case "MSSQL":
		return fmt.Sprintf("[%s].[%s]", schemaName, tableName)
	case "Oracle":
		// Use unquoted uppercase to avoid case-sensitivity issues
		return fmt.Sprintf("%s.%s", strings.ToUpper(schemaName), strings.ToUpper(tableName))
	default:
		return fmt.Sprintf("%s.%s", schemaName, tableName)
	}
}

func quoteFieldName(dbType, fieldName string) string {
	switch dbType {
	case "PostgreSQL":
		return fmt.Sprintf("\"%s\"", fieldName)
	case "MySQL":
		return fmt.Sprintf("`%s`", fieldName)
	case "MSSQL":
		return fmt.Sprintf("[%s]", fieldName)
	case "Oracle":
		// Use unquoted uppercase to avoid case-sensitivity issues
		return strings.ToUpper(fieldName)
	default:
		return fieldName
	}
}

func isNumericType(typeStr string) bool {
	typeLower := strings.ToLower(typeStr)
	return typeLower == "integer" || typeLower == "int" || typeLower == "bigint" ||
		typeLower == "smallint" || typeLower == "tinyint" ||
		typeLower == "numeric" || typeLower == "decimal" ||
		typeLower == "float" || typeLower == "double" ||
		typeLower == "real" || typeLower == "money" ||
		typeLower == "number"
}

func buildNumericTotalsSQL(dbType, schemaName, tableName string, numericColumns []string) string {
	totals := make([]string, len(numericColumns))
	for i, colName := range numericColumns {
		fieldRef := quoteFieldName(dbType, colName)
		totals[i] = fmt.Sprintf("SUM(%s)", fieldRef)
	}
	return strings.Join(totals, ", ")
}

func columnTypeMap(columnTypes map[string]utils.ColumnType) map[string]string {
	result := make(map[string]string)
	for name, colType := range columnTypes {
		result[name] = colType.Type
	}
	return result
}

// reorderColumnsForSortField moves the sort field to the front of columns array for better UX
func reorderColumnsForSortField(columns []string, sortField string) []string {
	if sortField == "" {
		return columns
	}

	// Check if sort field is in the columns
	sortFieldIndex := -1
	for i, col := range columns {
		if col == sortField {
			sortFieldIndex = i
			break
		}
	}

	// If sort field not found or already first, return original order
	if sortFieldIndex <= 0 {
		return columns
	}

	// Reorder: move sort field to front
	result := make([]string, len(columns))
	result[0] = sortField

	// Add remaining columns (skip the sort field)
	j := 1
	for _, col := range columns {
		if col != sortField {
			result[j] = col
			j++
		}
	}

	return result
}
