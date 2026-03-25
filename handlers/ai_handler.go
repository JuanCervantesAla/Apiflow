package handlers

import (
	"bytes"
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type AIGenerateFlowRequest struct {
	Description  string `json:"description" binding:"required"`
	Context      string `json:"context"`
	SystemPrompt string `json:"systemPrompt,omitempty"`
	APIKey       string `json:"apiKey,omitempty"`
	FlowID       string `json:"flowId,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
}

type AIRepairFlowRequest struct {
	Flow      map[string]interface{} `json:"flow" binding:"required"`
	Issues    string                 `json:"issues"`
	APIKey    string                 `json:"apiKey,omitempty"`
	FlowID    string                 `json:"flowId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
}

type AIFixFlowRequest struct {
	Flow                 map[string]interface{} `json:"flow" binding:"required"`
	Goal                 string                 `json:"goal"`
	Issues               string                 `json:"issues"`
	AdjustParametersOnly bool                   `json:"adjustParametersOnly"`
	StrictMode           *bool                  `json:"strictMode,omitempty"`
	MaxAttempts          int                    `json:"maxAttempts,omitempty"`
	APIKey               string                 `json:"apiKey,omitempty"`
	FlowID               string                 `json:"flowId,omitempty"`
	SessionID            string                 `json:"sessionId,omitempty"`
}

// Groq API structures (OpenAI-compatible)
type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequest struct {
	Model       string        `json:"model"`
	Messages    []GroqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type GroqResponse struct {
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

type AIGeneratedFlow struct {
	FlowName        string `json:"flowName"`
	FlowDescription string `json:"flowDescription"`
	Nodes           []any  `json:"nodes"`
	Edges           []any  `json:"edges"`
}

type AIGenerateFlowResponse struct {
	Success     bool             `json:"success"`
	Flow        *AIGeneratedFlow `json:"flow,omitempty"`
	Error       string           `json:"error,omitempty"`
	RawResponse string           `json:"rawResponse,omitempty"`
}

type AIHandler struct {
	DB               *gorm.DB
	analyticsService *services.AnalyticsService
}

func NewAIHandler(db *gorm.DB) *AIHandler {
	return &AIHandler{
		DB:               db,
		analyticsService: services.NewAnalyticsService(db),
	}
}

func (h *AIHandler) getAPIKey() (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not configured in environment")
	}
	return apiKey, nil
}

func (h *AIHandler) GenerateFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIGenerateFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiKey, err := h.getAPIKey()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Groq API key not configured. Please set GROQ_API_KEY in your .env file",
		})
		return
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(req)

	// Groq request using OpenAI-compatible format
	groqReq := GroqRequest{
		Model:       "llama-3.3-70b-versatile", // Fast and powerful Groq model
		Temperature: 0.25,
		MaxTokens:   8192,
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt + "\n\nRespond ONLY with valid JSON, no additional text.",
			},
		},
	}

	jsonData, err := json.Marshal(groqReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to prepare Groq request",
		})
		return
	}

	groqURL := "https://api.groq.com/openai/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", groqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to create request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to call Groq API",
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to read Groq response",
		})
		return
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to parse Groq response: " + err.Error(),
		})
		return
	}

	if groqResp.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Groq API error: " + groqResp.Error.Message,
		})
		return
	}

	if len(groqResp.Choices) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No response from Groq API",
		})
		return
	}

	content := groqResp.Choices[0].Message.Content

	// Clean the response (remove markdown code blocks if present)
	cleanedContent := content

	// Search for JSON inside code blocks ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.LastIndex(content, "```")
		if start > 7 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	} else if strings.Contains(content, "```") {
		// Try with generic code block
		start := strings.Index(content, "```") + 3
		end := strings.LastIndex(content, "```")
		if start > 3 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	}

	// Find the first { and last } to extract the JSON
	if !strings.HasPrefix(strings.TrimSpace(cleanedContent), "{") {
		firstBrace := strings.Index(cleanedContent, "{")
		lastBrace := strings.LastIndex(cleanedContent, "}")
		if firstBrace >= 0 && lastBrace > firstBrace {
			cleanedContent = strings.TrimSpace(cleanedContent[firstBrace : lastBrace+1])
		}
	}

	var flow AIGeneratedFlow
	if err := json.Unmarshal([]byte(cleanedContent), &flow); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success:     false,
			Error:       "Failed to parse generated flow: " + err.Error(),
			RawResponse: content,
		})
		return
	}

	if flow.Nodes == nil || len(flow.Nodes) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success:     false,
			Error:       "Generated flow has no nodes",
			RawResponse: content,
		})
		return
	}

	// Apply validations and defaults to the generated nodes
	flow.Nodes = normalizeAINodes(flow.Nodes)
	flow.Edges = normalizeAIEdges(flow.Edges)
	flow.Nodes, flow.Edges = ensureIfConditionBranches(flow.Nodes, flow.Edges)
	flow.Nodes = stabilizeAIFlowReferences(flow.Nodes, flow.Edges)
	if issues := findUnresolvedNodeReferences(flow.Nodes); len(issues) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Coherence error: unresolved node references after normalization: " + strings.Join(issues, "; "),
		})
		return
	}

	processedNodes, err := processAINodes(flow.Nodes)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success:     false,
			Error:       "Validation error: " + err.Error(),
			RawResponse: content,
		})
		return
	}
	flow.Nodes = processedNodes

	fmt.Printf("✅ Flow generated and validated successfully with %d nodes\n", len(flow.Nodes))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AIGenerateFlowResponse{
		Success:     true,
		Flow:        &flow,
		RawResponse: content,
	})
}

func buildSystemPrompt() string {
	return `You are an expert workflow designer for CapyFlow.

Your output must be production-ready and executable on the first run.

GLOBAL RULES:
1. Return ONLY valid JSON.
2. First node must be one of: manual-trigger, webhook-trigger, telegram-trigger.
3. Use real node types in BOTH node.type and node.data.type. Never use "custom" as business type.
4. Use edge type "customEdge" for all edges.
5. Configure every required parameter with realistic values.
6. Add data.description for EACH node explaining purpose and expected input/output in one short sentence.
7. Keep spatial readability: start at x=100, y=100 and increment x around 260 per step.

PARAMETER CONTRACTS (must match runtime):
- set-data: values (object)
- http-request: url (string), method (GET|POST|PUT|DELETE|PATCH), optional headers/body
- log: optional label, optional message
- groq: prompt (string), optional model, temperature, maxTokens
- ai-configurator: goal (string), targetNodeType (string), optional inputContext, model, temperature, maxTokens
- if-condition: field (string), operator (==|!=|>|<|>=|<=|contains), value (string/number/bool)
- loop: arraySource (string), optional operation (forEach|map|filter), mapExpression, filterExpr
- transform-data: transformations (array of {source,target,operation,value})
- json-parser: json (string)
- delay: duration (milliseconds, max 300000)
- email: to, subject, body, from, smtpHost, smtpPort, smtpUser, smtpPassword
- telegram: message, optional chatId
- database: driver (postgres|mysql|sqlite3), connectionUrl, query
- filter: inputData (array), field, operator, value, optional mode (keep|remove)
- switch: inputValue, cases ([{value, output}]), optional defaultCase, mode (equals|contains)

INTERPOLATION:
- Use {{node-ID.output.field}} when referencing previous node output.
- Prefer explicit node references over ambiguous placeholders.
- References MUST use existing node IDs present in the same flow. Never invent aliases like node-1/node-2 unless those IDs actually exist.

JSON SHAPE:
{
	"flowName": "...",
	"flowDescription": "...",
	"nodes": [
		{
			"id": "node-1",
			"type": "manual-trigger",
			"position": {"x": 100, "y": 100},
			"data": {
				"label": "Start",
				"type": "manual-trigger",
				"description": "Starts the flow manually and emits an empty payload.",
				"parameters": {}
			}
		}
	],
	"edges": [
		{"id": "edge-1", "source": "node-1", "target": "node-2", "type": "customEdge"}
	]
}`
}

