package apicompat

import "strings"

func canonicalClaudeToolName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimLeft(name, "_")
	return strings.ToLower(name)
}

// ClaudeToolNameMapFromTools builds a canonical->original name map from tools.
// Canonical collisions are resolved by keeping the first seen tool name.
func ClaudeToolNameMapFromTools(tools []ResponsesTool) map[string]string {
	nameMap := make(map[string]string, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || tool.Name == "" {
			continue
		}
		canonical := canonicalClaudeToolName(tool.Name)
		if canonical == "" {
			continue
		}
		if _, exists := nameMap[canonical]; exists {
			continue
		}
		nameMap[canonical] = tool.Name
	}
	return nameMap
}

// MapClaudeToolName restores an original Claude tool name using a canonical map.
// If there is no match, the input name is returned unchanged.
func MapClaudeToolName(name string, nameMap map[string]string) string {
	if len(nameMap) == 0 || name == "" {
		return name
	}
	if original, ok := nameMap[canonicalClaudeToolName(name)]; ok {
		return original
	}
	return name
}
