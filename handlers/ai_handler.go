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
			Error:   "API key de Groq no configurada. Por favor, configura GROQ_API_KEY en el archivo .env",
		})
		return
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := req.Description
	if req.Context != "" {
		userPrompt = req.Context + "\n\n" + req.Description
	}

	// Groq request using OpenAI-compatible format
	groqReq := GroqRequest{
		Model:       "llama-3.3-70b-versatile", // Fast and powerful Groq model
		Temperature: 0.7,
		MaxTokens:   8192,
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt + "\n\nResponde SOLO con JSON válido, sin texto adicional.",
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

	// Limpiar la respuesta (remover markdown code blocks si existen)
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

Analiza la descripción del usuario y genera un flujo de trabajo válido en formato JSON con nodos COMPLETAMENTE CONFIGURADOS.

REGLAS:
1. Todo flujo debe empezar con manual-trigger o webhook-trigger
2. Posiciona los nodos espaciados (mínimo 250px horizontalmente)
3. Usa "customEdge" como tipo de edge
4. IMPORTANTE: Configura TODOS los parámetros requeridos de cada nodo con valores realistas
5. CRITICAL: En cada nodo, usa el tipo REAL (http-request, gpt, log, etc.) tanto en "type" como en "data.type" - NUNCA uses "custom"

TIPOS DE NODOS DISPONIBLES:

--- TRIGGERS ---
• manual-trigger: Inicia el flujo manualmente
  Parámetros: ninguno

• webhook-trigger: Inicia el flujo vía webhook HTTP
  Parámetros: 
    - method: "POST" | "GET" (default: "POST")
    - path: "/webhook/path" (opcional)

--- LOGIC ---
• if-condition: Bifurcación condicional
  Parámetros REQUERIDOS:
    - left: "{{node-X.output.field}}" (valor izquierdo)
    - operator: "==" | "!=" | ">" | "<" | ">=" | "<=" | "contains"
    - right: "valor" (valor derecho)

• loop: Itera sobre un array
  Parámetros REQUERIDOS:
    - items: "{{node-X.output.data}}" (array)
    - max_iterations: 100 (opcional)

--- DATA ---
• set-data: Define datos estáticos
  Parámetros REQUERIDOS:
    - data: { "key": "value", "number": 42 }

• transform-data: Transforma datos
  Parámetros REQUERIDOS:
    - operation: "map" | "filter"
    - field: "fieldName"

• json-parser: Parsea JSON
  Parámetros REQUERIDOS:
    - input: "{{node-X.output}}"

--- I/O ---
• http-request: Hace petición HTTP
  Parámetros REQUERIDOS:
    - url: "https://api.example.com/endpoint"
    - method: "GET" | "POST" | "PUT" | "DELETE"
  Parámetros opcionales:
    - headers: {"Content-Type": "application/json"}
    - body: "{\"key\": \"value\"}"

• log: Registra información en consola
  Parámetros:
    - label: "Log name"
    - message: "Mensaje: {{node-X.output}}"

• delay: Pausa la ejecución
  Parámetros REQUERIDOS:
    - duration: 5 (segundos, máx 300)

--- AI ---
• gpt: Usa GPT de OpenAI
  Parámetros REQUERIDOS:
    - prompt: "Resume este texto: {{node-X.output}}"
  Parámetros opcionales:
    - model: "gpt-4" (default)
    - temperature: 0.7
    - max_tokens: 1000

• claude: Usa Claude de Anthropic
  Parámetros REQUERIDOS:
    - prompt: "Analiza: {{node-X.output}}"

• groq: Usa Groq AI (LLaMA, Mixtral, Gemma)
  Parámetros REQUERIDOS:
    - prompt: "Genera ideas sobre: {{node-X.output}}"

--- COMMUNICATION ---
• email: Envía email
  Parámetros REQUERIDOS:
    - to: "user@example.com"
    - subject: "Asunto del email"
    - body: "Contenido del mensaje"

• telegram: Envía mensaje a Telegram
  Parámetros REQUERIDOS:
    - message: "Texto del mensaje"

--- DATABASE ---
• database: Ejecuta query SQL
  Parámetros REQUERIDOS:
    - driver: "postgres" | "mysql" | "sqlite3"
    - connectionUrl: "postgres://user:pass@host:5432/db"
    - query: "SELECT * FROM users WHERE id = {{userId}}"

--- FILTER ---
• filter: Filtra arrays
  Parámetros REQUERIDOS:
    - field: "status"
    - operator: "==" | "!=" | ">" | "<" | "contains"
    - value: "active"

--- ADVANCED DATA ---
• split: Divide un array en chunks
  Parámetros REQUERIDOS:
    - array: "{{node-X.output.items}}" (array a dividir)
    - chunkSize: 10 (tamaño de cada chunk, número > 0)

• merge: Combina múltiples arrays
  Parámetros REQUERIDOS:
    - arrays: ["{{node-X.output}}", "{{node-Y.output}}"] (arrays a combinar)

• function: Ejecuta código JavaScript personalizado
  Parámetros REQUERIDOS:
    - code: "return input * 2;" (código JavaScript, usar 'input' como variable)

• sort: Ordena un array por campo
  Parámetros REQUERIDOS:
    - array: "{{node-X.output.items}}" (array a ordenar)
    - field: "name" (campo para ordenar)
  Parámetros opcionales:
    - order: "asc" | "desc" (default: "asc")

• csv-parser: Parsea string CSV a array
  Parámetros REQUERIDOS:
    - input: "{{node-X.output}}" (string CSV)
  Parámetros opcionales:
    - delimiter: "," (default: ",")
    - hasHeader: true (default: true)

• regex-extract: Extrae texto usando regex
  Parámetros REQUERIDOS:
    - input: "{{node-X.output.text}}" (string de entrada)
    - pattern: "[0-9]+" (expresión regular)
  Parámetros opcionales:
    - group: 0 (grupo de captura, 0 = match completo)

--- CONTROL FLOW ---
• switch: Múltiples condiciones (como switch/case)
  Parámetros REQUERIDOS:
    - value: "{{node-X.output.status}}" (valor a evaluar)
    - cases: [{"value": "active", "output": "A"}, {"value": "inactive", "output": "B"}]
  Parámetros opcionales:
    - default: "default output" (si no hay match)

• error-handler: Maneja errores con retry
  Parámetros opcionales:
    - retry: true (reintentar en error, default: false)
    - maxRetries: 3 (máximo reintentos, 1-10)
    - fallbackValue: "default" (valor por defecto si falla)

• stop: Detiene la ejecución del flujo
  Parámetros opcionales:
    - message: "Flujo terminado" (mensaje de parada)

INTERPOLACIÓN DE VARIABLES:
- Usa {{node-ID.output}} para referenciar el output completo de un nodo
- Usa {{node-ID.output.field}} para campos específicos
- Ejemplo: "{{node-2.output.temperature}}"

EJEMPLOS COMPLETOS:

Ejemplo 1 - Consulta API del clima:
{
  "flowName": "Weather API Query",
  "flowDescription": "Consulta el clima de una ciudad usando API",
  "nodes": [
    {
      "id": "node-1",
      "type": "manual-trigger",
      "position": {"x": 100, "y": 100},
      "data": {
        "label": "Start Weather Query",
        "type": "manual-trigger",
        "parameters": {}
      }
    },
    {
      "id": "node-2",
      "type": "http-request",
      "position": {"x": 360, "y": 100},
      "data": {
        "label": "Get Weather Data",
        "type": "http-request",
        "parameters": {
          "url": "https://api.openweathermap.org/data/2.5/weather?q=London&appid=YOUR_KEY",
          "method": "GET",
          "headers": {"Accept": "application/json"}
        }
      }
    },
    {
      "id": "node-3",
      "type": "log",
      "position": {"x": 620, "y": 100},
      "data": {
        "label": "Display Temperature",
        "type": "log",
        "parameters": {
          "label": "Weather Result",
          "message": "Temperature: {{node-2.output.main.temp}}°C"
        }
      }
    }
  ],
  "edges": [
    {"id": "e1", "source": "node-1", "target": "node-2", "type": "customEdge"},
    {"id": "e2", "source": "node-2", "target": "node-3", "type": "customEdge"}
  ]
}

Ejemplo 2 - Procesamiento con IA:
{
  "flowName": "AI Text Analysis",
  "flowDescription": "Analiza sentimiento de texto con GPT",
  "nodes": [
    {
      "id": "node-1",
      "type": "manual-trigger",
      "position": {"x": 100, "y": 100},
      "data": {
        "label": "Start Analysis",
        "type": "manual-trigger",
        "parameters": {}
      }
    },
    {
      "id": "node-2",
      "type": "set-data",
      "position": {"x": 360, "y": 100},
      "data": {
        "label": "Customer Feedback",
        "type": "set-data",
        "parameters": {
          "data": {
            "text": "The product is amazing! Best purchase ever.",
            "source": "Review #1234"
          }
        }
      }
    },
    {
      "id": "node-3",
      "type": "gpt",
      "position": {"x": 620, "y": 100},
      "data": {
        "label": "Analyze Sentiment",
        "type": "gpt",
        "parameters": {
          "prompt": "Analyze the sentiment (positive/negative/neutral) and extract key points from: {{node-2.output.text}}",
          "model": "gpt-4",
          "temperature": 0.3,
          "max_tokens": 500
        }
      }
    },
    {
      "id": "node-4",
      "type": "log",
      "position": {"x": 880, "y": 100},
      "data": {
        "label": "Show Results",
        "type": "log",
        "parameters": {
          "label": "Sentiment Analysis",
          "message": "Result: {{node-3.output}}"
        }
      }
    }
  ],
  "edges": [
    {"id": "e1", "source": "node-1", "target": "node-2", "type": "customEdge"},
    {"id": "e2", "source": "node-2", "target": "node-3", "type": "customEdge"},
    {"id": "e3", "source": "node-3", "target": "node-4", "type": "customEdge"}
  ]
}

FORMATO DE RESPUESTA (JSON ÚNICAMENTE):
IMPORTANTE: Usa el tipo REAL del nodo (manual-trigger, http-request, gpt, log, etc.)
NO uses "custom" - el sistema agregará automáticamente iconos y colores.

{
  "flowName": "Nombre descriptivo",
  "flowDescription": "Breve descripción",
  "nodes": [
    {
      "id": "node-X",
      "type": "TIPO-REAL-DEL-NODO",  // Ej: "http-request", "gpt", "log"
      "position": {"x": 100, "y": 100},
      "data": {
        "label": "Nombre descriptivo",
        "type": "TIPO-REAL-DEL-NODO",  // MISMO tipo que arriba
        "parameters": { ...parámetros completos... }
      }
    }
  ],
  "edges": [...]
}

Responde SOLO con JSON válido, sin explicaciones ni texto adicional. CONFIGURA TODOS LOS PARÁMETROS REQUERIDOS.`
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
			Error:   "API key de Groq no configurada. Por favor, configura GROQ_API_KEY en el archivo .env",
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
4. Valida que los tipos de nodos sean correctos
5. Corrige posiciones superpuestas (mínimo 250px de separación horizontal)
6. IMPORTANTE: Arregla parámetros inválidos o faltantes - CONFIGURA TODOS los parámetros requeridos
7. Elimina nodos o edges huérfanos

PARÁMETROS REQUERIDOS POR TIPO DE NODO:
• http-request: DEBE tener url (string) y method ("GET"|"POST"|"PUT"|"DELETE")
• if-condition: DEBE tener left, operator, right
• set-data: DEBE tener data (object)
• gpt/claude/groq: DEBE tener prompt (string con el texto o {{variable}})
• log: puede tener label y message
• delay: DEBE tener duration (número en segundos, max 300)
• email: DEBE tener to, subject, body
• telegram: DEBE tener message
• database: DEBE tener driver, connectionUrl, query
• loop: DEBE tener items (array o referencia {{node-X.output}})
• filter: DEBE tener field, operator, value

Si un nodo NO tiene parámetros configurados, AGREGALOS con valores realistas basados en su contexto.
Si un parámetro está vacío "", PONLE un valor placeholder realista.

Ejemplo de reparación de parámetros:
ANTES (incorrecto):
{
  "id": "node-2",
  "type": "http-request",
  "data": {
    "label": "Get Data",
    "type": "http-request",
    "parameters": {}
  }
}

DESPUÉS (corregido):
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

RESPONDE CON EL FLUJO REPARADO en este formato JSON:
{
  "flowName": "nombre del flujo",
  "flowDescription": "descripción",
  "nodes": [...nodos reparados CON PARÁMETROS COMPLETOS...],
  "edges": [...edges reparadas...],
  "fixes": ["fix 1", "fix 2", ...] // Lista de reparaciones realizadas
}

Responde SOLO con JSON válido.`, string(flowJSON), req.Issues)

	// Groq request using OpenAI-compatible format
	groqReq := GroqRequest{
		Model:       "llama-3.3-70b-versatile",
		Temperature: 0.3,
		MaxTokens:   8192,
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: "Eres un experto en reparar flujos de trabajo de CapyFlow. Devuelve SOLO JSON válido.",
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

	// Limpiar la respuesta (remover markdown code blocks si existen)
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

// getNodeMetadata retorna metadata (subtitle, icon, color, category) para un tipo de nodo
func getNodeMetadata(nodeType string) (subtitle, icon, color, category string) {
	switch nodeType {
	// Triggers
	case "manual-trigger":
		return "Trigger", "IconPlayerPlay", "#3B82F6", "trigger"
	case "webhook-trigger":
		return "Trigger", "IconWebhook", "#06B6D4", "trigger"

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
	case "gpt":
		return "AI", "IconBrain", "#10A37F", "ai"
	case "claude":
		return "AI", "IconSparkles", "#D97757", "ai"
	case "groq":
		return "AI", "IconBolt", "#F55036", "ai"

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

// processAINodes aplica validaciones y defaults a los nodos generados por IA
func processAINodes(nodesAny []any) ([]any, error) {
	if len(nodesAny) == 0 {
		return nodesAny, nil
	}

	// Convertir []any a JSON
	nodesJSON, err := json.Marshal(nodesAny)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %v", err)
	}

	// Primero unmarshaling a un struct intermedio que acepta position como objeto
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

	// Convertir a []models.Node serializando los campos necesarios
	var nodes []models.Node
	for _, input := range inputNodes {
		fmt.Printf("🔍 [ProcessNodes] Processing node: id='%s', type='%s', label='%s'\n", input.ID, input.Type, input.Label)

		// Si viene en formato React Flow con data anidado, extraer campos
		if input.Data != nil && len(input.Data) > 0 {
			if label, ok := input.Data["label"].(string); ok && input.Label == "" {
				input.Label = label
			}
			// SIEMPRE preferir el tipo real de data.type sobre el tipo React Flow
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

		// SIEMPRE auto-asignar metadata basada en el tipo de nodo
		// Esto garantiza consistencia sin importar lo que la IA retorne
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

		// Serializar position a JSON string
		positionJSON, err := json.Marshal(input.Position)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal position for node %s: %v", input.Label, err)
		}

		// Serializar parameters
		var parametersJSON string
		if input.Parameters != nil {
			params, err := json.Marshal(input.Parameters)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal parameters for node %s: %v", input.Label, err)
			}
			parametersJSON = string(params)
		}

		// Serializar inputs
		var inputsJSON string
		if input.Inputs != nil {
			inputs, err := json.Marshal(input.Inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal inputs for node %s: %v", input.Label, err)
			}
			inputsJSON = string(inputs)
		}

		// Serializar outputs
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

	// Aplicar defaults y validar cada nodo
	for i := range nodes {
		fmt.Printf("🔧 [ProcessNodes] Before ApplyDefaults: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)

		// Aplicar defaults automáticos
		if err := validators.ApplyDefaults(&nodes[i]); err != nil {
			return nil, fmt.Errorf("error applying defaults to node %s: %v", nodes[i].Label, err)
		}

		fmt.Printf("🔧 [ProcessNodes] After ApplyDefaults: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)

		// Validar parámetros
		if err := validators.ValidateNode(&nodes[i]); err != nil {
			return nil, fmt.Errorf("validation error for node %s: %v", nodes[i].Label, err)
		}

		fmt.Printf("✅ [ProcessNodes] After ValidateNode: nodes[%d].Subtitle='%s', Icon='%s', Color='%s'\n",
			i, nodes[i].Subtitle, nodes[i].Icon, nodes[i].Color)
	}

	// Convertir de vuelta a formato React Flow con data anidado
	var reactFlowNodes []map[string]interface{}
	for _, node := range nodes {
		// Parsear position de string a objeto
		var position map[string]interface{}
		if err := json.Unmarshal([]byte(node.Position), &position); err != nil {
			position = map[string]interface{}{"x": 0, "y": 0}
		}

		// Parsear parameters de string a objeto
		var parameters map[string]interface{}
		if node.Parameters != "" {
			if err := json.Unmarshal([]byte(node.Parameters), &parameters); err != nil {
				parameters = make(map[string]interface{})
			}
		} else {
			parameters = make(map[string]interface{})
		}

		// Parsear inputs de string a array
		var inputs []interface{}
		if node.Inputs != "" {
			if err := json.Unmarshal([]byte(node.Inputs), &inputs); err != nil {
				inputs = []interface{}{}
			}
		} else {
			inputs = []interface{}{}
		}

		// Parsear outputs de string a array
		var outputs []interface{}
		if node.Outputs != "" {
			if err := json.Unmarshal([]byte(node.Outputs), &outputs); err != nil {
				outputs = []interface{}{}
			}
		} else {
			outputs = []interface{}{}
		}

		// Crear nodo en formato React Flow
		reactFlowNode := map[string]interface{}{
			"id":       node.ID,
			"type":     "custom", // React Flow usa "custom" como tipo de componente
			"position": position,
			"data": map[string]interface{}{
				"label":       node.Label,
				"type":        node.Type, // El tipo REAL del nodo va en data.type
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

	// Log para verificar que la metadata está presente
	for _, rfNode := range reactFlowNodes {
		if data, ok := rfNode["data"].(map[string]interface{}); ok {
			fmt.Printf("📦 React Flow node '%s': type='%s', subtitle='%s', icon='%s', color='%s', category='%s'\n",
				data["label"], data["type"], data["subtitle"], data["icon"], data["color"], data["category"])
		}
	}

	// Convertir a []any
	var result []any
	for _, node := range reactFlowNodes {
		result = append(result, node)
	}

	return result, nil
}
