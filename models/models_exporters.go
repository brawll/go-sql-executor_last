package models

// models_exporter.go
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

	// Enhanced template configuration fields
	LogoPath         *string
	LogoPosition     string  // "left", "center", "right"
	LogoWidth        float64 // Width of logo in points
	LogoHeight       float64 // Height of logo in points
	ReportTitleColor []uint8 // RGB values for title text
	ReportTitleText  *string // Custom report title text
	ContactEnabled   bool
	ContactName      *string
	ContactAddress   *string
	ContactEmail     *string
	ContactPhone     *string
	ContactPosition  string  // "left", "right"
	ContactTextColor []uint8 // RGB values for contact text
	HeaderHeight     float64 // Total header height to reserve space

	// Footer configuration fields
	FooterEnabled   bool
	FooterText      *string
	FooterPosition  string  // "left", "center", "right" - default "center"
	FooterTextColor []uint8 // RGB values for footer text
	FooterHeight    float64 // Footer height to reserve space
}
