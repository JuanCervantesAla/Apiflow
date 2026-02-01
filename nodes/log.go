package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
)

type LogNode struct{}

type LogParams struct {
	Label string `json:"label"`
	Level string `json:"level"`
	Key   string `json:"key"`
}

func (n *LogNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	params := LogParams{
		Label: "LOG",
		Level: "info",
	}

	if node.Parameters != "" {
		_ = json.Unmarshal([]byte(node.Parameters), &params)
	}

	var value interface{} = prev

	if params.Key != "" {
		for _, out := range prev {
			if v, ok := out[params.Key]; ok {
				value = v
				break
			}
		}
	}

	fmt.Printf(
		"[LOG][%s][%s] %+v\n",
		params.Level,
		params.Label,
		value,
	)

	return map[string]interface{}{
		"value": value,
	}, nil
}
