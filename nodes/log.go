package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
)

type LogNode struct{}

type LogParams struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

func (n *LogNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	params := LogParams{
		Message: "",
		Level:   "info",
	}

	if node.Parameters != "" {
		if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
			return nil, fmt.Errorf("Invalid log parameters")
		}
	}

	message := params.Message
	message = interpolateString(message, prev)

	fmt.Printf(
		"[LOG][%s] %s\n",
		params.Level,
		message,
	)

	return map[string]interface{}{
		"message": message,
		"level":   params.Level,
	}, nil
}