func buildUserPrompt(req AIGenerateFlowRequest) string {
	parts := []string{
		"Create a CapyFlow workflow from this request:",
		req.Description,
	}

	if strings.TrimSpace(req.Context) != "" {
		parts = append(parts, "Additional context:", req.Context)
	}

	if strings.TrimSpace(req.SystemPrompt) != "" {
		parts = append(parts, "Client-side constraints (lower priority than runtime contracts):", req.SystemPrompt)
	}

	parts = append(parts,
		"Quality checklist:",
		"- Return executable parameters, not placeholders unless credentials are required.",
		"- Make node descriptions specific and contextual.",
		"- Ensure references between nodes are coherent and existing.",
		"- Respond ONLY with JSON.",
	)

	return strings.Join(parts, "\n\n")
}

func (h *AIHandler) RepairFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIRepairFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiKey, err := h.getAPIKey()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Groq API key not configured. Please set GROQ_API_KEY in your .env file",
		})
		return
	}

	flowJSON, _ := json.MarshalIndent(req.Flow, "", "  ")

	repairPrompt := fmt.Sprintf(`You are an expert in repairing CapyFlow workflows.

CURRENT FLOW (with possible errors):
%s

REPORTED ISSUES:
%s

ANALYZE AND REPAIR:
1. Verify that all nodes have unique IDs
2. Confirm that all edges connect existing nodes
3. Ensure there is at least one trigger node (manual-trigger, webhook-trigger, or telegram-trigger)
4. Validate that node types are correct
5. Fix overlapping positions (minimum 250px horizontal separation)
6. IMPORTANT: Fix invalid or missing parameters - CONFIGURE ALL required parameters
7. Remove orphan nodes or edges

REQUIRED PARAMETERS BY NODE TYPE:
• http-request: MUST have url (string) and method ("GET"|"POST"|"PUT"|"DELETE")
• if-condition: MUST have left, operator, right
• set-data: MUST have values (object)
• groq: MUST have prompt (string with text or {{variable}})
• log: can have label and message
• delay: MUST have duration (number in seconds, max 300)
• email: MUST have to, subject, body
• telegram: MUST have message (chatId is optional if a default chat is configured)
• database: MUST have driver, connectionUrl, query
• loop: MUST have arraySource (string path to array, e.g. data or items)
• filter: MUST have field, operator, value

If a node does NOT have configured parameters, ADD them with realistic values based on its context.
If a parameter is empty "", SET a realistic placeholder value.

Parameter repair example:
BEFORE (incorrect):
{
  "id": "node-2",
  "type": "http-request",
  "data": {
    "label": "Get Data",
    "type": "http-request",
    "parameters": {}
  }
}

AFTER (corrected):
{
  "id": "node-2",
  "type": "http-request",
  "data": {
    "label": "Get Data",
    "type": "http-request",
    "parameters": {
      "url": "https://api.example.com/data",
      "method": "GET",
      "headers": {"Content-Type": "application/json"}
    }
  }
}

RESPOND WITH THE REPAIRED FLOW in this JSON format:
{
  "flowName": "flow name",
  "flowDescription": "description",
  "nodes": [...repaired nodes WITH COMPLETE PARAMETERS...],
  "edges": [...repaired edges...],
  "fixes": ["fix 1", "fix 2", ...] // List of repairs made
}

Respond ONLY with valid JSON.`, string(flowJSON), req.Issues)

	// Groq request using OpenAI-compatible format
	groqReq := GroqRequest{
		Model:       "llama-3.3-70b-versatile",
		Temperature: 0.3,
		MaxTokens:   8192,
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: "You are an expert in repairing CapyFlow workflows. Return ONLY valid JSON.",
			},
			{
				Role:    "user",
				Content: repairPrompt,
			},
		},
	}

	jsonData, err := json.Marshal(groqReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to prepare request",
		})
		return
	}

	groqURL := "https://api.groq.com/openai/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", groqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to create request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to call Groq API",
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to read response",
		})
		return
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to parse response",
		})
		return
	}

	if groqResp.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Groq API error: " + groqResp.Error.Message,
		})
		return
	}

	if len(groqResp.Choices) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No response from Groq API",
		})
		return
	}

	content := groqResp.Choices[0].Message.Content

	// Clean the response (remove markdown code blocks if present)
	cleanedContent := content

	// Search for JSON inside code blocks ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.LastIndex(content, "```")
		if start > 7 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	} else if strings.Contains(content, "```") {
		// Try with generic code block
		start := strings.Index(content, "```") + 3
		end := strings.LastIndex(content, "```")
		if start > 3 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	}

	// Find the first { and last } to extract the JSON
	if !strings.HasPrefix(strings.TrimSpace(cleanedContent), "{") {
		firstBrace := strings.Index(cleanedContent, "{")
		lastBrace := strings.LastIndex(cleanedContent, "}")
		if firstBrace >= 0 && lastBrace > firstBrace {
			cleanedContent = strings.TrimSpace(cleanedContent[firstBrace : lastBrace+1])
		}
	}

	// Detect truncated response (missing JSON closing)
	openBraces := strings.Count(cleanedContent, "{")
	closeBraces := strings.Count(cleanedContent, "}")
	isTruncated := openBraces != closeBraces || !strings.HasSuffix(strings.TrimSpace(cleanedContent), "}")

	var repairedFlow struct {
		FlowName        string   `json:"flowName"`
		FlowDescription string   `json:"flowDescription"`
		Nodes           []any    `json:"nodes"`
		Edges           []any    `json:"edges"`
		Fixes           []string `json:"fixes"`
	}

	if err := json.Unmarshal([]byte(cleanedContent), &repairedFlow); err != nil {
		// Log for debugging
		fmt.Printf("❌ Error parsing repaired flow JSON: %v\n", err)
		fmt.Printf("📄 Original content length: %d\n", len(content))
		fmt.Printf("📄 Cleaned content length: %d\n", len(cleanedContent))
		fmt.Printf("🔢 Braces: open=%d, close=%d, truncated=%v\n", openBraces, closeBraces, isTruncated)
		fmt.Printf("📄 Cleaned content (last 200 chars): ...%s\n", cleanedContent[max(0, len(cleanedContent)-200):])

		hint := "AI did not return valid JSON. Try again."
		if isTruncated {
			hint = "AI response was truncated. The flow is too complex. Try simplifying it or repair it manually."
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     false,
			"error":       "Failed to parse repaired flow: " + err.Error(),
			"rawResponse": content,
			"cleaned":     cleanedContent,
			"hint":        hint,
			"truncated":   isTruncated,
		})
		return
	}

	// Validate that it has content
	if len(repairedFlow.Nodes) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Repaired flow contains no nodes",
			"fixes":   repairedFlow.Fixes,
		})
		return
	}

	// Apply validations and defaults to the repaired nodes
	repairedFlow.Nodes = normalizeAINodes(repairedFlow.Nodes)
	repairedFlow.Edges = normalizeAIEdges(repairedFlow.Edges)
	repairedFlow.Nodes, repairedFlow.Edges = ensureIfConditionBranches(repairedFlow.Nodes, repairedFlow.Edges)
	repairedFlow.Nodes = stabilizeAIFlowReferences(repairedFlow.Nodes, repairedFlow.Edges)
	if issues := findUnresolvedNodeReferences(repairedFlow.Nodes); len(issues) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Coherence error: unresolved node references after normalization: " + strings.Join(issues, "; "),
			"fixes":   repairedFlow.Fixes,
		})
		return
	}

	processedNodes, err := processAINodes(repairedFlow.Nodes)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Validation error: " + err.Error(),
			"fixes":   repairedFlow.Fixes,
		})
		return
	}
	repairedFlow.Nodes = processedNodes

	fmt.Printf("✅ Flow repaired and validated successfully with %d nodes, %d edges, and %d fixes\n",
		len(repairedFlow.Nodes), len(repairedFlow.Edges), len(repairedFlow.Fixes))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"flow": map[string]interface{}{
			"flowName":        repairedFlow.FlowName,
			"flowDescription": repairedFlow.FlowDescription,
			"nodes":           repairedFlow.Nodes,
			"edges":           repairedFlow.Edges,
		},
		"fixes":       repairedFlow.Fixes,
		"rawResponse": content,
	})
}

