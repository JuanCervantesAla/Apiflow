package services

import (
	"capyflow/api/models"
	"capyflow/api/rules"
	"time"
)

type ExecutorService struct{}

func NewExecutorService() *ExecutorService {
	return &ExecutorService{}
}

func (es *ExecutorService) ExecuteFlow(flow *models.Flow) *FlowExecutionResult {
	return es.execute(flow, map[string]map[string]interface{}{})
}

func (es *ExecutorService) ExecuteFlowWithContext(
	flow *models.Flow,
	initialContext map[string]map[string]interface{},
) *FlowExecutionResult {
	return es.execute(flow, initialContext)
}

func (es *ExecutorService) execute(
	flow *models.Flow,
	initialContext map[string]map[string]interface{},
) *FlowExecutionResult {

	start := time.Now()

	result := &FlowExecutionResult{
		Status:        "success",
		ExecutedNodes: []string{},
		Results:       map[string]*ExecutionResult{},
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
