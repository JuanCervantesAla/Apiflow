package services

import (
	"capyflow/api/models"
	"capyflow/api/websocket"
	"time"
)

func (es *ExecutorService) executeNode(
	nodeID string,
	nodeMap map[string]*models.Node,
	edgeMap map[string][]models.Edge,
	nodeOutputs map[string]map[string]interface{},
	visited map[string]bool,
	result *FlowExecutionResult,
) {
	if visited[nodeID] {
		return
	}
	visited[nodeID] = true

	node, ok := nodeMap[nodeID]
	if !ok {
		return
	}

	start := time.Now()
	exec := &ExecutionResult{NodeID: nodeID}

	if es.Hub != nil && result.UserID != "" {
		es.Hub.BroadcastToUser(result.UserID, websocket.ExecutionUpdate{
			Type:        "node",
			ExecutionID: result.ExecutionID,
			FlowID:      "",
			NodeID:      nodeID,
			Status:      "running",
			Message:     "Executing " + node.Label,
			Timestamp:   time.Now().Format(time.RFC3339),
		})
	}

	output, err := es.processNode(node, nodeOutputs)
	if err != nil {
		exec.Status = models.StatusError
		exec.Error = err.Error()
		exec.DurationMs = time.Since(start).Milliseconds()

		result.Status = "partial"
		result.Results[nodeID] = exec
		result.ExecutedNodes = append(result.ExecutedNodes, nodeID)
		return
	}

	exec.Status = models.StatusSuccess
	exec.Output = output
	exec.DurationMs = time.Since(start).Milliseconds()

	nodeOutputs[nodeID] = output
	result.Results[nodeID] = exec
	result.ExecutedNodes = append(result.ExecutedNodes, nodeID)

	edges := edgeMap[nodeID]

	if node.Type == "if-condition" {
		cond, ok := output["condition"].(bool)
		if !ok {
			exec.Status = models.StatusError
			exec.Error = "Invalid condition result"
			result.Status = "partial"
			return
		}

		branch := "false"
		if cond {
			branch = "true"
		}

		for _, e := range edges {
			if e.Label == branch {
				es.executeNode(
					e.Target,
					nodeMap,
					edgeMap,
					nodeOutputs,
					visited,
					result,
				)
				return
			}
		}

		exec.Status = models.StatusError
		exec.Error = "No valid branch found for IF node"
		result.Status = "partial"
		return
	}

	for _, e := range edges {
		es.executeNode(
			e.Target,
			nodeMap,
			edgeMap,
			nodeOutputs,
			visited,
			result,
		)
	}
}
