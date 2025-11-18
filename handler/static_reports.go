package handler

import (
	"database/sql"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// resolveReportFileByToken looks up filename and stored file_path from category_table using the token.
func resolveReportFileByToken(token string) (filename, filePath string, err error) {
	if categoryDB == nil {
		return "", "", fmt.Errorf("category database not initialized")
	}

	query := `
		SELECT filename, file_path
		FROM user_connection.category_table
		WHERE user_token = $1
		  AND (expiry_time IS NULL OR expiry_time > NOW())
		LIMIT 1
	`

	err = categoryDB.QueryRow(query, token).Scan(&filename, &filePath)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("token not found or expired")
	}
	if err != nil {
		return "", "", err
	}

	return filename, filePath, nil
}

// findExistingFile tries several locations to find the report file.
func findExistingFile(filename, storedPath string) (string, error) {
	// 1) stored path from DB
	if storedPath != "" {
		if info, err := os.Stat(storedPath); err == nil && !info.IsDir() {
			return storedPath, nil
		}
	}

	// 2) ./reports/<filename>
	candidate := filepath.Join(".", "reports", filename)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}

	// 3) temp dir
	candidate = filepath.Join(os.TempDir(), filename)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}

	return "", fmt.Errorf("file not found for %s", filename)
}

func contentTypeFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}

	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".csv":
		return "text/csv"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".html", ".htm":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

// DirectDownloadHandler serves the file as an attachment for the given token.
func DirectDownloadHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token query parameter"})
		return
	}

	filename, storedPath, err := resolveReportFileByToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found or expired"})
		return
	}

	fullPath, err := findExistingFile(filename, storedPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report file not found on disk"})
		return
	}

	c.Header("Content-Type", contentTypeFromFilename(filename))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filename)))
	c.File(fullPath)
}

// PreviewHandler streams the file inline for preview (primarily PDFs/HTML).
func PreviewHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token query parameter"})
		return
	}

	filename, storedPath, err := resolveReportFileByToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found or expired"})
		return
	}

	fullPath, err := findExistingFile(filename, storedPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report file not found on disk"})
		return
	}

	c.Header("Content-Type", contentTypeFromFilename(filename))
	c.File(fullPath)
}