func (h *AIHandler) FixFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIFixFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiKey, err := h.getAPIKey()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Groq API key not configured. Please set GROQ_API_KEY in your .env file",
		})
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		req.Goal = "Optimize and complete node parameters so the flow is executable and easier to understand."
	}

	if !req.AdjustParametersOnly {
		req.AdjustParametersOnly = true
	}

	strictMode := true
	if req.StrictMode != nil {
		strictMode = *req.StrictMode
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 3 {
		maxAttempts = 3
	}

	flowJSON, _ := json.MarshalIndent(req.Flow, "", "  ")
	flowContext := summarizeFlowContext(req.Flow)

	basePrompt := fmt.Sprintf(`You are an expert CapyFlow optimization assistant.

PRIMARY GOAL:
%s

USER-REPORTED ISSUES (optional):
%s

CURRENT FLOW (full context):
%s

FLOW SUMMARY:
%s

RULES:
1. Return ONLY valid JSON.
2. Preserve semantic behavior unless the goal explicitly requests behavior changes.
3. Keep node ids and edge ids stable whenever possible.
4. Keep node positions stable unless overlap is severe.
5. Prioritize PARAMETER optimization and completion.
6. Ensure all required parameters are configured with practical values.
7. Prefer explicit interpolation references ({{node-id.output.field}}).
8. Keep descriptions concise and useful for non-technical users.
9. If unsure about credentials, leave placeholders only for secret values.
10. Never use synthetic references like node-1/node-2 unless they are actual IDs in the flow.
11. Keep set-data values runtime-compatible and deterministic.

STRICT MODE:
- strictMode = %v
- adjustParametersOnly = %v
- If true: do not add/remove nodes unless absolutely necessary for validity.

RESPONSE FORMAT:
{
  "flowName": "...",
  "flowDescription": "...",
  "nodes": [...],
  "edges": [...],
  "fixes": ["what changed", "..."]
}

	Respond ONLY with JSON.`, req.Goal, req.Issues, string(flowJSON), flowContext, strictMode, req.AdjustParametersOnly)

	type aiFixedFlow struct {
		FlowName        string   `json:"flowName"`
		FlowDescription string   `json:"flowDescription"`
		Nodes           []any    `json:"nodes"`
		Edges           []any    `json:"edges"`
		Fixes           []string `json:"fixes"`
	}

	var (
		lastErrMsg       string
		lastHint         string
		lastRawResponse  string
		lastCleaned      string
		lastTruncated    bool
		lastFixes        []string
		feedbackForRetry string
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		prompt := basePrompt
		if strings.TrimSpace(feedbackForRetry) != "" {
			prompt = prompt + "\n\nPREVIOUS ATTEMPT FEEDBACK:\n" + feedbackForRetry + "\n\nUse this feedback to correct only problematic parts."
		}

		groqReq := GroqRequest{
			Model:       "llama-3.3-70b-versatile",
			Temperature: 0.2,
			MaxTokens:   8192,
			Messages: []GroqMessage{
				{Role: "system", Content: "You optimize CapyFlow workflows. Output ONLY valid JSON."},
				{Role: "user", Content: prompt},
			},
		}

		jsonData, err := json.Marshal(groqReq)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Failed to prepare request"})
			return
		}

		httpReq, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Failed to call Groq API"})
			return
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Failed to read response"})
			return
		}

		var groqResp GroqResponse
		if err := json.Unmarshal(body, &groqResp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Failed to parse response"})
			return
		}
		if groqResp.Error != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "Groq API error: " + groqResp.Error.Message})
			return
		}
		if len(groqResp.Choices) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AIGenerateFlowResponse{Success: false, Error: "No response from Groq API"})
			return
		}

		content := groqResp.Choices[0].Message.Content
		cleanedContent := content
		if strings.Contains(content, "```json") {
			start := strings.Index(content, "```json") + 7
			end := strings.LastIndex(content, "```")
			if start > 7 && end > start {
				cleanedContent = strings.TrimSpace(content[start:end])
			}
		} else if strings.Contains(content, "```") {
			start := strings.Index(content, "```") + 3
			end := strings.LastIndex(content, "```")
			if start > 3 && end > start {
				cleanedContent = strings.TrimSpace(content[start:end])
			}
		}
		if !strings.HasPrefix(strings.TrimSpace(cleanedContent), "{") {
			firstBrace := strings.Index(cleanedContent, "{")
			lastBrace := strings.LastIndex(cleanedContent, "}")
			if firstBrace >= 0 && lastBrace > firstBrace {
				cleanedContent = strings.TrimSpace(cleanedContent[firstBrace : lastBrace+1])
			}
		}

		openBraces := strings.Count(cleanedContent, "{")
		closeBraces := strings.Count(cleanedContent, "}")
		isTruncated := openBraces != closeBraces || !strings.HasSuffix(strings.TrimSpace(cleanedContent), "}")

		lastRawResponse = content
		lastCleaned = cleanedContent
		lastTruncated = isTruncated

		var fixedFlow aiFixedFlow
		if err := json.Unmarshal([]byte(cleanedContent), &fixedFlow); err != nil {
			lastErrMsg = "Failed to parse fixed flow: " + err.Error()
			if isTruncated {
				lastHint = "AI response was truncated. Please try with a shorter goal."
			} else {
				lastHint = "AI returned invalid JSON."
			}
			feedbackForRetry = lastErrMsg
			continue
		}

		lastFixes = fixedFlow.Fixes
		if len(fixedFlow.Nodes) == 0 {
			lastErrMsg = "Fixed flow contains no nodes"
			lastHint = "AI removed all nodes. Keep original structure and only adjust parameters."
			feedbackForRetry = lastErrMsg
			continue
		}

		fixedFlow.Nodes = normalizeAINodes(fixedFlow.Nodes)
		fixedFlow.Edges = normalizeAIEdges(fixedFlow.Edges)
		fixedFlow.Nodes, fixedFlow.Edges = ensureIfConditionBranches(fixedFlow.Nodes, fixedFlow.Edges)
		fixedFlow.Nodes = stabilizeAIFlowReferences(fixedFlow.Nodes, fixedFlow.Edges)

		coherence := buildFlowCoherenceReport(fixedFlow.Nodes)
		if strictMode && len(coherence["issues"].([]string)) > 0 {
			issues := coherence["issues"].([]string)
			lastErrMsg = "Coherence validation failed"
			lastHint = "AI produced inconsistent node references or runtime-incompatible values."
			feedbackForRetry = "Coherence issues: " + strings.Join(issues, " | ")
			continue
		}

		processedNodes, err := processAINodes(fixedFlow.Nodes)
		if err != nil {
			lastErrMsg = "Validation error: " + err.Error()
			lastHint = "Parameters are still incompatible with runtime contracts."
			feedbackForRetry = lastErrMsg
			continue
		}
		fixedFlow.Nodes = processedNodes

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"flow": map[string]interface{}{
				"flowName":        fixedFlow.FlowName,
				"flowDescription": fixedFlow.FlowDescription,
				"nodes":           fixedFlow.Nodes,
				"edges":           fixedFlow.Edges,
			},
			"fixes":       fixedFlow.Fixes,
			"rawResponse": content,
			"strictMode":  strictMode,
			"attempts":    attempt,
			"coherence":   coherence,
		})
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     false,
		"error":       lastErrMsg,
		"hint":        lastHint,
		"fixes":       lastFixes,
		"rawResponse": lastRawResponse,
		"cleaned":     lastCleaned,
		"truncated":   lastTruncated,
		"strictMode":  strictMode,
		"attempts":    maxAttempts,
	})
	return
}

