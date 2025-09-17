package utils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// getConnectionInfo builds database connection string and driver for given type
func getConnectionInfo(dbType, hostname string, port int, username, password, dbName string) (string, string) {
	switch dbType {
	case "PostgreSQL":
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			hostname, port, username, password, dbName)
		return connStr, "postgres"
	case "MySQL":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, hostname, port, dbName)
		return connStr, "mysql"
	case "MSSQL":
		connStr := fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s",
			hostname, port, dbName, username, password)
		return connStr, "sqlserver"
	default:
		return "", ""
	}
}

// VerifyConnection tests database connectivity and returns the open connection
func VerifyConnection(dbType, hostname string, port int, username, password, dbName string) (*sql.DB, error) {
	connStr, driverName := getConnectionInfo(dbType, hostname, port, username, password, dbName)
	if connStr == "" {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	testDB, err := sql.Open(driverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
	}

	// Use a bounded context to ping
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	if err := testDB.PingContext(ctx); err != nil {
		testDB.Close() // Close before returning error
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("db ping timed out after 4s: %w", err)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return testDB, nil
}

// ColumnType represents column information for selected fields
type ColumnType struct {
	Name     string
	Type     string
	Nullable string
}

// GetColumnsType retrieves column type information for specific selected fields
func GetColumnsType(db *sql.DB, dbType, schemaName, tableName string, selectedFields []string) (map[string]ColumnType, error) {
	var query string
	var args []interface{}

	switch dbType {
	case "PostgreSQL":
		if len(selectedFields) > 0 {
			// Start with schema and table parameters
			args = []interface{}{schemaName, tableName}

			// Build IN clause for selected fields - placeholders start from $3
			placeholders := make([]string, len(selectedFields))
			for i := range selectedFields {
				placeholders[i] = "$" + fmt.Sprintf("%d", len(args)+i+1)
			}

			args = append(args, stringSliceToInterface(selectedFields)...)
			query = fmt.Sprintf(`
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = $1
				AND table_name = $2
				AND column_name IN (%s)
				ORDER BY ordinal_position`, strings.Join(placeholders, ","))
		} else {
			query = `
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = $1
				AND table_name = $2
				ORDER BY ordinal_position`
			args = []interface{}{schemaName, tableName}
		}
	case "MySQL":
		if len(selectedFields) > 0 {
			// Convert selectedFields to interface{} slice
			args = []interface{}{schemaName, tableName}
			args = append(args, stringSliceToInterface(selectedFields)...)
			placeholders := strings.Repeat("?,", len(selectedFields))
			placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
			query = fmt.Sprintf(`
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = ?
				AND table_name = ?
				AND column_name IN (%s)
				ORDER BY ordinal_position`, placeholders)
		} else {
			query = `
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = ?
				AND table_name = ?
				ORDER BY ordinal_position`
			args = []interface{}{schemaName, tableName}
		}
	case "MSSQL":
		if len(selectedFields) > 0 {
			// Convert selectedFields to interface{} slice
			args = []interface{}{schemaName, tableName}
			args = append(args, stringSliceToInterface(selectedFields)...)
			placeholders := strings.Repeat("?,", len(selectedFields))
			placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
			query = fmt.Sprintf(`
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = ?
				AND table_name = ?
				AND column_name IN (%s)
				ORDER BY ordinal_position`, placeholders)
		} else {
			query = `
				SELECT column_name, data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = ?
				AND table_name = ?
				ORDER BY ordinal_position`
			args = []interface{}{schemaName, tableName}
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]ColumnType)
	for rows.Next() {
		var col ColumnType
		err := rows.Scan(&col.Name, &col.Type, &col.Nullable)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column row: %v", err)
		}
		columns[col.Name] = col
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over column rows: %v", err)
	}

	return columns, nil
}

// stringSliceToInterface converts []string to []interface{}
func stringSliceToInterface(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}
