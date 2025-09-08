package exporters

// Shared types that match main package
// QueryData holds the actual data from a query result.
type QueryData struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

// QueryResult holds the result of a single query execution.
type QueryResult struct {
	Query     string     `json:"query"`
	Data      *QueryData `json:"data,omitempty"`
	Error     string     `json:"error,omitempty"`
	Status    string     `json:"status"`
	Duration  string     `json:"duration,omitempty"`
	Timestamp string     `json:"timestamp"`
}

// PDFConfig holds PDF generation configuration.
type PDFConfig struct {
	Title          string
	Author         string
	Subject        string
	CompanyName    string
	HeaderColor    []uint8 // RGB values
	TableRowHeight float64
	FontSize       int
	MarginX        float64
	MarginY        float64
	ShowTimestamp  bool
	ShowQuery      bool
}