func summarizeFlowContext(flow map[string]interface{}) string {
	nodesAny, _ := flow["nodes"].([]interface{})
	edgesAny, _ := flow["edges"].([]interface{})

	typeCounts := map[string]int{}
	missingParams := []string{}

	for _, item := range nodesAny {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)
		data, _ := node["data"].(map[string]interface{})
		if data != nil {
			if dt, ok := data["type"].(string); ok && strings.TrimSpace(dt) != "" {
				nodeType = dt
			}
		}
		typeCounts[nodeType]++

		var params map[string]interface{}
		if data != nil {
			params, _ = data["parameters"].(map[string]interface{})
		}
		if params == nil {
			params, _ = node["parameters"].(map[string]interface{})
		}

		if params == nil || len(params) == 0 {
			missingParams = append(missingParams, fmt.Sprintf("%s(%s)", nodeID, nodeType))
		}
	}

	parts := []string{
		fmt.Sprintf("nodes=%d", len(nodesAny)),
		fmt.Sprintf("edges=%d", len(edgesAny)),
		fmt.Sprintf("typeCounts=%v", typeCounts),
	}

	if len(missingParams) > 0 {
		limit := len(missingParams)
		if limit > 15 {
			limit = 15
		}
		parts = append(parts, "nodesWithMissingParams="+strings.Join(missingParams[:limit], ", "))
	}

	return strings.Join(parts, " | ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeAINodes(nodes []any) []any {
	result := make([]any, 0, len(nodes))

	for i, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}

		if id, _ := nodeMap["id"].(string); strings.TrimSpace(id) == "" {
			nodeMap["id"] = fmt.Sprintf("node-%d", i+1)
		}

		data, _ := nodeMap["data"].(map[string]interface{})
		if data == nil {
			data = map[string]interface{}{}
			nodeMap["data"] = data
		}

		nodeType, _ := nodeMap["type"].(string)
		if dataType, ok := data["type"].(string); ok && strings.TrimSpace(dataType) != "" {
			nodeType = dataType
		}
		nodeType = strings.TrimSpace(nodeType)
		if nodeType == "" {
			nodeType = "log"
		}
		nodeMap["type"] = nodeType
		data["type"] = nodeType

		params, _ := data["parameters"].(map[string]interface{})
		if params == nil {
			params = map[string]interface{}{}
		}
		normalizeNodeParameters(nodeType, params)
		data["parameters"] = params

		if label, _ := data["label"].(string); strings.TrimSpace(label) == "" {
			data["label"] = defaultNodeLabel(nodeType)
		}

		if description, _ := data["description"].(string); strings.TrimSpace(description) == "" {
			data["description"] = defaultNodeDescription(nodeType, params)
		}

		result = append(result, nodeMap)
	}

	return result
}

func normalizeAIEdges(edges []any) []any {
	result := make([]any, 0, len(edges))
	for i, edgeAny := range edges {
		edge, ok := edgeAny.(map[string]interface{})
		if !ok {
			result = append(result, edgeAny)
			continue
		}

		if id, _ := edge["id"].(string); strings.TrimSpace(id) == "" {
			edge["id"] = fmt.Sprintf("edge-%d", i+1)
		}
		edge["type"] = "customEdge"
		result = append(result, edge)
	}
	return result
}

func stabilizeAIFlowReferences(nodes []any, edges []any) []any {
	_ = edges // Reserved for future graph-aware remapping.
	if len(nodes) == 0 {
		return nodes
	}

	aliasMap := buildLegacyAliasMap(nodes)
	if len(aliasMap) == 0 {
		return nodes
	}

	for _, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		nodeType, _ := nodeMap["type"].(string)
		data, _ := nodeMap["data"].(map[string]interface{})
		if data != nil {
			if dt, ok := data["type"].(string); ok && strings.TrimSpace(dt) != "" {
				nodeType = dt
			}
		}

		var params map[string]interface{}
		if data != nil {
			params, _ = data["parameters"].(map[string]interface{})
		}
		if params == nil {
			params, _ = nodeMap["parameters"].(map[string]interface{})
		}
		if params == nil {
			continue
		}

		replaced := replaceLegacyAliases(params, aliasMap)
		if replacedMap, ok := replaced.(map[string]interface{}); ok {
			params = replacedMap
		}

		if nodeType == "if-condition" {
			normalizeIfConditionFromExpression(params)
		}

		if data != nil {
			data["parameters"] = params
			nodeMap["data"] = data
		} else {
			nodeMap["parameters"] = params
		}
	}

	return nodes
}

