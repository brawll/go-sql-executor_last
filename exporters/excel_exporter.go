package exporters

import (
	"fmt"
	"strings"

	"go-sql-executor/models"

	"github.com/xuri/excelize/v2"
)

func ExportToExcel(result models.QueryResult, filename string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"

	// Handle errors
	if result.Error != "" {
		f.SetCellValue(sheetName, "A1", "ERROR")
		f.SetCellValue(sheetName, "B1", result.Error)
		return f.SaveAs(filename)
	}

	// Handle empty results
	if result.Data == nil || len(result.Data.Rows) == 0 {
		f.SetCellValue(sheetName, "A1", "No data returned")
		return f.SaveAs(filename)
	}

	// Write headers (row 1)
	for i, col := range result.Data.Columns {
		cell := fmt.Sprintf("%s1", columnName(i))
		f.SetCellValue(sheetName, cell, col)
	}

	// Write data (starting from row 2)
	for rowIdx, row := range result.Data.Rows {
		for colIdx, col := range result.Data.Columns {
			cell := fmt.Sprintf("%s%d", columnName(colIdx), rowIdx+2)

			if val := row[col]; val != nil {
				// Convert to string explicitly
				var strVal string
				if str, ok := val.(string); ok {
					strVal = str
				} else {
					strVal = fmt.Sprintf("%v", val)
				}

				// Force Excel to treat timestamp strings as text to prevent auto-formatting
				if isDateTimeString(strVal) {
					// Set as string explicitly to preserve timestamp format
					f.SetCellStr(sheetName, cell, strVal)
				} else {
					f.SetCellValue(sheetName, cell, strVal)
				}
			}
		}
	}

	return f.SaveAs(filename)
}

// Helper function to convert column index to Excel column name
func columnName(index int) string {
	result := ""
	for index >= 0 {
		result = string(rune('A'+index%26)) + result
		index = index/26 - 1
	}
	return result
}

// isDateTimeString checks if a string looks like a timestamp that Excel might auto-format
func isDateTimeString(s string) bool {
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
