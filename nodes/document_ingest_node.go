package nodes

import (
	"bytes"
	"capyflow/api/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type DocumentIngestNode struct{}

type DocumentIngestParams struct {
	InputData         string `json:"inputData"`
	FileContentBase64 string `json:"fileContentBase64"`
	FileName          string `json:"fileName"`
	MimeType          string `json:"mimeType"`
	MaxChars          int    `json:"maxChars"`
}

func (n *DocumentIngestNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params DocumentIngestParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing document-ingest params: %v", err)
	}

	params.InputData = interpolateString(params.InputData, prev)
	params.FileContentBase64 = interpolateString(params.FileContentBase64, prev)
	params.FileName = strings.TrimSpace(interpolateString(params.FileName, prev))
	params.MimeType = strings.TrimSpace(interpolateString(params.MimeType, prev))

	if params.MaxChars <= 0 {
		params.MaxChars = 20000
	}

	contentBytes := []byte(params.InputData)
	if strings.TrimSpace(params.FileContentBase64) != "" {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(params.FileContentBase64))
		if err != nil {
			return nil, fmt.Errorf("invalid fileContentBase64: %v", err)
		}
		contentBytes = decoded
	}

	if len(contentBytes) == 0 {
		return nil, fmt.Errorf("document content is required")
	}

	detectedMime := params.MimeType
	if detectedMime == "" {
		detectedMime = http.DetectContentType(contentBytes)
	}
	ext := strings.ToLower(filepath.Ext(params.FileName))

	text, format, requiresOCR, err := extractDocumentText(contentBytes, detectedMime, ext)
	if err != nil {
		return nil, err
	}

	if len(text) > params.MaxChars {
		text = text[:params.MaxChars]
	}

	output := map[string]interface{}{
		"text":              text,
		"format":            format,
		"fileName":          params.FileName,
		"mimeType":          detectedMime,
		"fileSize":          len(contentBytes),
		"requiresOCR":       requiresOCR,
		"fileContentBase64": base64.StdEncoding.EncodeToString(contentBytes),
	}

	return output, nil
}

func extractDocumentText(content []byte, mimeType, ext string) (string, string, bool, error) {
	if isSpreadsheet(ext, mimeType) {
		text, err := extractSpreadsheetText(content)
		if err != nil {
			return "", "spreadsheet", false, fmt.Errorf("failed to parse spreadsheet: %v", err)
		}
		return text, "spreadsheet", false, nil
	}

	if strings.Contains(mimeType, "text/") || ext == ".txt" || ext == ".csv" || ext == ".json" || ext == ".xml" || ext == ".md" {
		return string(content), "text", false, nil
	}

	if strings.HasPrefix(mimeType, "image/") {
		return "", "image", true, nil
	}

	if strings.Contains(mimeType, "application/pdf") || ext == ".pdf" {
		return "", "pdf", true, nil
	}

	// Fallback for unknown binary files.
	if strings.Contains(mimeType, "application/octet-stream") {
		if looksLikeUTF8(content) {
			return string(content), "text", false, nil
		}
		return "", "binary", true, nil
	}

	return string(content), "text", false, nil
}

func isSpreadsheet(ext, mimeType string) bool {
	if ext == ".xlsx" || ext == ".xlsm" || ext == ".xls" || ext == ".csv" {
		return true
	}
	return strings.Contains(mimeType, "spreadsheet") || strings.Contains(mimeType, "excel")
}

func extractSpreadsheetText(content []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("no sheets found")
	}

	var b strings.Builder
	maxSheets := 3
	maxRowsPerSheet := 200
	for i, sheet := range sheets {
		if i >= maxSheets {
			break
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		b.WriteString("Sheet: ")
		b.WriteString(sheet)
		b.WriteString("\n")
		for r, row := range rows {
			if r >= maxRowsPerSheet {
				break
			}
			b.WriteString(strings.Join(row, ", "))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func looksLikeUTF8(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return true
}