func ensureIfConditionBranches(nodes []any, edges []any) ([]any, []any) {
	if len(nodes) == 0 {
		return nodes, edges
	}

	nodeByID := map[string]map[string]interface{}{}
	nodeIDExists := map[string]bool{}
	ifNodeIDs := []string{}

	for _, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		nodeID, _ := nodeMap["id"].(string)
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}

		nodeByID[nodeID] = nodeMap
		nodeIDExists[nodeID] = true

		nodeType, _ := nodeMap["type"].(string)
		if data, ok := nodeMap["data"].(map[string]interface{}); ok {
			if dt, ok := data["type"].(string); ok && strings.TrimSpace(dt) != "" {
				nodeType = dt
			}
		}

		if strings.TrimSpace(nodeType) == "if-condition" {
			ifNodeIDs = append(ifNodeIDs, nodeID)
		}
	}

	if len(ifNodeIDs) == 0 {
		return nodes, edges
	}

	outgoingIdx := map[string][]int{}
	for i, edgeAny := range edges {
		edgeMap, ok := edgeAny.(map[string]interface{})
		if !ok {
			continue
		}
		source, _ := edgeMap["source"].(string)
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, isIf := nodeByID[source]; isIf {
			outgoingIdx[source] = append(outgoingIdx[source], i)
		}
	}

	for _, ifNodeID := range ifNodeIDs {
		edgeIndexes := outgoingIdx[ifNodeID]
		hasTrue := false
		hasFalse := false
		unassigned := make([]int, 0, len(edgeIndexes))

		for _, idx := range edgeIndexes {
			edgeMap, ok := edges[idx].(map[string]interface{})
			if !ok {
				continue
			}
			branch := readBranchFromEdge(edgeMap)
			switch branch {
			case "true":
				hasTrue = true
			case "false":
				hasFalse = true
			default:
				unassigned = append(unassigned, idx)
			}
		}

		for _, idx := range unassigned {
			edgeMap, ok := edges[idx].(map[string]interface{})
			if !ok {
				continue
			}
			if !hasTrue {
				edgeMap["sourceHandle"] = "true"
				edgeMap["label"] = "true"
				hasTrue = true
				continue
			}
			if !hasFalse {
				edgeMap["sourceHandle"] = "false"
				edgeMap["label"] = "false"
				hasFalse = true
				continue
			}
		}

		for _, missingBranch := range []string{"true", "false"} {
			if (missingBranch == "true" && hasTrue) || (missingBranch == "false" && hasFalse) {
				continue
			}

			ifNode := nodeByID[ifNodeID]
			x, y := readNodePosition(ifNode)
			offsetY := 120.0
			if missingBranch == "false" {
				offsetY = -120.0
			}

			stopID := fmt.Sprintf("%s-auto-stop-%s", ifNodeID, missingBranch)
			if nodeIDExists[stopID] {
				stopID = fmt.Sprintf("%s-auto-stop-%s-%d", ifNodeID, missingBranch, len(nodes)+1)
			}
			nodeIDExists[stopID] = true

			stopNode := map[string]interface{}{
				"id":       stopID,
				"type":     "stop",
				"label":    "Auto Stop " + strings.ToUpper(missingBranch),
				"category": "control",
				"position": map[string]interface{}{
					"x": x + 280,
					"y": y + offsetY,
				},
				"data": map[string]interface{}{
					"type":        "stop",
					"label":       "Auto Stop " + strings.ToUpper(missingBranch),
					"category":    "control",
					"description": "Auto-generated fallback branch for if-condition.",
					"parameters": map[string]interface{}{
						"reason": "Missing " + missingBranch + " branch was auto-fixed",
					},
				},
			}

			edgeID := fmt.Sprintf("edge-%s-auto-%s", ifNodeID, missingBranch)
			autoEdge := map[string]interface{}{
				"id":           edgeID,
				"type":         "customEdge",
				"source":       ifNodeID,
				"target":       stopID,
				"sourceHandle": missingBranch,
				"label":        missingBranch,
			}

			nodes = append(nodes, stopNode)
			edges = append(edges, autoEdge)

			if missingBranch == "true" {
				hasTrue = true
			} else {
				hasFalse = true
			}
		}
	}

	return nodes, edges
}

func readBranchFromEdge(edge map[string]interface{}) string {
	if branch, _ := edge["sourceHandle"].(string); strings.TrimSpace(branch) != "" {
		return strings.ToLower(strings.TrimSpace(branch))
	}
	if label, _ := edge["label"].(string); strings.TrimSpace(label) != "" {
		return strings.ToLower(strings.TrimSpace(label))
	}
	return ""
}

func buildLegacyAliasMap(nodes []any) map[string]string {
	type nodePoint struct {
		id string
		x  float64
		y  float64
	}

	points := make([]nodePoint, 0, len(nodes))
	for i, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := nodeMap["id"].(string)
		if strings.TrimSpace(id) == "" {
			id = fmt.Sprintf("node-%d", i+1)
		}
		x, y := readNodePosition(nodeMap)
		points = append(points, nodePoint{id: id, x: x, y: y})
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].x == points[j].x {
			return points[i].y < points[j].y
		}
		return points[i].x < points[j].x
	})

	aliasMap := map[string]string{}
	for i, p := range points {
		alias := fmt.Sprintf("node-%d", i+1)
		if strings.TrimSpace(alias) == strings.TrimSpace(p.id) {
			continue
		}
		aliasMap[alias] = p.id
	}
	return aliasMap
}

func readNodePosition(nodeMap map[string]interface{}) (float64, float64) {
	position := nodeMap["position"]
	if posMap, ok := position.(map[string]interface{}); ok {
		return toFloat(posMap["x"]), toFloat(posMap["y"])
	}
	if posStr, ok := position.(string); ok && strings.TrimSpace(posStr) != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(posStr), &parsed); err == nil {
			return toFloat(parsed["x"]), toFloat(parsed["y"])
		}
	}
	return 0, 0
}

func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func replaceLegacyAliases(value interface{}, aliasMap map[string]string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		updated := make(map[string]interface{}, len(v))
		for k, nested := range v {
			updated[k] = replaceLegacyAliases(nested, aliasMap)
		}
		return updated
	case []interface{}:
		updated := make([]interface{}, len(v))
		for i, nested := range v {
			updated[i] = replaceLegacyAliases(nested, aliasMap)
		}
		return updated
	case string:
		result := v
		for alias, realID := range aliasMap {
			result = strings.ReplaceAll(result, "{{"+alias+".", "{{"+realID+".")
			result = strings.ReplaceAll(result, "{{"+alias+"}}", "{{"+realID+"}}")
		}
		return result
	default:
		return value
	}
}

