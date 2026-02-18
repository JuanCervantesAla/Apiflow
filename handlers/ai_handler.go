package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type AIGenerateFlowRequest struct {
	Description string `json:"description" binding:"required"`
	Context     string `json:"context"`
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

// GenerateFlowWithAI handles AI-powered flow generation
func GenerateFlowWithAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AIGenerateFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Get Gemini API key from environment
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success: false,
			Error:   "Gemini API key not configured on server. Please use your own API key in the frontend.",
		})
		return
	}

	// Build the system prompt (simplified version - full version should come from a config)
	systemPrompt := buildSystemPrompt()
	userPrompt := req.Description
	if req.Context != "" {
		userPrompt = req.Context + "\n\n" + req.Description
	}

	// Combine prompts for Gemini
	fullPrompt := fmt.Sprintf("%s\n\nDescripción del usuario:\n%s\n\nResponde SOLO con JSON válido, sin texto adicional.", systemPrompt, userPrompt)

	// Call Gemini API
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
			MaxOutputTokens:  4000,
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

	// Make HTTP request to Gemini
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

	// Check for Gemini error
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

	// Parse the generated flow
	var flow AIGeneratedFlow
	if err := json.Unmarshal([]byte(content), &flow); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success:     false,
			Error:       "Failed to parse generated flow",
			RawResponse: content,
		})
		return
	}

	// Validate flow structure
	if flow.Nodes == nil || len(flow.Nodes) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AIGenerateFlowResponse{
			Success:     false,
			Error:       "Generated flow has no nodes",
			RawResponse: content,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AIGenerateFlowResponse{
		Success:     true,
		Flow:        &flow,
		RawResponse: content,
	})
}

func buildSystemPrompt() string {
	// This is a simplified version. In production, you'd want to:
	// 1. Load this from a file or database
	// 2. Include actual node types from your database
	// 3. Make it configurable

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
