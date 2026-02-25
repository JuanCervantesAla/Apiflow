package nodes

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"capyflow/api/models"
)

type CSVParserNode struct{}

type CSVParserParams struct {
	InputData string `json:"inputData"` // CSV string or array to convert
	Mode      string `json:"mode"`      // "parse" or "stringify"
	Delimiter string `json:"delimiter"` // "," ";" "\t" etc
	HasHeader bool   `json:"hasHeader"` // First row is header
}

func (n *CSVParserNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params CSVParserParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Default values
	if params.Mode == "" {
		params.Mode = "parse"
	}
	if params.Delimiter == "" {
		params.Delimiter = ","
	}

	switch params.Mode {
	case "parse":
		return n.parseCSV(params)
	case "stringify":
		return n.stringifyCSV(params)
	default:
		return nil, fmt.Errorf("invalid mode: %s (must be 'parse' or 'stringify')", params.Mode)
	}
}

func (n *CSVParserNode) parseCSV(params CSVParserParams) (map[string]interface{}, error) {
	if params.InputData == "" {
		return nil, fmt.Errorf("input data is required for parsing")
	}

	reader := csv.NewReader(strings.NewReader(params.InputData))
	reader.Comma = rune(params.Delimiter[0])
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error parsing CSV: %v", err)
	}

	if len(records) == 0 {
		return map[string]interface{}{
			"data":    []interface{}{},
			"count":   0,
			"headers": []string{},
		}, nil
	}

	var headers []string
	var startRow int

	if params.HasHeader {
		headers = records[0]
		startRow = 1
	} else {
		// Generate generic headers: col1, col2, col3...
		for i := 0; i < len(records[0]); i++ {
			headers = append(headers, fmt.Sprintf("col%d", i+1))
		}
		startRow = 0
	}

	// Convert to array of objects
	var data []interface{}
	for i := startRow; i < len(records); i++ {
		row := records[i]
		obj := make(map[string]interface{})
		for j, header := range headers {
			if j < len(row) {
				obj[header] = row[j]
			} else {
				obj[header] = ""
			}
		}
		data = append(data, obj)
	}

	return map[string]interface{}{
		"data":    data,
		"count":   len(data),
		"headers": headers,
	}, nil
}

func (n *CSVParserNode) stringifyCSV(params CSVParserParams) (map[string]interface{}, error) {
	// Parse input as JSON array
	var inputArray []interface{}
	if err := json.Unmarshal([]byte(params.InputData), &inputArray); err != nil {
		return nil, fmt.Errorf("input data must be a JSON array for stringify mode: %v", err)
	}

	if len(inputArray) == 0 {
		return map[string]interface{}{
			"csv":   "",
			"count": 0,
		}, nil
	}

	// Extract headers from first object
	firstObj, ok := inputArray[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("array elements must be objects")
	}

	var headers []string
	for key := range firstObj {
		headers = append(headers, key)
	}

	// Build CSV
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	writer.Comma = rune(params.Delimiter[0])

	// Write header if requested
	if params.HasHeader {
		writer.Write(headers)
	}

	// Write rows
	for _, item := range inputArray {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		row := make([]string, len(headers))
		for i, header := range headers {
			if val, exists := obj[header]; exists {
				row[i] = fmt.Sprintf("%v", val)
			} else {
				row[i] = ""
			}
		}
		writer.Write(row)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("error writing CSV: %v", err)
	}

	csvString := builder.String()

	return map[string]interface{}{
		"csv":   csvString,
		"count": len(inputArray),
	}, nil
}