func normalizeIfConditionFromExpression(params map[string]interface{}) {
	condition, _ := params["condition"].(string)
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return
	}

	// Example: {{node-uuid.output.priority}} == 'P1'
	re := regexp.MustCompile(`^\s*\{\{([^}]+)\}\}\s*(==|!=|>=|<=|>|<|contains)\s*['"]?(.+?)['"]?\s*$`)
	match := re.FindStringSubmatch(condition)
	if len(match) != 4 {
		return
	}

	leftRef := strings.TrimSpace(match[1])
	op := strings.TrimSpace(match[2])
	rightVal := strings.TrimSpace(match[3])

	if leftRef != "" {
		params["field"] = extractOutputFieldFromRef("{{" + leftRef + "}}")
	}
	if op != "" {
		params["operator"] = op
	}
	if rightVal != "" {
		params["value"] = rightVal
	}
}

func findUnresolvedNodeReferences(nodes []any) []string {
	idSet := map[string]bool{}
	for _, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := nodeMap["id"].(string)
		if strings.TrimSpace(id) != "" {
			idSet[id] = true
		}
	}

	placeholderRe := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	issues := []string{}
	seen := map[string]bool{}

	var walk func(interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case map[string]interface{}:
			for _, nested := range t {
				walk(nested)
			}
		case []interface{}:
			for _, nested := range t {
				walk(nested)
			}
		case string:
			matches := placeholderRe.FindAllStringSubmatch(t, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				root := strings.TrimSpace(strings.Split(m[1], ".")[0])
				if root == "" {
					continue
				}
				if idSet[root] {
					continue
				}
				// Validate only explicit node-like references.
				if strings.HasPrefix(root, "node-") || looksLikeUUID(root) {
					if !seen[root] {
						issues = append(issues, root)
						seen[root] = true
					}
				}
			}
		}
	}

	for _, item := range nodes {
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		data, _ := nodeMap["data"].(map[string]interface{})
		if data != nil {
			if params, ok := data["parameters"]; ok {
				walk(params)
			}
		}
		if params, ok := nodeMap["parameters"]; ok {
			walk(params)
		}
	}

	return issues
}

func buildFlowCoherenceReport(nodes []any) map[string]interface{} {
	issues := []string{}

	unresolvedRefs := findUnresolvedNodeReferences(nodes)
	for _, ref := range unresolvedRefs {
		issues = append(issues, fmt.Sprintf("Unresolved node reference: %s", ref))
	}

	return map[string]interface{}{
		"valid":  len(issues) == 0,
		"issues": issues,
	}
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func normalizeNodeParameters(nodeType string, params map[string]interface{}) {
	switch nodeType {
	case "set-data":
		if _, ok := params["values"]; !ok {
			if legacy, hasLegacy := params["data"]; hasLegacy {
				params["values"] = legacy
			}
		}

	case "if-condition":
		if _, ok := params["field"]; !ok {
			if left, hasLeft := params["left"]; hasLeft {
				if leftStr, ok := left.(string); ok {
					params["field"] = extractOutputFieldFromRef(leftStr)
				}
			}
		}
		if _, ok := params["value"]; !ok {
			if right, hasRight := params["right"]; hasRight {
				params["value"] = right
			}
		}
		if _, ok := params["field"]; !ok {
			params["field"] = "status"
		}
		if _, ok := params["operator"]; !ok {
			params["operator"] = "=="
		}
		if _, ok := params["value"]; !ok {
			params["value"] = "success"
		}

	case "json-parser":
		if _, ok := params["json"]; !ok {
			if input, hasInput := params["input"]; hasInput {
				params["json"] = input
			}
		}

	case "loop":
		if _, ok := params["arraySource"]; !ok {
			if items, hasItems := params["items"]; hasItems {
				if itemsStr, ok := items.(string); ok {
					params["arraySource"] = extractOutputFieldFromRef(itemsStr)
				}
			}
			if _, ok := params["arraySource"]; !ok {
				if arr, hasArray := params["array"]; hasArray {
					if arrStr, ok := arr.(string); ok {
						params["arraySource"] = extractOutputFieldFromRef(arrStr)
					}
				}
			}
		}

	case "transform-data":
		if _, ok := params["transformations"]; !ok {
			op, _ := params["operation"].(string)
			field, _ := params["field"].(string)
			if strings.TrimSpace(op) != "" && strings.TrimSpace(field) != "" {
				params["transformations"] = []map[string]interface{}{
					{
						"source":    field,
						"target":    field,
						"operation": "extract",
						"value":     "",
					},
				}
			}
		}

	case "delay":
		if duration, ok := params["duration"]; ok {
			if val, parsed := normalizeNumber(duration); parsed {
				if val > 0 && val <= 300 {
					params["duration"] = val * 1000
				} else {
					params["duration"] = val
				}
			}
		}

	case "telegram":
		if _, ok := params["chatId"]; !ok {
			if chatID, hasLegacy := params["chat_id"]; hasLegacy {
				params["chatId"] = chatID
			}
		}

	case "filter":
		if _, ok := params["inputData"]; !ok {
			if input, hasInput := params["input"]; hasInput {
				params["inputData"] = input
			} else if arr, hasArr := params["array"]; hasArr {
				params["inputData"] = arr
			}
		}
		if op, ok := params["operator"].(string); ok {
			params["operator"] = normalizeFilterOperator(op)
		}

	case "switch":
		if _, ok := params["inputValue"]; !ok {
			if val, hasValue := params["value"]; hasValue {
				params["inputValue"] = val
			}
		}
		if _, ok := params["defaultCase"]; !ok {
			if def, hasDef := params["default"]; hasDef {
				params["defaultCase"] = def
			}
		}

	case "ai-configurator":
		if _, ok := params["goal"]; !ok {
			params["goal"] = "Configure the target node with practical values for my workflow."
		}
		if _, ok := params["targetNodeType"]; !ok {
			params["targetNodeType"] = "http-request"
		}
		if _, ok := params["temperature"]; !ok {
			params["temperature"] = 0.2
		}
		if _, ok := params["maxTokens"]; !ok {
			params["maxTokens"] = 900
		}
	}
}

func normalizeFilterOperator(op string) string {
	switch op {
	case "==", "equals":
		return "equals"
	case "!=", "notEquals":
		return "notEquals"
	case ">", "greaterThan":
		return "greaterThan"
	case "<", "lessThan":
		return "lessThan"
	case ">=", "greaterOrEqual":
		return "greaterOrEqual"
	case "<=", "lessOrEqual":
		return "lessOrEqual"
	case "contains":
		return "contains"
	default:
		return op
	}
}

func extractOutputFieldFromRef(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		trimmed = strings.TrimPrefix(trimmed, "{{")
		trimmed = strings.TrimSuffix(trimmed, "}}")
	}
	parts := strings.Split(trimmed, ".")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "output" && i+1 < len(parts) {
			return strings.Join(parts[i+1:], ".")
		}
	}
	return trimmed
}

