package exporters

import (
	"encoding/csv"
	"fmt"
	"os"

	"go-sql-executor/models"
)

// CSVExporter handles exporting SQL query results to CSV format
type CSVExporter struct {
	IncludeQueryInfo bool
	Separator        rune
}

// NewCSVExporter creates a new CSV exporter with default settings
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{
		IncludeQueryInfo: true,
		Separator:        ',',
	}
}

func (ce *CSVExporter) ExportToCSV(results []models.QueryResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if ce.Separator != ',' {
		writer.Comma = ce.Separator
	}
	defer writer.Flush()

	for queryIndex, result := range results {
		// Add query information header
		if ce.IncludeQueryInfo {
			if err := ce.writeQueryHeader(writer, result); err != nil {
				return fmt.Errorf("failed to write query header: %w", err)
			}
		}

		// Handle errors
		if result.Error != "" {
			if err := writer.Write([]string{"ERROR", result.Error}); err != nil {
				return fmt.Errorf("failed to write error: %w", err)
			}
			continue
		}

		// Handle empty results
		if result.Data == nil || len(result.Data.Rows) == 0 {
			if err := writer.Write([]string{"INFO", "No data returned"}); err != nil {
				return fmt.Errorf("failed to write no data message: %w", err)
			}
			continue
		}

		// Write column headers
		if err := writer.Write(result.Data.Columns); err != nil {
			return fmt.Errorf("failed to write column headers: %w", err)
		}

		// Write data rows
		for _, row := range result.Data.Rows {
			csvRow := make([]string, len(result.Data.Columns))
			for i, col := range result.Data.Columns {
				if val := row[col]; val != nil {
					csvRow[i] = fmt.Sprintf("%v", val)
				} else {
					csvRow[i] = ""
				}
			}
			if err := writer.Write(csvRow); err != nil {
				return fmt.Errorf("failed to write data row: %w", err)
			}
		}

		// Add separator between queries if there are multiple
		if len(results) > 1 && queryIndex < len(results)-1 {
			if err := writer.Write([]string{}); err != nil {
				return fmt.Errorf("failed to write separator: %w", err)
			}
		}
	}

	return nil
}

// writeQueryHeader writes query information to CSV
func (ce *CSVExporter) writeQueryHeader(writer *csv.Writer, result models.QueryResult) error {
	// Query text
	if err := writer.Write([]string{"SQL", result.Query}); err != nil {
		return err
	}

	// Empty line for separation
	return writer.Write([]string{})
}
