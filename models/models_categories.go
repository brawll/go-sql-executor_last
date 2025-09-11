package models

import (
	"time"
)

// Data models for categories
type CategoryReportResponse struct {
	ID             string    `json:"id"`
	CategoryName   string    `json:"category_name"`
	ReportName     string    `json:"report_name"`
	ReportFilename string    `json:"report_filename"`
	DownloadUrl    string    `json:"download_url"`
	PreviewUrl     string    `json:"preview_url"`
	CreatedAt      time.Time `json:"created_at"`
}

type CategoryCreate struct {
	CategoryName string `json:"category_name" binding:"required"`
	Description  string `json:"description"`
}

type CategoryResponse struct {
	ID           string    `json:"id"`
	CategoryName string    `json:"category_name"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type BusinessHoursFilter struct {
	BusinessHours  string `json:"business_hours"`
	TimezoneOffset int    `json:"timezone_offset"`
	TimeRange      string `json:"time_range"`
}

type ReportUpdateRequest struct {
	StartDate           string               `json:"start_date" binding:"required"`
	EndDate             string               `json:"end_date" binding:"required"`
	BusinessHoursFilter *BusinessHoursFilter `json:"business_hours_filter,omitempty"`
}

type ScheduleReportRequest struct {
	ScheduleType   string   `json:"schedule_type" binding:"required"`
	MinutesFromNow int      `json:"minutes_from_now,omitempty"`
	ScheduledTimes []string `json:"scheduled_times,omitempty"`
	WeeklyDay      string   `json:"weekly_day,omitempty"`
	WeeklyTime     string   `json:"weekly_time,omitempty"`
	MonthlyDay     int      `json:"monthly_day,omitempty"`
	MonthlyTime    string   `json:"monthly_time,omitempty"`
	YearlyMonth    int      `json:"yearly_month,omitempty"`
	YearlyDay      int      `json:"yearly_day,omitempty"`
	YearlyTime     string   `json:"yearly_time,omitempty"`
}

type ScheduledReportResponse struct {
	ID                   string     `json:"id"`
	OriginalReportID     string     `json:"original_report_id"`
	OriginalCategoryName string     `json:"original_category_name"`
	ReportName           string     `json:"report_name"`
	ReportFilename       string     `json:"report_filename"`
	ScheduledTime        string     `json:"scheduled_time"`
	IsActive             bool       `json:"is_active"`
	DownloadUrl          string     `json:"download_url"`
	PreviewUrl           string     `json:"preview_url"`
	CreatedAt            time.Time  `json:"created_at"`
	LastExecutedAt       *time.Time `json:"last_executed_at"`
	NextExecutionAt      *time.Time `json:"next_execution_at,omitempty"`
	ExecutionCount       int        `json:"execution_count"`
}