func normalizeNumber(value interface{}) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

func defaultNodeLabel(nodeType string) string {
	switch nodeType {
	case "manual-trigger":
		return "Manual Trigger"
	case "webhook-trigger":
		return "Webhook Trigger"
	case "telegram-trigger":
		return "Telegram Trigger"
	case "http-request":
		return "HTTP Request"
	case "if-condition":
		return "If Condition"
	case "set-data":
		return "Set Data"
	case "transform-data":
		return "Transform Data"
	case "json-parser":
		return "JSON Parser"
	case "groq":
		return "Groq AI"
	case "ai-configurator":
		return "AI Configurator"
	case "log":
		return "Log"
	case "delay":
		return "Delay"
	default:
		return strings.ReplaceAll(strings.Title(strings.ReplaceAll(nodeType, "-", " ")), "  ", " ")
	}
}

func defaultNodeDescription(nodeType string, params map[string]interface{}) string {
	switch nodeType {
	case "manual-trigger":
		return "Starts the workflow manually and emits an initial empty payload."
	case "webhook-trigger":
		return "Starts the workflow when an HTTP webhook request is received."
	case "telegram-trigger":
		return "Starts the workflow when a Telegram message or file is received."
	case "http-request":
		method, _ := params["method"].(string)
		url, _ := params["url"].(string)
		if method == "" {
			method = "GET"
		}
		if url == "" {
			url = "the configured endpoint"
		}
		return fmt.Sprintf("Calls %s %s and outputs response status, headers, and body.", method, url)
	case "groq":
		return "Sends a prompt to Groq AI and outputs generated text and token usage metadata."
	case "ai-configurator":
		return "Generates node parameter suggestions from a natural-language goal to reduce manual setup."
	case "if-condition":
		field, _ := params["field"].(string)
		operator, _ := params["operator"].(string)
		return fmt.Sprintf("Evaluates whether field '%s' %s the configured value and routes true/false branches.", field, operator)
	case "set-data":
		return "Creates a structured payload with predefined key-value pairs for downstream nodes."
	case "log":
		return "Writes a contextual message to execution logs for observability and debugging."
	case "delay":
		return "Pauses the workflow for the configured duration before continuing."
	default:
		return fmt.Sprintf("Executes node type '%s' with configured parameters and passes output to next nodes.", nodeType)
	}
}

// getNodeMetadata returns metadata (subtitle, icon, color, category) for a node type
func getNodeMetadata(nodeType string) (subtitle, icon, color, category string) {
	switch nodeType {
	// Triggers
	case "manual-trigger":
		return "Trigger", "IconPlayerPlay", "#3B82F6", "trigger"
	case "webhook-trigger":
		return "Trigger", "IconWebhook", "#06B6D4", "trigger"
	case "telegram-trigger":
		return "Trigger", "IconBrandTelegram", "#0088cc", "trigger"

	// Logic
	case "if-condition", "if-condition-v2":
		return "Logic", "IconGitBranch", "#8B5CF6", "logic"
	case "loop":
		return "Logic", "IconRepeat", "#A855F7", "logic"
	case "switch":
		return "Logic", "IconSwitch", "#9333EA", "logic"

	// Data
	case "set-data":
		return "Data", "IconVariable", "#10B981", "data"
	case "transform-data":
		return "Data", "IconTransform", "#14B8A6", "data"
	case "json-parser":
		return "Data", "IconBraces", "#06B6D4", "data"
	case "csv-parser":
		return "Data", "IconFileTypeCsv", "#10B981", "data"
	case "filter":
		return "Data", "IconFilter", "#059669", "data"
	case "sort":
		return "Data", "IconSortAscending", "#0D9488", "data"
	case "merge":
		return "Data", "IconGitMerge", "#14B8A6", "data"
	case "split":
		return "Data", "IconGitFork", "#06B6D4", "data"
	case "function":
		return "Data", "IconCode", "#8B5CF6", "data"

	// I/O
	case "http-request":
		return "I/O", "IconWorld", "#F59E0B", "io"
	case "log":
		return "I/O", "IconFileText", "#6B7280", "io"
	case "email":
		return "I/O", "IconMail", "#EF4444", "io"
	case "telegram":
		return "I/O", "IconBrandTelegram", "#0088cc", "io"

	// AI
	case "groq":
		return "AI", "IconBolt", "#F55036", "ai"
	case "ai-configurator":
		return "AI", "IconBrain", "#0EA5E9", "ai"

	// Control
	case "delay":
		return "Control", "IconClock", "#EC4899", "control"
	case "stop":
		return "Control", "IconPlayerStop", "#DC2626", "control"
	case "error-handler":
		return "Control", "IconAlertTriangle", "#F59E0B", "control"

	// Database
	case "database":
		return "Data", "IconDatabase", "#3B82F6", "data"

	// Regex
	case "regex-extract":
		return "Data", "IconRegex", "#8B5CF6", "data"

	default:
		return "Other", "IconBolt", "#6B7280", "other"
	}
}

