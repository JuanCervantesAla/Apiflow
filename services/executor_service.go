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

	es.executeNode(
		startNode.ID,
		nodeMap,
		edgeMap,
		nodeOutputs,
		visited,
		result,
	)

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}
