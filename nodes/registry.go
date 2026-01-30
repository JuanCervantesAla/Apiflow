package nodes

var Registry = map[string]NodeHandler{
	"manual-trigger":  &ManualTriggerNode{},
	"set-data":        &SetDataNode{},
	"log":             &LogNode{},
	"if-condition":    &IfConditionNode{},
	"http-request":    &HttpRequestNode{},
	"json-parser":     &JsonParserNode{},
	"webhook-trigger": &WebhookTriggerNode{},
	"transform-data":  &TransformDataNode{},
}
