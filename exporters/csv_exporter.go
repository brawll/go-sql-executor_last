package exporters

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

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
					// Convert to string explicitly and treat all values as text to prevent Excel date formatting
					var strVal string
					if str, ok := val.(string); ok {
						strVal = str
					} else {
						strVal = fmt.Sprintf("%v", val)
					}

					// Preserve timestamp strings by ensuring they're treated as text
					// Excel can auto-format timestamps, so we make sure they stay as strings
					if ce.isDateTimeString(strVal) {
						// This looks like a timestamp/date/time string that Excel might auto-format
						csvRow[i] = `"` + strVal + `"`
					} else {
						csvRow[i] = strVal
					}
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

// isDateTimeString checks if a string looks like a timestamp that Excel might auto-format
func (ce *CSVExporter) isDateTimeString(s string) bool {

	// Date + Time patterns (what we currently generate)
	if strings.Contains(s, ":") && (strings.Contains(s, "-") || strings.Contains(s, "/")) {
		// Examples: "2025-09-05 17:40:33", "09/05/2025 17:40:33.733761"
		return true
	}

	// Date-only patterns
	dateOnlyPatterns := []string{
		"????-??-??", // YYYY-MM-DD
		"????/??/??", // YYYY/MM/DD
		"??/??/????", // MM/DD/YYYY
		"??-??-????", // MM-DD-YYYY
		"??-??-??",   // DD-MM-YY or MM-DD-YY
	}
	for _, pattern := range dateOnlyPatterns {
		if len(s) == len(pattern) && (strings.Contains(s, "-") || strings.Contains(s, "/")) {
			return true
		}
	}

	// Time-only patterns
	timeOnlyPatterns := []string{
		"??:??:??",          // HH:MM:SS (8 chars)
		"??:??:??\\.??????", // HH:MM:SS.microseconds
		"?:??:??",           // H:MM:SS (7 chars)
	}
	// For time-only patterns, just check length and presence of colons
	for _, _ = range timeOnlyPatterns {
		if strings.Contains(s, ":") && !strings.Contains(s, "-") && !strings.Contains(s, "/") && len(s) >= 7 && len(s) <= 15 {
			// Time-only (no date separators), check if length matches expected patterns
			if len(s) >= 7 && len(s) <= 15 { // Reasonable length for time
				return true
			}
		}
	}

	return false
}
