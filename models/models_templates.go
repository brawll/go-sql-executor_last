package models

import (
	"time"

	"github.com/google/uuid"
)

// ReportTemplate represents the database model
type ReportTemplate struct {
	ID                  uuid.UUID `json:"id" db:"id"`
	Name                string    `json:"name" db:"name"`
	Logo                *string   `json:"logo" db:"logo"`
	LogoPosition        string    `json:"logo_position" db:"logo_position"`
	ReportTitle         *string   `json:"report_title" db:"report_title"`
	ReportTitlePosition string    `json:"report_title_position" db:"report_title_position"`
	ContactEnabled      bool      `json:"contact_enabled" db:"contact_enabled"`
	ContactName         *string   `json:"contact_name" db:"contact_name"`
	ContactAddress      *string   `json:"contact_address" db:"contact_address"`
	ContactEmail        *string   `json:"contact_email" db:"contact_email"`
	ContactPhone        *string   `json:"contact_phone" db:"contact_phone"`
	ContactPosition     string    `json:"contact_position" db:"contact_position"`
	ContactTextColor    string    `json:"contact_text_color" db:"contact_text_color"`
	FooterEnabled       bool      `json:"footer_enabled" db:"footer_enabled"`
	FooterText          *string   `json:"footer_text" db:"footer_text"`
	FooterPosition      string    `json:"footer_position" db:"footer_position"`
	TableHeadBgColor    string    `json:"table_head_bg_color" db:"table_head_bg_color"`
	TableHeadTextColor  string    `json:"table_head_text_color" db:"table_head_text_color"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// Request/Response models
type ReportTemplateCreate struct {
	Name                string         `json:"name"`
	Logo                *string        `json:"logo,omitempty"`
	LogoPosition        string         `json:"logoPosition"`
	ReportTitle         *string        `json:"reportTitle,omitempty"`
	ReportTitlePosition string         `json:"reportTitlePosition"`
	ContactDetails      ContactDetails `json:"contactDetails"`
	Footer              Footer         `json:"footer"`
	TableHeadBgColor    string         `json:"tableHeadBgColor"`
	TableHeadTextColor  string         `json:"tableHeadTextColor"`
}

// Frontend-compatible request model (for CUSTOM initialization)
type FrontendTemplateCreate struct {
	Name         string         `json:"name"`
	ConnectionID string         `json:"connection_id,omitempty"`
	SchemaName   string         `json:"schema_name,omitempty"`
	TableName    string         `json:"table_name,omitempty"`
	CategoryName string         `json:"category_name,omitempty"`
	Logo         *string        `json:"logo,omitempty"`
	LogoPosition string         `json:"logoPosition,omitempty"`
	Contact      ContactDetails `json:"contact,omitempty"`    // Frontend uses "contact" instead of "contactDetails"
	TheadColor   string         `json:"theadColor,omitempty"` // Frontend uses "theadColor" instead of "tableHeadBgColor"

	// Optional fields that might be missing from frontend
	ContactDetails     ContactDetails `json:"contactDetails,omitempty"`
	Footer             Footer         `json:"footer,omitempty"`
	TableHeadBgColor   string         `json:"tableHeadBgColor,omitempty"`
	TableHeadTextColor string         `json:"tableHeadTextColor,omitempty"`
	ReportTitle        *string        `json:"reportTitle,omitempty"`
	ReportTitlePos     string         `json:"reportTitlePosition,omitempty"`
}

type ContactDetails struct {
	Enabled   bool    `json:"enabled"`
	Name      *string `json:"name,omitempty"`
	Address   *string `json:"address,omitempty"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Position  string  `json:"position"`
	TextColor string  `json:"textColor"`
}

type Footer struct {
	Enabled  bool    `json:"enabled"`
	Text     *string `json:"text,omitempty"`
	Position string  `json:"position"`
}

type ReportTemplateResponse struct {
	ID                  uuid.UUID      `json:"id"`
	Name                string         `json:"name"`
	Logo                *string        `json:"logo"`
	LogoPosition        string         `json:"logoPosition"`
	ReportTitle         *string        `json:"reportTitle"`
	ReportTitlePosition string         `json:"reportTitlePosition"`
	ContactDetails      ContactDetails `json:"contactDetails"`
	Footer              Footer         `json:"footer"`
	TableHeadBgColor    string         `json:"tableHeadBgColor"`
	TableHeadTextColor  string         `json:"tableHeadTextColor"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}
