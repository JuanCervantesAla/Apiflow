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
	ConnectionURL  string                 `json:"connectionUrl"`  // URL de conexión a la base de datos
	Query          string                 `json:"query"`          // SQL query a ejecutar
	Timeout        int                    `json:"timeout"`        // Timeout en ms (default: 30000)
	MaxRetries     int                    `json:"maxRetries"`     // Número de reintentos en caso de error
	RetryDelay     int                    `json:"retryDelay"`     // Delay entre reintentos en ms
	QueryParams    map[string]interface{} `json:"queryParams"`    // Parámetros para query parametrizada
	ReturnMetadata bool                   `json:"returnMetadata"` // Si true, incluye metadata de la query
}

type DatabaseResult struct {
	Rows          []map[string]interface{} `json:"rows"`
	RowCount      int                      `json:"rowCount"`
	AffectedRows  int64                    `json:"affectedRows,omitempty"` // Para INSERT/UPDATE/DELETE
	LastInsertID  int64                    `json:"lastInsertId,omitempty"` // Para INSERT
	ExecutionTime int64                    `json:"executionTime"`          // Tiempo de ejecución en ms
	Query         string                   `json:"query,omitempty"`        // Query ejecutada (si returnMetadata=true)
}

func (n *DatabaseNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params DatabaseParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid database parameters: %v", err)
	}

	// Validaciones básicas
	if params.Driver == "" {
		return nil, fmt.Errorf("driver is required (postgres, mysql, or sqlite3)")
	}
	if params.ConnectionURL == "" {
		return nil, fmt.Errorf("connectionUrl is required")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Aplicar defaults
	if params.Timeout <= 0 {
		params.Timeout = 30000 // 30 segundos
	}
	if params.MaxRetries < 0 {
		params.MaxRetries = 0
	}
	if params.MaxRetries > 5 {
		params.MaxRetries = 5 // Límite de seguridad
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 1000 // 1 segundo
	}

	// Interpolar variables en la query
	query := interpolateString(params.Query, prev)
	connectionURL := interpolateString(params.ConnectionURL, prev)

	// Interpolar parámetros de query
	interpolatedParams := make(map[string]interface{})
	for key, value := range params.QueryParams {
		if strVal, ok := value.(string); ok {
			interpolatedParams[key] = interpolateString(strVal, prev)
		} else {
			interpolatedParams[key] = value
		}
	}

	// Ejecutar con reintentos
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
			break // Éxito
		}

		// Si es el último intento, retornar el error
		if attempt == params.MaxRetries {
			return nil, fmt.Errorf("database query failed after %d attempts: %v", attempt+1, lastErr)
		}
	}

	result.ExecutionTime = time.Since(startTime).Milliseconds()

	// Agregar metadata si se solicita
	if params.ReturnMetadata {
		result.Query = query
	}

	// Convertir result a map[string]interface{}
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

// executeDatabaseQuery ejecuta una query SQL y retorna los resultados
func executeDatabaseQuery(
	driver string,
	connectionURL string,
	query string,
	params map[string]interface{},
	timeoutMs int,
) (DatabaseResult, error) {

	var result DatabaseResult

	// Abrir conexión a la base de datos
	db, err := sql.Open(driver, connectionURL)
	if err != nil {
		return result, fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configurar timeout de conexión
	db.SetConnMaxLifetime(time.Duration(timeoutMs) * time.Millisecond)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Verificar conexión
	if err := db.Ping(); err != nil {
		return result, fmt.Errorf("failed to ping database: %v", err)
	}

	// Determinar si es una query de selección o modificación
	queryType := strings.ToUpper(strings.TrimSpace(query))
	isSelect := strings.HasPrefix(queryType, "SELECT") || strings.HasPrefix(queryType, "SHOW") || strings.HasPrefix(queryType, "DESCRIBE")

	if isSelect {
		// Ejecutar SELECT
		return executeSelectQuery(db, query, params)
	} else {
		// Ejecutar INSERT/UPDATE/DELETE
		return executeModifyQuery(db, query, params)
	}
}

// executeSelectQuery ejecuta una query SELECT y retorna las filas
func executeSelectQuery(
	db *sql.DB,
	query string,
	params map[string]interface{},
) (DatabaseResult, error) {

	var result DatabaseResult

	// Reemplazar parámetros nombrados por valores
	args := make([]interface{}, 0)
	finalQuery := query
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(finalQuery, placeholder) {
			finalQuery = strings.ReplaceAll(finalQuery, placeholder, "?")
			args = append(args, value)
		}
	}

	// Ejecutar query
	rows, err := db.Query(finalQuery, args...)
	if err != nil {
		return result, fmt.Errorf("query execution failed: %v", err)
	}
	defer rows.Close()

	// Obtener nombres de columnas
	columns, err := rows.Columns()
	if err != nil {
		return result, fmt.Errorf("failed to get column names: %v", err)
	}

	// Leer filas
	result.Rows = make([]map[string]interface{}, 0)
	for rows.Next() {
		// Crear slice para escanear valores
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return result, fmt.Errorf("failed to scan row: %v", err)
		}

		// Convertir a map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Convertir []byte a string (común en MySQL/PostgreSQL)
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

// executeModifyQuery ejecuta INSERT/UPDATE/DELETE y retorna filas afectadas
func executeModifyQuery(
	db *sql.DB,
	query string,
	params map[string]interface{},
) (DatabaseResult, error) {

	var result DatabaseResult

	// Reemplazar parámetros nombrados por valores
	args := make([]interface{}, 0)
	finalQuery := query
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(finalQuery, placeholder) {
			finalQuery = strings.ReplaceAll(finalQuery, placeholder, "?")
			args = append(args, value)
		}
	}

	// Ejecutar query
	execResult, err := db.Exec(finalQuery, args...)
	if err != nil {
		return result, fmt.Errorf("query execution failed: %v", err)
	}

	// Obtener filas afectadas
	affectedRows, err := execResult.RowsAffected()
	if err == nil {
		result.AffectedRows = affectedRows
	}

	// Obtener last insert ID (solo para INSERT)
	lastID, err := execResult.LastInsertId()
	if err == nil && lastID > 0 {
		result.LastInsertID = lastID
	}

	result.RowCount = int(affectedRows)

	return result, nil
}
