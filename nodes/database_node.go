package nodes

import (
	"capyflow/api/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type DatabaseNode struct{}

type DatabaseParams struct {
	Driver         string                 `json:"driver"`         // "postgres", "mysql", "sqlite3"
	ConnectionURL  string                 `json:"connectionUrl"`  // Database connection URL
	Query          string                 `json:"query"`          // SQL query to execute
	Timeout        int                    `json:"timeout"`        // Timeout in ms (default: 30000)
	MaxRetries     int                    `json:"maxRetries"`     // Number of retries on error
	RetryDelay     int                    `json:"retryDelay"`     // Delay between retries in ms
	QueryParams    map[string]interface{} `json:"queryParams"`    // Parameters for parameterized query
	ReturnMetadata bool                   `json:"returnMetadata"` // If true, includes query metadata
}

type DatabaseResult struct {
	Rows          []map[string]interface{} `json:"rows"`
	RowCount      int                      `json:"rowCount"`
	AffectedRows  int64                    `json:"affectedRows,omitempty"` // For INSERT/UPDATE/DELETE
	LastInsertID  int64                    `json:"lastInsertId,omitempty"` // For INSERT
	ExecutionTime int64                    `json:"executionTime"`          // Execution time in ms
	Query         string                   `json:"query,omitempty"`        // Executed query (if returnMetadata=true)
}

func (n *DatabaseNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params DatabaseParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid database parameters: %v", err)
	}

	// Basic validations
	if params.Driver == "" {
		return nil, fmt.Errorf("driver is required (postgres, mysql, or sqlite3)")
	}
	if params.ConnectionURL == "" {
		return nil, fmt.Errorf("connectionUrl is required")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Apply defaults
	if params.Timeout <= 0 {
		params.Timeout = 30000 // 30 seconds
	}
	if params.MaxRetries < 0 {
		params.MaxRetries = 0
	}
	if params.MaxRetries > 5 {
		params.MaxRetries = 5 // Safety limit
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 1000 // 1 second
	}

	// Interpolate variables in the query
	query := interpolateString(params.Query, prev)
	connectionURL := interpolateString(params.ConnectionURL, prev)

	// Interpolate query parameters
	interpolatedParams := make(map[string]interface{})
	for key, value := range params.QueryParams {
		if strVal, ok := value.(string); ok {
			interpolatedParams[key] = interpolateString(strVal, prev)
		} else {
			interpolatedParams[key] = value
		}
	}

	// Execute with retries
	var result DatabaseResult
	var lastErr error

	startTime := time.Now()

	for attempt := 0; attempt <= params.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(params.RetryDelay) * time.Millisecond
			time.Sleep(delay)
		}

		result, lastErr = executeDatabaseQuery(
			params.Driver,
			connectionURL,
			query,
			interpolatedParams,
			params.Timeout,
		)

		if lastErr == nil {
			break // Success
		}

		// If this is the last attempt, return the error
		if attempt == params.MaxRetries {
			return nil, fmt.Errorf("database query failed after %d attempts: %v", attempt+1, lastErr)
		}
	}

	result.ExecutionTime = time.Since(startTime).Milliseconds()

	// Add metadata if requested
	if params.ReturnMetadata {
		result.Query = query
	}

	// Convert result to map[string]interface{}
	resultMap := map[string]interface{}{
		"rows":          result.Rows,
		"rowCount":      result.RowCount,
		"executionTime": result.ExecutionTime,
	}

	if result.AffectedRows > 0 {
		resultMap["affectedRows"] = result.AffectedRows
	}
	if result.LastInsertID > 0 {
		resultMap["lastInsertId"] = result.LastInsertID
	}
	if params.ReturnMetadata {
		resultMap["query"] = result.Query
	}

	return resultMap, nil
}

// executeDatabaseQuery executes a SQL query and returns the results
func executeDatabaseQuery(
	driver string,
	connectionURL string,
	query string,
	params map[string]interface{},
	timeoutMs int,
) (DatabaseResult, error) {

	var result DatabaseResult

	// Open database connection
	db, err := sql.Open(driver, connectionURL)
	if err != nil {
		return result, fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection timeout
	db.SetConnMaxLifetime(time.Duration(timeoutMs) * time.Millisecond)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Verify connection
	if err := db.Ping(); err != nil {
		return result, fmt.Errorf("failed to ping database: %v", err)
	}

	// Determine if it's a select or modification query
	queryType := strings.ToUpper(strings.TrimSpace(query))
	isSelect := strings.HasPrefix(queryType, "SELECT") || strings.HasPrefix(queryType, "SHOW") || strings.HasPrefix(queryType, "DESCRIBE")

	if isSelect {
		// Execute SELECT
		return executeSelectQuery(db, query, params)
	} else {
		// Execute INSERT/UPDATE/DELETE
		return executeModifyQuery(db, query, params)
	}
}

// executeSelectQuery executes a SELECT query and returns the rows
func executeSelectQuery(
	db *sql.DB,
	query string,
	params map[string]interface{},
) (DatabaseResult, error) {

	var result DatabaseResult

	// Replace named parameters with values
	args := make([]interface{}, 0)
	finalQuery := query
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(finalQuery, placeholder) {
			finalQuery = strings.ReplaceAll(finalQuery, placeholder, "?")
			args = append(args, value)
		}
	}

	// Execute query
	rows, err := db.Query(finalQuery, args...)
	if err != nil {
		return result, fmt.Errorf("query execution failed: %v", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return result, fmt.Errorf("failed to get column names: %v", err)
	}

	// Read rows
	result.Rows = make([]map[string]interface{}, 0)
	for rows.Next() {
		// Create slice to scan values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return result, fmt.Errorf("failed to scan row: %v", err)
		}

		// Convert to map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Convert []byte to string (common in MySQL/PostgreSQL)
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		result.Rows = append(result.Rows, rowMap)
	}

	result.RowCount = len(result.Rows)

	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("error iterating rows: %v", err)
	}

	return result, nil
}

// executeModifyQuery executes INSERT/UPDATE/DELETE and returns affected rows
func executeModifyQuery(
	db *sql.DB,
	query string,
	params map[string]interface{},
) (DatabaseResult, error) {

	var result DatabaseResult

	// Replace named parameters with values
	args := make([]interface{}, 0)
	finalQuery := query
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(finalQuery, placeholder) {
			finalQuery = strings.ReplaceAll(finalQuery, placeholder, "?")
			args = append(args, value)
		}
	}

	// Execute query
	execResult, err := db.Exec(finalQuery, args...)
	if err != nil {
		return result, fmt.Errorf("query execution failed: %v", err)
	}

	// Get affected rows
	affectedRows, err := execResult.RowsAffected()
	if err == nil {
		result.AffectedRows = affectedRows
	}

	// Get last insert ID (only for INSERT)
	lastID, err := execResult.LastInsertId()
	if err == nil && lastID > 0 {
		result.LastInsertID = lastID
	}

	result.RowCount = int(affectedRows)

	return result, nil
}
