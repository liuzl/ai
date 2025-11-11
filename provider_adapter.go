package ai

import (
	"context"
	"fmt"
)

// providerAdapter defines the interface for provider-specific logic,
// allowing the core client to remain generic.
type providerAdapter interface {
	// buildRequestPayload converts the universal Request into the provider-specific
	// request body struct.
	buildRequestPayload(req *Request) (any, error)

	// parseResponse converts the provider-specific JSON response body
	// into the universal Response.
	parseResponse(providerResp []byte) (*Response, error)

	// getModel returns the default model for the provider if not specified in the request.
	getModel(req *Request) string

	// getEndpoint returns the API endpoint for the generation request.
	getEndpoint(model string) string
}

// genericClient handles the common logic for making AI requests, delegating
// provider-specific tasks to an adapter.
type genericClient struct {
	b       *baseClient
	adapter providerAdapter
}

// Generate implements the core logic for the Client interface.
func (c *genericClient) Generate(ctx context.Context, req *Request) (*Response, error) {
	// 0. Validate the request before processing
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 1. Build the provider-specific request payload using the adapter.
	payload, err := c.adapter.buildRequestPayload(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build request payload: %w", err)
	}

	// 2. Get model and endpoint from the adapter.
	model := c.adapter.getModel(req)
	endpoint := c.adapter.getEndpoint(model)

	// 3. Make the raw HTTP request.
	respBytes, err := c.b.doRequestRaw(ctx, "POST", endpoint, payload)
	if err != nil {
		return nil, err
	}

	// 4. Convert the provider-specific response to the universal response using the adapter.
	return c.adapter.parseResponse(respBytes)
}
