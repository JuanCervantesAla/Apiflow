package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
)

// Struct of the type of the node
type SetDataNode struct{}

// ACTION: Sets the node output by mapping the specific 'values' object from the provided parameters.
func (n *SetDataNode) Execute(
	node *models.Node, //Takes the node model meaning the class
	prev map[string]map[string]interface{}, //Creates a Map of string,object
) (map[string]interface{}, error) {
	var params map[string]interface{}                                        //Params
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil { //If params are note correct or is null
		return nil, fmt.Errorf("Invalid parameters in set-data") //Error
	}

	values, ok := params["values"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Set-data Requieres 'values' object")
	}

	output := map[string]interface{}{}

	for k, v := range values { //Search the values in the range
		output[k] = v
	}

	return output, nil //Return the setted output
}
