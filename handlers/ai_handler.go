package handlers

import (
	"bytes"
	"capyflow/api/helpers"
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gorm.io/gorm"
)

type AIGenerateFlowRequest struct {
	Description string `json:"description" binding:"required"`
	Context     string `json:"context"`
	APIKey      string `json:"apiKey,omitempty"`
	FlowID      string `json:"flowId,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
}

type AIRepairFlowRequest struct {
	Flow      map[string]interface{} `json:"flow" binding:"required"`
	Issues    string                 `json:"issues"`
	APIKey    string                 `json:"apiKey,omitempty"`
	FlowID    string                 `json:"flowId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
}

// Gemini API structures
type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiGenerationConfig struct {
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
	ResponseMimeType string  `json:"responseMimeType"`
}

type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	GenerationConfig GeminiGenerationConfig `json:"generationConfig"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
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

func (h *AIHandler) getUserAPIKey(userID interface{}) (string, error) {
	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		return "", err
	}

	if user.GeminiAPIKey == "" {
		return "", nil
	}

	return helpers.DecryptString(user.GeminiAPIKey)
}

func (h *AIHandler) getAPIKey(r *http.Request, customKey string) (string, error) {
	if customKey != "" {
		return customKey, nil
	}

	userID := r.Context().Value("userId")
	if userID != nil {
		userKey, err := h.getUserAPIKey(userID)
		if err == nil && userKey != "" {
			return userKey, nil
		}
	}

	serverKey := os.Getenv("GEMINI_API_KEY")
	if serverKey != "" {
		return serverKey, nil
	}

	return "", fmt.Errorf("no API key available")
}

func (h *AIHandler) GenerateFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIGenerateFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiKey, err := h.getAPIKey(r, req.APIKey)
	if err != nil || apiKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No se encontró una API key de Gemini. Por favor, configura una en tu perfil o proporciona una temporal.",
		})
		return
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := req.Description
	if req.Context != "" {
		userPrompt = req.Context + "\n\n" + req.Description
	}

	fullPrompt := fmt.Sprintf("%s\n\nDescripción del usuario:\n%s\n\nResponde SOLO con JSON válido, sin texto adicional.", systemPrompt, userPrompt)

	geminiReq := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:      0.7,
			MaxOutputTokens:  8192, // Increased for complex flows
			ResponseMimeType: "",
		},
	}

	jsonData, err := json.Marshal(geminiReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to prepare Gemini request",
		})
		return
	}

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)
	httpReq, err := http.NewRequest("POST", geminiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to create request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to call Gemini API",
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to read Gemini response",
		})
		return
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to parse Gemini response",
		})
		return
	}

	if geminiResp.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Gemini API error: " + geminiResp.Error.Message,
		})
		return
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No response from Gemini",
		})
		return
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// Limpiar la respuesta de Gemini (remover markdown code blocks si existen)
	cleanedContent := content

	// Buscar JSON dentro de code blocks ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.LastIndex(content, "```")
		if start > 7 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	} else if strings.Contains(content, "```") {
		// Intentar con code block genérico
		start := strings.Index(content, "```") + 3
		end := strings.LastIndex(content, "```")
		if start > 3 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	}

	// Buscar el primer { y el último } para extraer el JSON
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

	// Aplicar validaciones y defaults a los nodos generados
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

	return `Eres un asistente experto en crear flujos de trabajo (workflows) para CapyFlow.

Analiza la descripción del usuario y genera un flujo de trabajo válido en formato JSON.

REGLAS:
1. Todo flujo debe empezar con manual-trigger o webhook-trigger
2. Usa tipos de nodos válidos: manual-trigger, webhook-trigger, if-condition, loop, set-data, transform-data, json-parser, http-request, log, delay
3. Posiciona los nodos espaciados (mínimo 250px horizontalmente)
4. Usa "customEdge" como tipo de edge

FORMATO DE RESPUESTA (JSON ÚNICAMENTE):
{
  "flowName": "Nombre del flujo",
  "flowDescription": "Descripción",
  "nodes": [
    {
      "id": "node-1",
      "type": "tipo-del-nodo",
      "position": { "x": 100, "y": 100 },
      "data": {
        "label": "Nombre",
        "type": "tipo-del-nodo",
        "parameters": {}
      }
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "node-1",
      "target": "node-2",
      "type": "customEdge"
    }
  ]
}

Responde SOLO con JSON válido, sin texto adicional.`
}

