package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"time"
)

type DelayNode struct{}

type DelayParams struct {
	Duration int    `json:"duration"` // Milisegundos
	Reason   string `json:"reason"`   // Opcional: descripción del delay
}

func (n *DelayNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params DelayParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error al decodificar parámetros: %v", err)
	}

	// Validar duración
	if params.Duration <= 0 {
		return nil, fmt.Errorf("la duración debe ser mayor a 0ms")
	}

	// Límite máximo de 5 minutos (300000ms) para evitar timeouts excesivos
	if params.Duration > 300000 {
		return nil, fmt.Errorf("la duración máxima es 300000ms (5 minutos)")
	}

	// Ejecutar el delay
	time.Sleep(time.Duration(params.Duration) * time.Millisecond)

	result := map[string]interface{}{
		"delayed":    true,
		"durationMs": params.Duration,
	}

	if params.Reason != "" {
		result["reason"] = params.Reason
	}

	return result, nil
}
