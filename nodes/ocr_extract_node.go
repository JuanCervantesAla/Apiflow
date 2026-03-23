package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type OCRExtractNode struct{}

type OCRExtractParams struct {
	FileURL           string `json:"fileUrl"`
	FileContentBase64 string `json:"fileContentBase64"`
	MimeType          string `json:"mimeType"`
	Language          string `json:"language"`
	Engine            string `json:"engine"`
}

type ocrSpaceResponse struct {
	OCRExitCode           int  `json:"OCRExitCode"`
	IsErroredOnProcessing bool `json:"IsErroredOnProcessing"`
	ErrorMessage          any  `json:"ErrorMessage"`
	ParsedResults         []struct {
		ParsedText string `json:"ParsedText"`
	} `json:"ParsedResults"`
}

func (n *OCRExtractNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params OCRExtractParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing ocr-extract params: %v", err)
	}

	params.FileURL = strings.TrimSpace(interpolateString(params.FileURL, prev))
	params.FileContentBase64 = strings.TrimSpace(interpolateString(params.FileContentBase64, prev))
	params.MimeType = strings.TrimSpace(interpolateString(params.MimeType, prev))
	params.Language = strings.TrimSpace(interpolateString(params.Language, prev))
	params.Engine = strings.TrimSpace(strings.ToLower(interpolateString(params.Engine, prev)))

	if params.Engine == "" {
		params.Engine = "ocrspace"
	}
	if params.Engine != "ocrspace" {
		return nil, fmt.Errorf("unsupported OCR engine: %s", params.Engine)
	}
	if params.Language == "" {
		params.Language = "eng"
	}

	apiKey := strings.TrimSpace(os.Getenv("OCR_SPACE_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("OCR_SPACE_API_KEY not configured in environment")
	}

	form := url.Values{}
	form.Set("language", params.Language)
	form.Set("isOverlayRequired", "false")

	if params.FileURL != "" {
		form.Set("url", params.FileURL)
	} else if params.FileContentBase64 != "" {
		prefix := "data:application/octet-stream;base64,"
		if params.MimeType != "" {
			prefix = "data:" + params.MimeType + ";base64,"
		}
		form.Set("base64Image", prefix+params.FileContentBase64)
	} else {
		return nil, fmt.Errorf("fileUrl or fileContentBase64 is required")
	}

	req, err := http.NewRequest("POST", "https://api.ocr.space/parse/image", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create OCR request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apikey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OCR service: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR response: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ocr service error (%d): %s", resp.StatusCode, string(body))
	}

	var parsed ocrSpaceResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse OCR response: %v", err)
	}

	if parsed.IsErroredOnProcessing {
		return nil, fmt.Errorf("ocr processing error: %v", parsed.ErrorMessage)
	}

	parts := make([]string, 0, len(parsed.ParsedResults))
	for _, result := range parsed.ParsedResults {
		if strings.TrimSpace(result.ParsedText) != "" {
			parts = append(parts, strings.TrimSpace(result.ParsedText))
		}
	}

	text := strings.Join(parts, "\n")

	return map[string]interface{}{
		"text":        text,
		"language":    params.Language,
		"engine":      params.Engine,
		"hasText":     strings.TrimSpace(text) != "",
		"rawExitCode": parsed.OCRExitCode,
	}, nil
}
