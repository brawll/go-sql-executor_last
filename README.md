# Go SQL Executor

A simple yet powerful Go application designed to execute thousands of SQL queries concurrently against a relational database. It is built to be easily configurable and supports both PostgreSQL and MySQL out of the box.

## Features

-   **Concurrent Query Execution:** Leverages Goroutines and Channels to run a large number of SQL queries in parallel, significantly speeding up data processing tasks.
-   **Multi-Database Support:** Easily switch between PostgreSQL and MySQL by changing a single configuration variable.
-   **Easy to Configure:** Database connection strings and the list of queries to be executed are stored in the `main.go` file for straightforward modification.

## Prerequisites

-   [Go](https://golang.org/doc/install) (version 1.18 or later recommended)
-   A running instance of [PostgreSQL](https://www.postgresql.org/download/) or [MySQL](https://www.mysql.com/downloads/)

## Setup

1.  **Clone the repository or download the source code.**
    (Assuming you already have the `go-sql-executor` directory from our previous steps)

2.  **Install dependencies:**
    Navigate to the project directory and let Go automatically fetch the required database drivers based on the `go.mod` file. If you run `go run main.go` or `go build`, Go will handle this automatically. You can also do it manually:
    ```bash
    cd go-sql-executor
    go mod tidy
    ```

## Configuration

1.  **Open `main.go` in your favorite editor.**

2.  **Configure the Database Connection:**
    Locate the `dbConfigs` map and replace the placeholder values with your actual database credentials.

    ```go
    // Replace with your actual database connection strings.
    dbConfigs := map[string]DBConfig{
        "postgres": {
            DriverName:     "postgres",
            DataSourceName: "user=youruser password=yourpassword dbname=yourdb sslmode=disable", // <-- EDIT THIS
        },
        "mysql": {
            DriverName:     "mysql",
            DataSourceName: "youruser:yourpassword@tcp(127.0.0.1:3306)/yourdb", // <-- EDIT THIS
        },
    }
    ```

3.  **Select the Database:**
    Change the `selectedDB` variable to either `"postgres"` or `"mysql"`.

    ```go
    // Choose the database to use.
    selectedDB := "postgres" // or "mysql"
    ```

4.  **Add Your Queries:**
    Find the `queries` slice and populate it with the SQL queries you wish to execute.

    ```go
    // A list of queries to execute.
    // Replace these with your actual queries.
    queries := []string{
        "SELECT * FROM users",
        "SELECT * FROM products WHERE category = 'electronics'",
        "UPDATE inventory SET quantity = 0 WHERE last_updated < '2024-01-01'",
        // Add thousands of your queries here.
    }
    ```

## Usage

Once configured, run the application from within the `go-sql-executor` directory:

```bash
go run main.go
```

The application will connect to the specified database, execute all the queries concurrently, and print the results (or any errors) to the console.
