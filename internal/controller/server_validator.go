package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Kuadrant/mcp-gateway/internal/broker"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServerValidator validates MCP servers by calling broker endpoints
type ServerValidator struct {
	k8sClient  client.Client
	httpClient *http.Client
}

// NewServerValidator creates a new server validator
func NewServerValidator(k8sClient client.Client) *ServerValidator {
	return &ServerValidator{
		k8sClient: k8sClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateServers validates MCP servers by calling the mcp-gateway service's /status endpoint
func (v *ServerValidator) ValidateServers(ctx context.Context, namespace string) (*broker.StatusResponse, error) {
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080/status", brokerRouterName, namespace)
	return v.getStatusFromEndpoint(ctx, url)
}

func (v *ServerValidator) getStatusFromEndpoint(ctx context.Context, url string) (*broker.StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status %d", resp.StatusCode)
	}

	var status broker.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}