func (h *AIHandler) RepairFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIRepairFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiKey, err := h.getAPIKey(r, req.APIKey)
	if err != nil || apiKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No se encontró una API key de Gemini. Por favor, configura una en tu perfil.",
		})
		return
	}

	flowJSON, _ := json.MarshalIndent(req.Flow, "", "  ")

	repairPrompt := fmt.Sprintf(`Eres un experto en reparar flujos de trabajo de CapyFlow.

FLUJO ACTUAL (con posibles errores):
%s

PROBLEMAS REPORTADOS:
%s

ANALIZA Y REPARA:
1. Verifica que todos los nodos tengan IDs únicos
2. Confirma que todos los edges conecten nodos existentes
3. Asegura que haya al menos un nodo trigger (manual-trigger o webhook-trigger)
4. Valida que los tipos de nodos sean correctos: manual-trigger, webhook-trigger, if-condition, loop, set-data, transform-data, json-parser, http-request, log, delay
5. Corrige posiciones superpuestas (mínimo 250px de separación horizontal)
6. Arregla parámetros inválidos o faltantes
7. Elimina nodos o edges huérfanos

RESPONDE CON EL FLUJO REPARADO en este formato JSON:
{
  "flowName": "nombre del flujo",
  "flowDescription": "descripción",
  "nodes": [...nodos reparados...],
  "edges": [...edges reparadas...],
  "fixes": ["fix 1", "fix 2", ...] // Lista de reparaciones realizadas
}

Responde SOLO con JSON válido.`, string(flowJSON), req.Issues)

	geminiReq := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: repairPrompt},
				},
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  8192, // Increased for complex flows
			ResponseMimeType: "",
		},
	}

	jsonData, err := json.Marshal(geminiReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to prepare request",
		})
		return
	}

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)
	httpReq, err := http.NewRequest("POST", geminiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to create request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to call Gemini API",
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

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Failed to parse response",
		})
		return
	}

	if geminiResp.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Gemini API error: " + geminiResp.Error.Message,
		})
		return
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "No response from Gemini",
		})
		return
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// Limpiar la respuesta de Gemini (remover markdown code blocks si existen)
	cleanedContent := content

	// Buscar JSON dentro de code blocks ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.LastIndex(content, "```")
		if start > 7 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	} else if strings.Contains(content, "```") {
		// Intentar con code block genérico
		start := strings.Index(content, "```") + 3
		end := strings.LastIndex(content, "```")
		if start > 3 && end > start {
			cleanedContent = strings.TrimSpace(content[start:end])
		}
	}

	// Buscar el primer { y el último } para extraer el JSON
	if !strings.HasPrefix(strings.TrimSpace(cleanedContent), "{") {
		firstBrace := strings.Index(cleanedContent, "{")
		lastBrace := strings.LastIndex(cleanedContent, "}")
		if firstBrace >= 0 && lastBrace > firstBrace {
			cleanedContent = strings.TrimSpace(cleanedContent[firstBrace : lastBrace+1])
		}
	}

	// Detectar respuesta truncada (falta el cierre del JSON)
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
		// Log para debugging
		fmt.Printf("❌ Error parsing repaired flow JSON: %v\n", err)
		fmt.Printf("📄 Original content length: %d\n", len(content))
		fmt.Printf("📄 Cleaned content length: %d\n", len(cleanedContent))
		fmt.Printf("🔢 Braces: open=%d, close=%d, truncated=%v\n", openBraces, closeBraces, isTruncated)
		fmt.Printf("📄 Cleaned content (last 200 chars): ...%s\n", cleanedContent[max(0, len(cleanedContent)-200):])

		hint := "La IA no devolvió un JSON válido. Intenta de nuevo."
		if isTruncated {
			hint = "La respuesta de la IA fue truncada. El flujo es demasiado complejo. Intenta simplificarlo o repáralo manualmente."
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

	// Validar que tenga contenido
	if len(repairedFlow.Nodes) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "El flujo reparado no contiene nodos",
			"fixes":   repairedFlow.Fixes,
		})
		return
	}

	// Aplicar validaciones y defaults a los nodos reparados
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

// processAINodes aplica validaciones y defaults a los nodos generados por IA
func processAINodes(nodesAny []any) ([]any, error) {
	if len(nodesAny) == 0 {
		return nodesAny, nil
	}

	// Convertir []any a JSON y luego a []models.Node
	nodesJSON, err := json.Marshal(nodesAny)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %v", err)
	}

	var nodes []models.Node
	if err := json.Unmarshal(nodesJSON, &nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %v", err)
	}

	// Aplicar defaults y validar cada nodo
	for i := range nodes {
		// Aplicar defaults automáticos
		if err := validators.ApplyDefaults(&nodes[i]); err != nil {
			return nil, fmt.Errorf("error applying defaults to node %s: %v", nodes[i].Label, err)
		}

		// Validar parámetros
		if err := validators.ValidateNode(&nodes[i]); err != nil {
			return nil, fmt.Errorf("validation error for node %s: %v", nodes[i].Label, err)
		}
	}

	// Convertir de vuelta a []any
	processedJSON, err := json.Marshal(nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processed nodes: %v", err)
	}

	var processedNodes []any
	if err := json.Unmarshal(processedJSON, &processedNodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal processed nodes: %v", err)
	}

	return processedNodes, nil
}
