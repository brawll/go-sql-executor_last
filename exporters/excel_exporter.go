package exporters

import (
	"fmt"

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
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", val))
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
