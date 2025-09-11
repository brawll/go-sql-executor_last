package handler

import (
	"fmt"
	"net/http"
	"time"

	"go-sql-executor/models"

	"github.com/gin-gonic/gin"
)

func (rs *ReportService) HealthCheckHandler(c *gin.Context) {
	// Check database connection
	dbStatus := "disconnected"
	if rs.Db != nil {
		if err := rs.Db.Ping(); err == nil {
			dbStatus = "connected"
		}
	}

	// Determine overall health status
	healthStatus := "healthy"
	if dbStatus == "disconnected" {
		healthStatus = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
		return
	}

	response := models.HealthResponse{
		Status:    healthStatus,
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   "report-generator",
		Version:   "1.0.0",
		Database:  dbStatus,
	}

	fmt.Println(healthStatus)

	c.JSON(http.StatusOK, response)
}
