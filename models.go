package main

import (
	"go-sql-executor/exporters"
)

// DBConfig holds the configuration for a database connection.
type DBConfig struct {
	DriverName     string
	DataSourceName string
}

// Type aliases to exporters package types to avoid duplication
type QueryData = exporters.QueryData
type QueryResult = exporters.QueryResult
type PDFConfig = exporters.PDFConfig

// HTMLConfig holds HTML generation configuration.
type HTMLConfig struct {
	Title         string
	CompanyName   string
	HeaderColor   string // CSS color
	ShowTimestamp bool
	ShowQuery     bool
	Theme         string // "light" or "dark"
}
