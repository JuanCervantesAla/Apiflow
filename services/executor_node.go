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

	output, err := es.processNode(node, nodeOutputs, result.UserID, result.ExecutedNodes)
	if err != nil {
		exec.Status = models.StatusError
		exec.Error = err.Error()
		exec.DurationMs = time.Since(start).Milliseconds()

		result.Status = "partial"
		result.Results[nodeID] = exec
		result.ExecutedNodes = append(result.ExecutedNodes, nodeID)

		// Send node error message
		if es.Hub != nil && result.UserID != "" {
			es.Hub.BroadcastToUser(result.UserID, websocket.ExecutionUpdate{
				Type:        "node",
				ExecutionID: result.ExecutionID,
				FlowID:      "",
				NodeID:      nodeID,
				Status:      "error",
				Message:     err.Error(),
				Timestamp:   time.Now().Format(time.RFC3339),
			})
		}

		return
	}

	exec.Status = models.StatusSuccess
	exec.Output = output
	exec.DurationMs = time.Since(start).Milliseconds()

	nodeOutputs[nodeID] = output
	result.Results[nodeID] = exec
	result.ExecutedNodes = append(result.ExecutedNodes, nodeID)

	// Send node completed successfully message
	if es.Hub != nil && result.UserID != "" {
		// Create a safe copy of output data without circular references
		safeData := make(map[string]interface{})
		for k, v := range output {
			// Only include simple types to avoid circular references
			switch v.(type) {
			case string, int, int64, float64, bool, nil:
				safeData[k] = v
			case map[string]interface{}:
				// For maps, create a shallow copy
				if mapVal, ok := v.(map[string]interface{}); ok {
					safeCopy := make(map[string]interface{})
					for mk, mv := range mapVal {
						switch mv.(type) {
						case string, int, int64, float64, bool, nil:
							safeCopy[mk] = mv
						}
					}
					safeData[k] = safeCopy
				}
			}
		}

		es.Hub.BroadcastToUser(result.UserID, websocket.ExecutionUpdate{
			Type:        "node",
			ExecutionID: result.ExecutionID,
			FlowID:      "",
			NodeID:      nodeID,
			Status:      "success",
			Message:     "Node completed successfully",
			Data:        safeData,
			Timestamp:   time.Now().Format(time.RFC3339),
		})
	}

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
			// Use sourceHandle if available, otherwise use label
			edgeIdentifier := e.SourceHandle
			if edgeIdentifier == "" {
				edgeIdentifier = e.Label
			}

			if edgeIdentifier == branch {
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

		// Tolerant fallback: if only one edge exists, continue through it.
		if len(edges) == 1 {
			es.executeNode(
				edges[0].Target,
				nodeMap,
				edgeMap,
				nodeOutputs,
				visited,
				result,
			)
			return
		}

		// If branch is not connected, end this path without failing the whole execution.
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
