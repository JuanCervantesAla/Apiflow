package nodes

import (
	"bytes"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type AIConfiguratorNode struct{}

type AIConfiguratorParams struct {
	Goal           string  `json:"goal"`
	TargetNodeType string  `json:"targetNodeType"`
	InputContext   string  `json:"inputContext"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"maxTokens"`
}

type AIConfiguratorMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIConfiguratorRequest struct {
	Model       string                  `json:"model"`
	Messages    []AIConfiguratorMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
}

type AIConfiguratorResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (n *AIConfiguratorNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params AIConfiguratorParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid ai-configurator parameters: %v", err)
	}

	params.Goal = strings.TrimSpace(interpolateString(params.Goal, prev))
	params.TargetNodeType = strings.TrimSpace(interpolateString(params.TargetNodeType, prev))
	params.InputContext = strings.TrimSpace(interpolateString(params.InputContext, prev))

	if params.Goal == "" {
		return nil, fmt.Errorf("goal is required")
	}
	if params.TargetNodeType == "" {
		return nil, fmt.Errorf("targetNodeType is required")
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not configured in environment")
	}

	if params.Model == "" {
		params.Model = "llama-3.3-70b-versatile"
	}
	if params.Temperature <= 0 {
		params.Temperature = 0.2
	}
	if params.Temperature > 2 {
		params.Temperature = 2
	}
	if params.MaxTokens <= 0 {
		params.MaxTokens = 900
	}
	if params.MaxTokens > 4096 {
		params.MaxTokens = 4096
	}

	nodeContract := getNodeContractForConfigurator(params.TargetNodeType)
	systemPrompt := "You are CapyFlow's node configuration assistant. Always respond in English and ONLY return valid JSON. Be concise and practical."
	userPrompt := fmt.Sprintf(
		"Generate a practical parameter configuration for this CapyFlow node.\n\nTarget node type: %s\nGoal: %s\nInput context: %s\n\nNode contract:\n%s\n\nOutput STRICT JSON format:\n{\n  \"suggestedParameters\": { ... },\n  \"explanation\": \"short explanation for a non-technical user\",\n  \"warnings\": [\"optional warning\"],\n  \"exampleUsage\": \"how to connect/interpolate briefly\"\n}\n\nRules:\n- Include all required parameters.\n- Use realistic values, not empty placeholders unless secret credentials are required.\n- If target node type is unknown, return best-effort generic key/value parameters in suggestedParameters.\n- Do not include markdown.",
		params.TargetNodeType,
		params.Goal,
		params.InputContext,
		nodeContract,
	)

	reqBody, err := json.Marshal(AIConfiguratorRequest{
		Model:       params.Model,
		Temperature: params.Temperature,
		MaxTokens:   params.MaxTokens,
		Messages: []AIConfiguratorMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build ai-configurator request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create ai-configurator request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Groq API: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ai-configurator response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var aiResp AIConfiguratorResponse
	if err := json.Unmarshal(bodyBytes, &aiResp); err != nil {
		return nil, fmt.Errorf("failed to parse ai-configurator response: %v", err)
	}
	if aiResp.Error != nil {
		return nil, fmt.Errorf("Groq API error: %s", aiResp.Error.Message)
	}
	if len(aiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from Groq API")
	}

	raw := strings.TrimSpace(aiResp.Choices[0].Message.Content)
	clean := extractJSONObject(raw)
	if clean == "" {
		clean = raw
	}

	var parsed struct {
		SuggestedParameters map[string]interface{} `json:"suggestedParameters"`
		Explanation         string                 `json:"explanation"`
		Warnings            []string               `json:"warnings"`
		ExampleUsage        string                 `json:"exampleUsage"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return map[string]interface{}{
			"targetNodeType": params.TargetNodeType,
			"goal":           params.Goal,
			"rawSuggestion":  raw,
			"parseError":     err.Error(),
		}, nil
	}

	if parsed.SuggestedParameters == nil {
		parsed.SuggestedParameters = map[string]interface{}{}
	}

	output := map[string]interface{}{
		"targetNodeType":      params.TargetNodeType,
		"goal":                params.Goal,
		"suggestedParameters": parsed.SuggestedParameters,
		"explanation":         strings.TrimSpace(parsed.Explanation),
		"warnings":            parsed.Warnings,
		"exampleUsage":        strings.TrimSpace(parsed.ExampleUsage),
	}
	return output, nil
}

func extractJSONObject(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return ""
}

func getNodeContractForConfigurator(nodeType string) string {
	contracts := map[string]string{
		"http-request":    "Required: url (string), method (GET|POST|PUT|DELETE|PATCH). Optional: headers (object), body (object)",
		"email":           "Required: to, subject. Common: body. Optional depending provider: from/smtpHost/smtpPort/smtpUser/smtpPassword",
		"groq":            "Required: prompt. Optional: model, temperature, maxTokens, systemPrompt",
		"database":        "Required: driver (postgres|mysql|sqlite3), connectionUrl, query",
		"if-condition":    "Required: field, operator. Optional: value",
		"set-data":        "Required: values (object)",
		"document-ingest": "Optional: inputData, fileContentBase64, fileName, mimeType, maxChars",
		"ocr-extract":     "Optional: fileUrl or fileContentBase64, mimeType, language",
		"finance-extract": "Required: text. Optional: defaultCurrency",
		"telegram":        "Required: message. Optional: chatId, parseMode",
		"filter":          "Required: inputData, operator. Optional: field, value, mode",
		"switch":          "Required: cases. Optional: inputValue, defaultCase, mode",
		"loop":            "Required: arraySource. Optional: operation, mapExpression, filterExpr",
		"delay":           "Required: duration (milliseconds)",
		"json-parser":     "Required: json",
	}
	if contract, ok := contracts[nodeType]; ok {
		return contract
	}
	return "Unknown node type. Produce a best-effort parameter object based on the goal."
}
