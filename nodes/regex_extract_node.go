package nodes

import (
	"encoding/json"
	"fmt"
	"regexp"

	"capyflow/api/models"
)

type RegexExtractNode struct{}

type RegexExtractParams struct {
	InputData string `json:"inputData"` // Text to search in
	Pattern   string `json:"pattern"`   // Regex pattern
	Mode      string `json:"mode"`      // "first", "all", "groups"
	Flags     string `json:"flags"`     // "i" for case-insensitive, "m" for multiline
}

func (n *RegexExtractNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params RegexExtractParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Validate required fields
	if params.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if params.InputData == "" {
		return map[string]interface{}{
			"matches": []string{},
			"count":   0,
			"pattern": params.Pattern,
		}, nil
	}

	// Default mode
	if params.Mode == "" {
		params.Mode = "first"
	}

	// Apply flags to pattern
	pattern := params.Pattern
	if params.Flags != "" {
		// Go regex doesn't use inline flags like JavaScript
		// We handle case-insensitive manually with (?i) prefix
		if containsFlag(params.Flags, 'i') {
			pattern = "(?i)" + pattern
		}
		if containsFlag(params.Flags, 'm') {
			pattern = "(?m)" + pattern
		}
		if containsFlag(params.Flags, 's') {
			pattern = "(?s)" + pattern
		}
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	// Execute based on mode
	switch params.Mode {
	case "first":
		return n.extractFirst(re, params.InputData, params.Pattern)
	case "all":
		return n.extractAll(re, params.InputData, params.Pattern)
	case "groups":
		return n.extractGroups(re, params.InputData, params.Pattern)
	default:
		return nil, fmt.Errorf("invalid mode: %s (must be 'first', 'all', or 'groups')", params.Mode)
	}
}

func (n *RegexExtractNode) extractFirst(re *regexp.Regexp, text string, pattern string) (map[string]interface{}, error) {
	match := re.FindString(text)

	var matches []string
	count := 0
	if match != "" {
		matches = []string{match}
		count = 1
	} else {
		matches = []string{}
	}

	return map[string]interface{}{
		"match":   match,
		"matches": matches,
		"count":   count,
		"pattern": pattern,
	}, nil
}

func (n *RegexExtractNode) extractAll(re *regexp.Regexp, text string, pattern string) (map[string]interface{}, error) {
	matches := re.FindAllString(text, -1)

	if matches == nil {
		matches = []string{}
	}

	return map[string]interface{}{
		"matches": matches,
		"count":   len(matches),
		"pattern": pattern,
	}, nil
}

func (n *RegexExtractNode) extractGroups(re *regexp.Regexp, text string, pattern string) (map[string]interface{}, error) {
	// Find all matches with subgroups
	allMatches := re.FindAllStringSubmatch(text, -1)

	if allMatches == nil {
		return map[string]interface{}{
			"groups":  []interface{}{},
			"count":   0,
			"pattern": pattern,
		}, nil
	}

	// Convert to array of groups
	var groups []interface{}
	for _, match := range allMatches {
		// match[0] is the full match, match[1:] are the capture groups
		groupObj := make(map[string]interface{})
		groupObj["full"] = match[0]

		// Add numbered groups
		if len(match) > 1 {
			namedGroups := make([]string, len(match)-1)
			for i := 1; i < len(match); i++ {
				namedGroups[i-1] = match[i]
			}
			groupObj["groups"] = namedGroups
		} else {
			groupObj["groups"] = []string{}
		}

		groups = append(groups, groupObj)
	}

	return map[string]interface{}{
		"groups":  groups,
		"count":   len(groups),
		"pattern": pattern,
	}, nil
}

// containsFlag checks if flags string contains a specific flag character
func containsFlag(flags string, flag rune) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}
