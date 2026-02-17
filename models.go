package zen

import (
	"context"
	"encoding/json"
)

type ModelsResponse struct {
	Data []Model         `json:"data"`
	Raw  json.RawMessage `json:"-"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func (c *Client) ListModels(ctx context.Context) (*ModelsResponse, error) {
	data, _, err := c.doRequest(ctx, "GET", "/models", nil)
	if err != nil {
		return nil, err
	}

	var resp ModelsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	resp.Raw = json.RawMessage(data)
	return &resp, nil
}
