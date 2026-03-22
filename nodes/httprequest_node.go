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
	MaxRetries         int                    `json:"maxRetries"`         // Maximum number of retries (0 = no retry)
	RetryDelay         int                    `json:"retryDelay"`         // Initial delay in ms between retries
	BackoffMultiplier  float64                `json:"backoffMultiplier"`  // Multiplier for exponential backoff
	RetryOnStatusCodes []int                  `json:"retryOnStatusCodes"` // Status codes that trigger retry
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

	// Defaults for retry
	if params.MaxRetries < 0 {
		params.MaxRetries = 0
	}
	if params.MaxRetries > 10 {
		params.MaxRetries = 10 // Safety limit
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 1000 // 1 second by default
	}
	if params.BackoffMultiplier <= 0 {
		params.BackoffMultiplier = 2.0 // Exponential backoff by default
	}
	// If no codes specified, retry on server errors and timeout
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

	// Execute request with retry logic
	var lastErr error
	var resp *http.Response
	var respBody []byte
	totalStartTime := time.Now()
	attempts := 0
	currentDelay := params.RetryDelay
	totalRetryTime := int64(0)

	for attempts = 0; attempts <= params.MaxRetries; attempts++ {
		// If this is not the first attempt, apply delay
		if attempts > 0 {
			time.Sleep(time.Duration(currentDelay) * time.Millisecond)
			totalRetryTime += int64(currentDelay)
			currentDelay = int(float64(currentDelay) * params.BackoffMultiplier)
		}

		// Recreate bodyReader for each attempt
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
			// If there's a network error, retry
			if attempts < params.MaxRetries {
				continue
			}
			break
		}

		// Read response body
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %v", err)
			if attempts < params.MaxRetries {
				continue
			}
			break
		}

		// Check if the status code requires retry
		shouldRetry := false
		for _, code := range params.RetryOnStatusCodes {
			if resp.StatusCode == code {
				shouldRetry = true
				break
			}
		}

		// If no retry needed or no more attempts left, exit
		if !shouldRetry || attempts >= params.MaxRetries {
			break
		}

		lastErr = fmt.Errorf("status code %d requires retry", resp.StatusCode)
	}

	// If all attempts failed
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

	// Add retry information if there were retries
	if params.MaxRetries > 0 {
		result["attempts"] = attempts + 1
		result["retried"] = attempts > 0
		if attempts > 0 {
			result["totalRetryTime"] = totalRetryTime
		}
	}

	return result, nil
}
