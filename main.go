package main

import (
	"bufio"
	"database/sql"
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

// QueryResult holds the result of a single query execution.
type QueryResult struct {
	Result string
	Err    error
}

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

// executeQuery executes a single SQL query on the given database and sends the result to a channel.
func executeQuery(wg *sync.WaitGroup, db *sql.DB, query string, resultChan chan<- QueryResult) {
	defer wg.Done()

	rows, err := db.Query(query)
	if err != nil {
		resultChan <- QueryResult{Err: fmt.Errorf("failed to execute query '%s': %w", query, err)}
		return
	}
	defer rows.Close()

	// For simplicity, we'll just count the rows.
	// You can modify this to scan the rows into a struct.
	rowCount := 0
	for rows.Next() {
		rowCount++
	}

	if err := rows.Err(); err != nil {
		resultChan <- QueryResult{Err: fmt.Errorf("error iterating rows for query '%s': %w", query, err)}
		return
	}

	resultChan <- QueryResult{Result: fmt.Sprintf("Query '%s' executed successfully, %d rows returned", query, rowCount)}
}

func main() {
	// --- Configuration ---
	// Replace with your actual database connection strings.
	dbConfigs := map[string]DBConfig{
		"postgres": {
			DriverName:     "postgres",
			DataSourceName: "host=localhost port=5432 user=postgres password=RahulM6? dbname=sql-executor sslmode=disable",
		},
		"mysql": {
			DriverName:     "mysql",
			DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb",
		},
	}

	// Choose the database to use.
	selectedDB := "postgres" // or "mysql"
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

	// Check if the connection is actually alive.
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	fmt.Printf("Successfully connected to %s!\n", selectedDB)

	// --- Concurrent Query Execution ---
	// Read queries from the file.
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

	for _, query := range queries {
		wg.Add(1)
		go executeQuery(&wg, db, query, resultChan)
	}

	// Wait for all queries to finish.
	wg.Wait()
	close(resultChan)

	// --- Process Results ---
	fmt.Println("\n--- Query Results ---")
	for result := range resultChan {
		if result.Err != nil {
			log.Printf("Error: %v", result.Err)
		} else {
			fmt.Println(result.Result)
		}
	}
}
