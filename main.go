package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
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
	Query  string     `json:"query"`
	Data   *QueryData `json:"data,omitempty"`
	Error  string     `json:"error,omitempty"`
	Status string     `json:"status"`
}

// OutputFormat represents different ways to display results.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// readQueriesFromFile reads SQL queries from a given file.
// It ignores empty lines and lines starting with "--".
func readQueriesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open query file: %w", err)
	}
	defer file.Close()

	var queries []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "--") {
			queries = append(queries, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading query file: %w", err)
	}

	return queries, nil
}

// executeQuery executes a single SQL query and stores the complete result.
func executeQuery(
	wg *sync.WaitGroup,
	db *sql.DB,
	query string,
	resultChan chan<- QueryResult,
) {
	defer wg.Done()

	rows, err := db.Query(query)
	if err != nil {
		resultChan <- QueryResult{
			Query:  query,
			Error:  fmt.Sprintf("failed to execute query: %v", err),
			Status: "error",
		}
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		resultChan <- QueryResult{
			Query:  query,
			Error:  fmt.Sprintf("failed to get columns: %v", err),
			Status: "error",
		}
		return
	}

	// Get column types
	// columnTypes, err := rows.ColumnTypes()
	// if err != nil {
	// 	resultChan <- QueryResult{
	// 		Query:  query,
	// 		Error:  fmt.Sprintf("failed to get column types: %v", err),
	// 		Status: "error",
	// 	}
	// 	return
	// }

	var queryData QueryData
	queryData.Columns = columns
	queryData.Rows = make([]map[string]interface{}, 0)

	// Scan all rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		// Create pointers to the values
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row
		if err := rows.Scan(valuePtrs...); err != nil {
			resultChan <- QueryResult{
				Query:  query,
				Error:  fmt.Sprintf("failed to scan row: %v", err),
				Status: "error",
			}
			return
		}

		// Convert to map with proper type handling
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle different data types and NULL values
			if val == nil {
				row[col] = nil
			} else {
				// Convert byte arrays to strings (common for text fields)
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
			Query:  query,
			Error:  fmt.Sprintf("error iterating rows: %v", err),
			Status: "error",
		}
		return
	}

	resultChan <- QueryResult{
		Query:  query,
		Data:   &queryData,
		Status: "success",
	}
}

// displayResults shows the query results in the specified format.
func displayResults(results []QueryResult, format OutputFormat) {
	fmt.Printf("\n--- Query Results (%s format) ---\n", format)

	for i, result := range results {
		fmt.Printf("\n=== Query %d ===\n", i+1)
		fmt.Printf("SQL: %s\n", result.Query)
		fmt.Printf("Status: %s\n", result.Status)

		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
			continue
		}

		if result.Data == nil || len(result.Data.Rows) == 0 {
			fmt.Println("No data returned.")
			continue
		}

		switch format {
		case FormatJSON:
			displayJSON(result.Data)
		case FormatCSV:
			displayCSV(result.Data)
		case FormatTable:
			displayTable(result.Data)
		default:
			displayTable(result.Data)
		}
	}
}

// displayJSON shows results in JSON format.
func displayJSON(data *QueryData) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

// displayCSV shows results in CSV format.
func displayCSV(data *QueryData) {
	// Print headers
	fmt.Println(strings.Join(data.Columns, ","))

	// Print rows
	for _, row := range data.Rows {
		values := make([]string, len(data.Columns))
		for i, col := range data.Columns {
			if val := row[col]; val != nil {
				values[i] = fmt.Sprintf("%v", val)
			} else {
				values[i] = ""
			}
		}
		fmt.Println(strings.Join(values, ","))
	}
}

// displayTable shows results in a formatted table.
func displayTable(data *QueryData) {
	if len(data.Rows) == 0 {
		fmt.Println("No rows returned.")
		return
	}

	// Calculate column widths
	colWidths := make([]int, len(data.Columns))
	for i, col := range data.Columns {
		colWidths[i] = len(col)
	}

	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := ""
			if row[col] != nil {
				val = fmt.Sprintf("%v", row[col])
			}
			if len(val) > colWidths[i] {
				colWidths[i] = len(val)
			}
		}
	}

	// Print header
	for i, col := range data.Columns {
		fmt.Printf("%-*s", colWidths[i]+2, col)
	}
	fmt.Println()

	// Print separator
	for _, width := range colWidths {
		fmt.Printf("%s", strings.Repeat("-", width+2))
	}
	fmt.Println()

	// Print rows
	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := ""
			if row[col] != nil {
				val = fmt.Sprintf("%v", row[col])
			}
			fmt.Printf("%-*s", colWidths[i]+2, val)
		}
		fmt.Println()
	}

	fmt.Printf("\nTotal rows: %d\n", len(data.Rows))
}

// saveResultsToFile saves all results to a JSON file.
func saveResultsToFile(results []QueryResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func main() {
	// --- Configuration ---
	dbConfigs := map[string]DBConfig{
		"postgres": {
			DriverName: "postgres",
			DataSourceName: "host=localhost port=5432 " +
				"user=postgres password=RahulM6? " +
				"dbname=sql-executor sslmode=disable",
		},
		"mysql": {
			DriverName:     "mysql",
			DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb",
		},
	}

	// Configuration options
	selectedDB := "postgres"  // or "mysql"
	outputFormat := FormatCSV // FormatTable, FormatJSON, or FormatCSV
	saveToFile := true
	outputFile := "query_results.json"

	config, ok := dbConfigs[selectedDB]
	if !ok {
		log.Fatalf("Database '%s' not configured", selectedDB)
	}

	// --- Database Connection ---
	db, err := sql.Open(config.DriverName, config.DataSourceName)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Check if the connection is actually alive
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	fmt.Printf("Successfully connected to %s!\n", selectedDB)

	// --- Concurrent Query Execution ---
	queries, err := readQueriesFromFile("queries.txt")
	if err != nil {
		log.Fatalf("Failed to read queries: %v", err)
	}
	if len(queries) == 0 {
		log.Fatalf("No queries found in queries.txt")
	}
	fmt.Printf("Found %d queries to execute.\n", len(queries))

	var wg sync.WaitGroup
	resultChan := make(chan QueryResult, len(queries))

	// Execute all queries concurrently
	for _, query := range queries {
		wg.Add(1)
		go executeQuery(&wg, db, query, resultChan)
	}

	// Wait for all queries to finish
	wg.Wait()
	close(resultChan)

	// --- Collect and Process Results ---
	var results []QueryResult
	for result := range resultChan {
		results = append(results, result)
	}

	// Display results
	displayResults(results, outputFormat)

	// Save results to file if requested
	if saveToFile {
		if err := saveResultsToFile(results, outputFile); err != nil {
			log.Printf("Failed to save results to file: %v", err)
		} else {
			fmt.Printf("\nResults saved to: %s\n", outputFile)
		}
	}
}
