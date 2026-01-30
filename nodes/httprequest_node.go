package nodes

import (
	"bytes"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpRequestNode struct{}

type HttpParams struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	BodyKey   string            `json:"bodyKey"`
	TimeoutMs int               `json:"timeoutMs"`
}

func (n *HttpRequestNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params HttpParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid http parameters")
	}

	if params.Method == "" || params.URL == "" {
		return nil, fmt.Errorf("method and url are required")
	}

	var bodyData interface{}
	if params.BodyKey != "" {
		for _, out := range prev {
			if v, ok := out[params.BodyKey]; ok {
				bodyData = v
				break
			}
		}
	}

	var body io.Reader
	if bodyData != nil {
		b, err := json.Marshal(bodyData)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize body")
		}
		body = bytes.NewBuffer(b)
	}

	client := &http.Client{
		Timeout: time.Duration(params.TimeoutMs) * time.Millisecond,
	}

	req, err := http.NewRequest(params.Method, params.URL, body)
	if err != nil {
		return nil, err
	}

	for k, v := range params.Headers {
		req.Header.Set(k, v)
	}

	if bodyData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var parsed interface{}
	_ = json.Unmarshal(respBody, &parsed)

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"body":    parsed,
		"rawBody": string(respBody),
	}, nil
}