// processAINodes applies validations and defaults to AI-generated nodes
func processAINodes(nodesAny []any) ([]any, error) {
	if len(nodesAny) == 0 {
		return nodesAny, nil
	}

	// Convert []any to JSON
	nodesJSON, err := json.Marshal(nodesAny)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %v", err)
	}

	// First unmarshal to an intermediate struct that accepts position as object
	type NodeInput struct {
		ID          string                 `json:"id"`
		FlowID      string                 `json:"flowId"`
		Type        string                 `json:"type"`
		Position    models.Position        `json:"position"` // Objeto con X, Y
		Label       string                 `json:"label"`
		Subtitle    string                 `json:"subtitle,omitempty"`
		Icon        string                 `json:"icon,omitempty"`
		Color       string                 `json:"color,omitempty"`
		Category    string                 `json:"category,omitempty"`
		Description string                 `json:"description,omitempty"`
		Parameters  map[string]interface{} `json:"parameters,omitempty"`
		Inputs      []interface{}          `json:"inputs,omitempty"`
		Outputs     []interface{}          `json:"outputs,omitempty"`
		Status      models.NodeStatus      `json:"status"`
		Disabled    bool                   `json:"disabled"`
		RetryOnFail bool                   `json:"retryOnFail"`
		Retries     int                    `json:"retries"`
		Version     string                 `json:"version,omitempty"`
		// React Flow format with nested data
		Data map[string]interface{} `json:"data,omitempty"`
	}

	var inputNodes []NodeInput
	if err := json.Unmarshal(nodesJSON, &inputNodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %v", err)
	}

	// Convert to []models.Node by serializing the necessary fields
	var nodes []models.Node
	for _, input := range inputNodes {
		fmt.Printf("🔍 [ProcessNodes] Processing node: id='%s', type='%s', label='%s'\n", input.ID, input.Type, input.Label)

		// If it comes in React Flow format with nested data, extract fields
		if input.Data != nil && len(input.Data) > 0 {
			if label, ok := input.Data["label"].(string); ok && input.Label == "" {
				input.Label = label
			}
			// ALWAYS prefer the real type from data.type over the React Flow type
			if nodeType, ok := input.Data["type"].(string); ok {
				input.Type = nodeType
			}
			if params, ok := input.Data["parameters"].(map[string]interface{}); ok && input.Parameters == nil {
				input.Parameters = params
			}
			if inputs, ok := input.Data["inputs"].([]interface{}); ok && input.Inputs == nil {
				input.Inputs = inputs
			}
			if outputs, ok := input.Data["outputs"].([]interface{}); ok && input.Outputs == nil {
				input.Outputs = outputs
			}
			if subtitle, ok := input.Data["subtitle"].(string); ok && input.Subtitle == "" {
				input.Subtitle = subtitle
			}
			if icon, ok := input.Data["icon"].(string); ok && input.Icon == "" {
				input.Icon = icon
			}
			if color, ok := input.Data["color"].(string); ok && input.Color == "" {
				input.Color = color
			}
			if category, ok := input.Data["category"].(string); ok && input.Category == "" {
				input.Category = category
			}
		}

		// ALWAYS auto-assign metadata based on node type
		// This ensures consistency regardless of what the AI returns
		if input.Type != "" {
			subtitle, icon, color, category := getNodeMetadata(input.Type)
			input.Subtitle = subtitle
			input.Icon = icon
			input.Color = color
			input.Category = category
			fmt.Printf("🎨 Auto-assigned metadata to node '%s' (type: %s): subtitle='%s', icon='%s', color='%s', category='%s'\n",
				input.Label, input.Type, subtitle, icon, color, category)
		}

		fmt.Printf("✅ [ProcessNodes] Before creating models.Node: ID='%s', Type='%s', Label='%s', Subtitle='%s', Icon='%s', Color='%s', Category='%s'\n",
			input.ID, input.Type, input.Label, input.Subtitle, input.Icon, input.Color, input.Category)

		// Serialize position to JSON string
		positionJSON, err := json.Marshal(input.Position)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal position for node %s: %v", input.Label, err)
		}

		// Serialize parameters
		var parametersJSON string
		if input.Parameters != nil {
			params, err := json.Marshal(input.Parameters)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal parameters for node %s: %v", input.Label, err)
			}
			parametersJSON = string(params)
		}

		// Serialize inputs
		var inputsJSON string
		if input.Inputs != nil {
			inputs, err := json.Marshal(input.Inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal inputs for node %s: %v", input.Label, err)
			}
			inputsJSON = string(inputs)
		}

		// Serialize outputs
		var outputsJSON string
		if input.Outputs != nil {
			outputs, err := json.Marshal(input.Outputs)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal outputs for node %s: %v", input.Label, err)
			}
			outputsJSON = string(outputs)
		}

		node := models.Node{
			ID:          input.ID,
			FlowID:      input.FlowID,
			Type:        input.Type,
			Position:    string(positionJSON),
			Label:       input.Label,
			Subtitle:    input.Subtitle,
			Icon:        input.Icon,
			Color:       input.Color,
			Category:    input.Category,
			Description: input.Description,
			Parameters:  parametersJSON,
			Inputs:      inputsJSON,
			Outputs:     outputsJSON,
			Status:      input.Status,
			Disabled:    input.Disabled,
			RetryOnFail: input.RetryOnFail,
			Retries:     input.Retries,
			Version:     input.Version,
		}

		fmt.Printf("📋 [ProcessNodes] models.Node created: ID='%s', Type='%s', Label='%s', Subtitle='%s', Icon='%s', Color='%s', Category='%s'\n",
			node.ID, node.Type, node.Label, node.Subtitle, node.Icon, node.Color, node.Category)

		nodes = append(nodes, node)
	}

	// Apply defaults and validate each node
	for i := range nodes {
		fmt.Printf("🔧 [ProcessNodes] Before ApplyDefaults: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)

		// Apply automatic defaults
		if err := validators.ApplyDefaults(&nodes[i]); err != nil {
			return nil, fmt.Errorf("error applying defaults to node %s: %v", nodes[i].Label, err)
		}

		fmt.Printf("🔧 [ProcessNodes] After ApplyDefaults: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)

		// Validate parameters
		if err := validators.ValidateNode(&nodes[i]); err != nil {
			return nil, fmt.Errorf("validation error for node %s: %v", nodes[i].Label, err)
		}

		fmt.Printf("✅ [ProcessNodes] After ValidateNode: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)
	}

	// Convert back to React Flow format with nested data
	var reactFlowNodes []map[string]interface{}
	for _, node := range nodes {
		// Parse position from string to object
		var position map[string]interface{}
		if err := json.Unmarshal([]byte(node.Position), &position); err != nil {
			position = map[string]interface{}{"x": 0, "y": 0}
		}

		// Parse parameters from string to object
		var parameters map[string]interface{}
		if node.Parameters != "" {
			if err := json.Unmarshal([]byte(node.Parameters), &parameters); err != nil {
				parameters = make(map[string]interface{})
			}
		} else {
			parameters = make(map[string]interface{})
		}

		// Parse inputs from string to array
		var inputs []interface{}
		if node.Inputs != "" {
			if err := json.Unmarshal([]byte(node.Inputs), &inputs); err != nil {
				inputs = []interface{}{}
			}
		} else {
			inputs = []interface{}{}
		}

		// Parse outputs from string to array
		var outputs []interface{}
		if node.Outputs != "" {
			if err := json.Unmarshal([]byte(node.Outputs), &outputs); err != nil {
				outputs = []interface{}{}
			}
		} else {
			outputs = []interface{}{}
		}

		// Create node in React Flow format
		reactFlowNode := map[string]interface{}{
			"id":       node.ID,
			"type":     "custom", // React Flow uses "custom" as the component type
			"position": position,
			"data": map[string]interface{}{
				"label":       node.Label,
				"type":        node.Type, // The REAL node type goes in data.type
				"subtitle":    node.Subtitle,
				"icon":        node.Icon,
				"color":       node.Color,
				"category":    node.Category,
				"description": node.Description,
				"parameters":  parameters,
				"inputs":      inputs,
				"outputs":     outputs,
				"status":      node.Status,
				"disabled":    node.Disabled,
				"retryOnFail": node.RetryOnFail,
				"retries":     node.Retries,
				"version":     node.Version,
			},
		}

		reactFlowNodes = append(reactFlowNodes, reactFlowNode)
	}

	// Log to verify metadata is present
	for _, rfNode := range reactFlowNodes {
		if data, ok := rfNode["data"].(map[string]interface{}); ok {
			fmt.Printf("📦 React Flow node '%s': type='%s', subtitle='%s', icon='%s', color='%s', category='%s'\n",
				data["label"], data["type"], data["subtitle"], data["icon"], data["color"], data["category"])
		}
	}

	// Convert to []any
	var result []any
	for _, node := range reactFlowNodes {
		result = append(result, node)
	}

	return result, nil
}
