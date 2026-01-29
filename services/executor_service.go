package services

import (
	"capyflow/api/models"
	"capyflow/api/rules"
	"encoding/json"
	"fmt"
	"time"
)

type ExecutionResult struct {
	NodeID     string                 `json:"nodeId"`
	Status     models.NodeStatus      `json:"status"`
	Output     map[string]interface{} `json:"output"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int64                  `json:"durationMs"`
}

type FlowExecutionResult struct {
	Status        string                      `json:"status"`
	ExecutedNodes []string                    `json:"executedNodes"`
	Results       map[string]*ExecutionResult `json:"results"`
	DurationMs    int64                       `json:"durationMs"`
	ErrorMessage  string                      `json:"errorMessage,omitempty"`
}

type ExecutorService struct{}

func NewExecutorService() *ExecutorService {
	return &ExecutorService{}
}

func (es *ExecutorService) ExecuteFlow(flow *models.Flow) *FlowExecutionResult {
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
	edgeMap := map[string][]string{}

	for i := range flow.Nodes {
		nodeMap[flow.Nodes[i].ID] = &flow.Nodes[i]
	}

	for _, e := range flow.Edges {
		edgeMap[e.Source] = append(edgeMap[e.Source], e.Target)
	}

	var currentNode *models.Node
	for _, n := range flow.Nodes {
		if n.Category == string(models.CategoryTrigger) {
			currentNode = &n
			break
		}
	}

	nodeOutputs := map[string]map[string]interface{}{}
	visited := map[string]bool{}

	es.executeNode(currentNode.ID, nodeMap, edgeMap, nodeOutputs, visited, result)

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func (es *ExecutorService) executeNode(
	nodeID string,
	nodeMap map[string]*models.Node,
	edgeMap map[string][]string,
	nodeOutputs map[string]map[string]interface{},
	visited map[string]bool,
	result *FlowExecutionResult,
) {
	if visited[nodeID] {
		return
	}
	visited[nodeID] = true

	node := nodeMap[nodeID]
	start := time.Now()

	output, err := es.processNode(node, nodeOutputs)

	exec := &ExecutionResult{
		NodeID: nodeID,
	}

	if err != nil {
		exec.Status = models.StatusError
		exec.Error = err.Error()
		result.Status = "partial"
	} else {
		exec.Status = models.StatusSuccess
		exec.Output = output
		nodeOutputs[nodeID] = output
	}

	exec.DurationMs = time.Since(start).Milliseconds()
	result.Results[nodeID] = exec
	result.ExecutedNodes = append(result.ExecutedNodes, nodeID)

	for _, next := range edgeMap[nodeID] {
		es.executeNode(next, nodeMap, edgeMap, nodeOutputs, visited, result)
	}
}

func (es *ExecutorService) processNode(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	out := map[string]interface{}{}

	switch node.Type {

	case "manual-trigger":
		out["triggered"] = true
		out["timestamp"] = time.Now().Unix()
		out["message"] = "Flujo iniciado manualmente"

	case "webhook-trigger":
		out["triggered"] = true
		out["message"] = "Flujo activado por webhook"

	case "set-data":
		var params map[string]interface{}
		json.Unmarshal([]byte(node.Parameters), &params)
		for k, v := range params {
			out[k] = v
		}

	case "transform-data":
		out["result"] = prev

	case "json-parser":
		var parsed interface{}
		if err := json.Unmarshal([]byte(node.Parameters), &parsed); err != nil {
			return nil, err
		}
		out["parsed"] = parsed

	case "if-condition":
		out["condition"] = true

	case "http-request":
		out["status"] = 200
		out["response"] = map[string]interface{}{"ok": true}

	case "custom":
		out["result"] = prev

	default:
		return nil, fmt.Errorf("nodo no implementado")
	}

	time.Sleep(50 * time.Millisecond)
	return out, nil
}
