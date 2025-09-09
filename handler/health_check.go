package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (rs *ReportService) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database connection
	dbStatus := "disconnected"
	if rs.db != nil {
		if err := rs.db.Ping(); err == nil {
			dbStatus = "connected"
		}
	}

	// Determine overall health status
	healthStatus := "healthy"
	if dbStatus == "disconnected" {
		healthStatus = "unhealthy"
		http.Error(w, `{"error":"Database connection failed"}`, http.StatusServiceUnavailable)
		return
	}

	response := HealthResponse{
		Status:    healthStatus,
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   "report-generator",
		Version:   "1.0.0",
		Database:  dbStatus,
	}

	fmt.Println(healthStatus)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
