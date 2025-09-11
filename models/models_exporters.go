package models

// type CSVExporter is defined in the csv_exporter file as we have method dependency on that type.

type HTMLConfig struct {
	Title         string
	CompanyName   string
	HeaderColor   string // CSS color
	ShowTimestamp bool
	ShowQuery     bool
	Theme         string // "light" or "dark"
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
