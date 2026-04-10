package service

import (
	"bytes"
	"encoding/json"
	"sort"
)

var openAIResponsesRequestOrderedKeys = []string{
	"model",
	"instructions",
	"tools",
	"tool_choice",
	"parallel_tool_calls",
	"include",
	"reasoning",
	"service_tier",
	"temperature",
	"top_p",
	"max_output_tokens",
	"store",
	"stream",
	"prompt_cache_key",
	"input",
	"previous_response_id",
}

// marshalOpenAIResponsesRequestBodyOrdered keeps stable, high-cardinality fields
// ahead of dynamic input so upstream prompt-cache matching can consume a longer
// fixed prefix without changing the payload content.
func marshalOpenAIResponsesRequestBodyOrdered(reqBody map[string]any) ([]byte, error) {
	if len(reqBody) == 0 {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	_ = buf.WriteByte('{')
	first := true
	written := make(map[string]struct{}, len(reqBody))

	writeField := func(key string, value any) error {
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return err
		}
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if !first {
			_ = buf.WriteByte(',')
		}
		first = false
		_, _ = buf.Write(keyJSON)
		_ = buf.WriteByte(':')
		_, _ = buf.Write(valueJSON)
		written[key] = struct{}{}
		return nil
	}

	for _, key := range openAIResponsesRequestOrderedKeys {
		value, ok := reqBody[key]
		if !ok {
			continue
		}
		if err := writeField(key, value); err != nil {
			return nil, err
		}
	}

	remainingKeys := make([]string, 0, len(reqBody))
	for key := range reqBody {
		if _, ok := written[key]; ok {
			continue
		}
		remainingKeys = append(remainingKeys, key)
	}
	sort.Strings(remainingKeys)
	for _, key := range remainingKeys {
		if err := writeField(key, reqBody[key]); err != nil {
			return nil, err
		}
	}

	_ = buf.WriteByte('}')
	return buf.Bytes(), nil
}
