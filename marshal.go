package zen

import "encoding/json"

func marshalWithExtra(base map[string]any, extra map[string]any) ([]byte, error) {
	if len(extra) == 0 {
		return json.Marshal(base)
	}

	for k, v := range extra {
		if _, exists := base[k]; exists {
			continue
		}
		base[k] = v
	}

	return json.Marshal(base)
}
