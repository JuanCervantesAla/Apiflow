package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"time"
)

// CronTriggerNode is a trigger that starts a flow on a scheduled interval.
// The actual scheduling is handled by the CronService; this node just
// produces a basic output context when the flow starts.
type CronTriggerNode struct{}

type CronTriggerParams struct {
	// IntervalMinutes is duplicated here mainly for transparency in the
	// node parameters. The scheduler also reads this from the same JSON.
	IntervalMinutes int `json:"intervalMinutes"`
}

func (n *CronTriggerNode) Execute(
	node *models.Node,
	context map[string]map[string]interface{},
) (map[string]interface{}, error) {
	output := map[string]interface{}{
		"triggered": true,
		"source":    "cron",
		"timestamp": time.Now().Unix(),
	}

	// Best-effort: expose the interval configured on this node (if any)
	// so it can be used by downstream nodes if needed.
	if node.Parameters != "" {
		var params CronTriggerParams
		if err := json.Unmarshal([]byte(node.Parameters), &params); err == nil {
			if params.IntervalMinutes > 0 {
				output["intervalMinutes"] = params.IntervalMinutes
			}
		} else {
			// Do not fail the execution because of invalid params; just report.
			output["warning"] = fmt.Sprintf("invalid cron parameters: %v", err)
		}
	}

	// Also expose any initial context injected by the scheduler under
	// the "cron" key for convenience.
	if cronCtx, ok := context["cron"]; ok && cronCtx != nil {
		for k, v := range cronCtx {
			// Flatten cron context into the output with a prefix
			output[fmt.Sprintf("cron_%s", k)] = v
		}
	}

	return output, nil
}
