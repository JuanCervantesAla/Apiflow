package services

import (
	"capyflow/api/models"
	"capyflow/api/rules"
	"capyflow/api/websocket"
	"time"
)

type ExecutorService struct {
	Hub *websocket.Hub
}

func NewExecutorService(hub *websocket.Hub) *ExecutorService {
	return &ExecutorService{
		Hub: hub,
	}
}

func (es *ExecutorService) ExecuteFlow(flow *models.Flow, userID, executionID string) *FlowExecutionResult {
	return es.execute(flow, map[string]map[string]interface{}{}, userID, executionID)
}

func (es *ExecutorService) ExecuteFlowWithContext(
	flow *models.Flow,
	initialContext map[string]map[string]interface{},
	userID, executionID string,
) *FlowExecutionResult {
	return es.execute(flow, initialContext, userID, executionID)
}

func (es *ExecutorService) execute(
	flow *models.Flow,
	initialContext map[string]map[string]interface{},
	userID, executionID string,
) *FlowExecutionResult {

	start := time.Now()

	result := &FlowExecutionResult{
		Status:        "success",
		ExecutedNodes: []string{},
		Results:       map[string]*ExecutionResult{},
		UserID:        userID,
		ExecutionID:   executionID,
	}

	if err := rules.ValidateFlow(flow); err != nil {
		result.Status = "error"
		result.ErrorMessage = err.Error()

		// Enviar error detallado por WebSocket
		if es.Hub != nil {
			es.Hub.BroadcastToUser(userID, websocket.ExecutionUpdate{
				Type:        "execution-error",
				ExecutionID: executionID,
				FlowID:      flow.ID,
				Status:      "error",
				Message:     err.Error(),
				Data: map[string]string{
					"details": "Revisa las conexiones del flujo. Los triggers deben estar al inicio y conectar hacia otros nodos (no recibir conexiones).",
				},
				Timestamp: time.Now().Format(time.RFC3339),
			})
		}

		return result
	}

	nodeMap := map[string]*models.Node{}
	edgeMap := map[string][]models.Edge{}

	for i := range flow.Nodes {
		nodeMap[flow.Nodes[i].ID] = &flow.Nodes[i]
	}

	for _, e := range flow.Edges {
		edgeMap[e.Source] = append(edgeMap[e.Source], e)
	}

	var triggers []*models.Node
	for i := range flow.Nodes {
		if flow.Nodes[i].Category == string(models.CategoryTrigger) {
			triggers = append(triggers, &flow.Nodes[i])
		}
	}

	if len(triggers) == 0 {
		result.Status = "error"
		result.ErrorMessage = "No trigger node found"
		return result
	}

	if len(triggers) > 1 {
		result.Status = "error"
		result.ErrorMessage = "Multiple trigger nodes are not supported"
		return result
	}

	startNode := triggers[0]

	nodeOutputs := map[string]map[string]interface{}{}
	for k, v := range initialContext {
		cloned := map[string]interface{}{}
		for kk, vv := range v {
			cloned[kk] = vv
		}
		nodeOutputs[k] = cloned
	}

	visited := map[string]bool{}

	if es.Hub != nil && userID != "" {
		es.Hub.BroadcastToUser(userID, websocket.ExecutionUpdate{
			Type:        "start",
			ExecutionID: executionID,
			FlowID:      flow.ID,
			Status:      "running",
			Message:     "Execution started",
			Timestamp:   time.Now().Format(time.RFC3339),
		})
	}

	es.executeNode(
		startNode.ID,
		nodeMap,
		edgeMap,
		nodeOutputs,
		visited,
		result,
	)

	result.DurationMs = time.Since(start).Milliseconds()

	// Enviar mensaje de finalización
	if es.Hub != nil && userID != "" {
		updateType := "complete"
		message := "Execution completed successfully"
		if result.Status == "error" || result.Status == "partial" {
			updateType = "error"
			message = result.ErrorMessage
			if message == "" {
				message = "Execution completed with errors"
			}
		}

		es.Hub.BroadcastToUser(userID, websocket.ExecutionUpdate{
			Type:        updateType,
			ExecutionID: executionID,
			FlowID:      flow.ID,
			Status:      result.Status,
			Message:     message,
			Timestamp:   time.Now().Format(time.RFC3339),
		})
	}

	return result
}
