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
	"loop":            &LoopNode{},
	"delay":           &DelayNode{},
	"gemini":          &GeminiNode{},
	"gpt":             &GPTNode{},
	"claude":          &ClaudeNode{},
	"filter":          &FilterNode{},
	"split":           &SplitNode{},
	"merge":           &MergeNode{},
	"function":        &FunctionNode{},
}
