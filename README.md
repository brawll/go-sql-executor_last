# SQL Report Generator API

[![OpenAPI 3.0](https://img.shields.io/badge/OpenAPI-3.0-green.svg)](openapi.yml)

A REST API for executing SQL queries and generating reports in multiple formats (CSV, Excel, HTML, PDF) that conforms to OpenAPI 3.0.3 standard.

## 📖 Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [API Documentation](#api-documentation)
- [OpenAPI Specification](#openapi-specification)
- [Response Format](#response-format)
- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Examples](#examples)
- [Contributing](#contributing)

## ✨ Features

- ✅ **OpenAPI 3.0.3 Compliant** - Fully documented REST API conforming to OpenAPI standards
- 📊 **Multiple Export Formats** - Generate reports in CSV, Excel, HTML, and PDF formats
- 🔒 **Structured Error Handling** - Consistent error responses with HTTP status codes
- 📝 **Request Validation** - Strict JSON validation with detailed error messages
- 💾 **File Generation** - Automatic file creation with metadata tracking
- 🏥 **Health Checks** - Endpoint monitoring with database connectivity status
- 🔐 **Security Ready** - Prepared for API key authentication

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Database connection (PostgreSQL or MySQL)

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd go-sql-executor

# Install dependencies
go mod tidy

# Start the server
go run main.go server.go
```

### Environment Configuration

Create a `.env` file with your database configuration:

```env
DB_DRIVER=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_database
SSL_MODE=require
```

### Launch the Server

```bash
go run main.go server.go
```

The server will start on `http://localhost:8081`

## 📚 API Documentation

### Available Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check with database connectivity status |
| `POST` | `/api/v1/generate-report` | Generate reports from SQL queries |

## 📋 OpenAPI Specification

The API is fully documented in the [openapi.yml](openapi.yml) file, which complies with OpenAPI 3.0.3 specification. This file defines:

- Request/response schemas
- Authentication requirements
- Error response formats
- Parameter validation rules
- Security schemes

### Key Components

#### Request Schemas

**ReportRequest**
```json
{
  "query": "SELECT * FROM users WHERE status = 'active'",
  "parameters": {
    "user_id": "123",
    "date_from": "2025-01-01"
  }
}
```

#### Response Schemas

**ReportResponse**
```json
{
  "success": true,
  "message": "Report generated successfully",
  "data": { ... },
  "files": [ ... ],
  "summary": { ... }
}
```

**HealthResponse**
```json
{
  "status": "healthy",
  "timestamp": "2025-09-04T03:54:36Z",
  "service": "report-generator",
  "version": "1.0.0",
  "database": "connected"
}
```

## 📤 Response Format

### Successful Report Generation

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "message": "Report generated successfully",
  "data": {
    "query": "SELECT * FROM users",
    "data": {
      "columns": ["id", "name", "email"],
      "rows": [
        { "id": 1, "name": "John Doe", "email": "john@example.com" },
        { "id": 2, "name": "Jane Smith", "email": "jane@example.com" }
      ]
    },
    "status": "success",
    "duration": "25.3ms",
    "timestamp": "2025-09-04T03:54:36Z"
  },
  "files": [
    {
      "type": "csv",
      "filename": "query_results_combined.csv",
      "path": "/app/reports/query_results_combined.csv",
      "size_bytes": 1024,
      "columns": 3,
      "rows": 2
    }
  ],
  "summary": {
    "total_queries": 1,
    "successful_queries": 1,
    "failed_queries": 0,
    "total_execution_time": "1.25s"
  }
}
```

### Error Response

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json

{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": {
      "field": "query",
      "reason": "query is required"
    }
  },
  "timestamp": "2025-09-04T03:54:36Z"
}
```

## 🔐 Authentication

The API includes security schemes for authentication (currently optional):

### API Key Authentication

```bash
curl -X POST http://localhost:8081/api/v1/generate-report \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT * FROM users"}'
```

## 🛠 Examples

### 1. Generate CSV Report

```bash
curl -X POST http://localhost:8081/api/v1/generate-report \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT id, name, email FROM users WHERE created_at > '\''2025-01-01'\''"
  }'
```

### 2. Health Check

```bash
curl http://localhost:8081/health
```

### 3. Error Example (Invalid JSON)

```bash
curl -X POST http://localhost:8081/api/v1/generate-report \
  -H "Content-Type: application/json" \
  -d '{"query": ""}'
```

Response:
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": {
      "field": "query",
      "reason": "query is required"
    }
  },
  "timestamp": "2025-09-04T03:54:36Z"
}
```

### 4. Method Not Allowed

```bash
curl -X PUT http://localhost:8081/api/v1/generate-report
```

Response:
```json
{
  "success": false,
  "error": {
    "code": "METHOD_NOT_ALLOWED",
    "message": "Method not allowed"
  },
  "timestamp": "2025-09-04T03:54:36Z"
}
```

## 🔍 Error Handling

The API provides consistent error handling with descriptive codes:

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `METHOD_NOT_ALLOWED` | 405 | Invalid HTTP method |
| `INVALID_JSON` | 400 | Malformed JSON payload |
| `VALIDATION_ERROR` | 422 | Request validation failed |
| (General) | 500 | Internal server error |

All error responses follow the same structure with detailed error information.

## 📋 Request Validation

The API implements strict request validation:

- Required fields are marked as such
- JSON structure is validated
- SQL injection protection through query parameter validation
- Type checking for request parameters

## 🔧 Configuration

The server supports various configuration options through environment variables:

- Database connection settings
- Export format toggles
- Server port configuration
- SSL/TLS settings

## 🚀 Deployment

### Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o app

FROM alpine:latest
COPY --from=builder /app/app /app/app
EXPOSE 8081
CMD ["/app/app"]
```

### Systemd Service

Create `/etc/systemd/system/sql-report-api.service`:

```ini
[Unit]
Description=SQL Report Generator API
After=network.target

[Service]
Type=simple
User=api-user
WorkingDirectory=/opt/sql-report-api
ExecStart=/opt/sql-report-api/app
Restart=always

[Install]
WantedBy=multi-user.target
```

## 🔒 Security Considerations

- Input validation and sanitization
- SQL injection protection
- Rate limiting (to be implemented)
- API key authentication support
- File upload size limits (to be implemented)

## 🧪 Testing

The API can be tested using various tools:

- **Curl** (command line)
- **Postman** (GUI)
- **REST Client** (VS Code extension)
- **OpenAPI Generator** (for client SDKs)

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

If you need help:

1. Check the [OpenAPI documentation](openapi.yml)
2. Review the [examples](#examples)
3. Open an issue on GitHub

---

**API Version**: 1.0.0
**OpenAPI Version**: 3.0.3
**Last Updated**: September 2025
