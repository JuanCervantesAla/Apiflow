package nodes

import (
	"bytes"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HttpRequestNode struct{}

type HttpParams struct {
	Method             string                 `json:"method"`
	URL                string                 `json:"url"`
	Headers            map[string]string      `json:"headers"`
	Body               map[string]interface{} `json:"body"`
	Timeout            int                    `json:"timeout"`
	MaxRetries         int                    `json:"maxRetries"`         // Número máximo de reintentos (0 = sin retry)
	RetryDelay         int                    `json:"retryDelay"`         // Delay inicial en ms entre reintentos
	BackoffMultiplier  float64                `json:"backoffMultiplier"`  // Multiplicador para backoff exponencial
	RetryOnStatusCodes []int                  `json:"retryOnStatusCodes"` // Códigos de estado que disparan retry
}

func (n *HttpRequestNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params HttpParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid http parameters: %v", err)
	}

	if params.Method == "" {
		params.Method = "GET"
	}
	if params.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if params.Timeout <= 0 {
		params.Timeout = 30000
	}

	// Defaults para retry
	if params.MaxRetries < 0 {
		params.MaxRetries = 0
	}
	if params.MaxRetries > 10 {
		params.MaxRetries = 10 // Límite de seguridad
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 1000 // 1 segundo por defecto
	}
	if params.BackoffMultiplier <= 0 {
		params.BackoffMultiplier = 2.0 // Backoff exponencial por defecto
	}
	// Si no se especifican códigos, retry en errores de servidor y timeout
	if len(params.RetryOnStatusCodes) == 0 {
		params.RetryOnStatusCodes = []int{408, 429, 500, 502, 503, 504}
	}

	url := interpolateString(params.URL, prev)

	headers := make(map[string]string)
	for k, v := range params.Headers {
		headers[k] = interpolateString(v, prev)
	}

	client := &http.Client{
		Timeout: time.Duration(params.Timeout) * time.Millisecond,
	}

	// Ejecutar request con retry logic
	var lastErr error
	var resp *http.Response
	var respBody []byte
	totalStartTime := time.Now()
	attempts := 0
	currentDelay := params.RetryDelay
	totalRetryTime := int64(0)

	for attempts = 0; attempts <= params.MaxRetries; attempts++ {
		// Si no es el primer intento, aplicar delay
		if attempts > 0 {
			time.Sleep(time.Duration(currentDelay) * time.Millisecond)
			totalRetryTime += int64(currentDelay)
			currentDelay = int(float64(currentDelay) * params.BackoffMultiplier)
		}

		// Recrear bodyReader para cada intento
		var requestBody io.Reader
		if params.Body != nil && len(params.Body) > 0 {
			interpolatedBody := interpolateMap(params.Body, prev)
			bodyBytes, err := json.Marshal(interpolatedBody)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %v", err)
			}
			requestBody = bytes.NewBuffer(bodyBytes)
		}

		req, err := http.NewRequest(strings.ToUpper(params.Method), url, requestBody)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %v", err)
			continue
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		if requestBody != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err = client.Do(req)

		if err != nil {
			lastErr = fmt.Errorf("request failed: %v", err)
			// Si hay error de red, reintentar
			if attempts < params.MaxRetries {
				continue
			}
			break
		}

		// Leer response body
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %v", err)
			if attempts < params.MaxRetries {
				continue
			}
			break
		}

		// Verificar si el status code requiere retry
		shouldRetry := false
		for _, code := range params.RetryOnStatusCodes {
			if resp.StatusCode == code {
				shouldRetry = true
				break
			}
		}

		// Si no necesita retry o ya no hay más intentos, salir
		if !shouldRetry || attempts >= params.MaxRetries {
			break
		}

		lastErr = fmt.Errorf("status code %d requires retry", resp.StatusCode)
	}

	// Si todos los intentos fallaron
	if lastErr != nil && (resp == nil || resp.StatusCode >= 400) {
		return nil, lastErr
	}

	totalDuration := time.Since(totalStartTime).Milliseconds()

	var parsedBody interface{}
	if err := json.Unmarshal(respBody, &parsedBody); err != nil {
		parsedBody = string(respBody)
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	result := map[string]interface{}{
		"statusCode": resp.StatusCode,
		"status":     resp.Status,
		"body":       parsedBody,
		"headers":    respHeaders,
		"durationMs": totalDuration,
		"success":    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	// Agregar información de retry si hubo reintentos
	if params.MaxRetries > 0 {
		result["attempts"] = attempts + 1
		result["retried"] = attempts > 0
		if attempts > 0 {
			result["totalRetryTime"] = totalRetryTime
		}
	}

	return result, nil
}
