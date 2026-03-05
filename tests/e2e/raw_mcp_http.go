//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func newMCPHTTPClient() *http.Client {
	if strings.ToLower(useInsecureClient) == "true" {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return &http.Client{}
}

// mcpPost sends a raw HTTP POST to the MCP gateway and returns the response.
// Caller is responsible for closing the response body.
func mcpPost(url, sessionID string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	return newMCPHTTPClient().Do(req)
}

// mcpInitialize sends an initialize request and returns the session ID from the response header
func mcpInitialize(url string) (string, error) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"e2e-raw","version":"0.0.1"}}}`
	resp, err := mcpPost(url, "", []byte(body))
	if err != nil {
		return "", fmt.Errorf("initialize request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("initialize returned status %d", resp.StatusCode)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		return "", fmt.Errorf("no Mcp-Session-Id in initialize response")
	}
	return sessionID, nil
}

// mcpNotifyInitialized sends the notifications/initialized notification
func mcpNotifyInitialized(url, sessionID string) error {
	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resp, err := mcpPost(url, sessionID, []byte(body))
	if err != nil {
		return fmt.Errorf("notifications/initialized failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// mcpCallTool sends a tools/call request and returns the parsed result content
func mcpCallTool(url, sessionID, toolName string) ([]toolContent, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s"}}`, toolName)
	resp, err := mcpPost(url, sessionID, []byte(body))
	if err != nil {
		return nil, fmt.Errorf("tools/call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tools/call returned status %d: %s", resp.StatusCode, string(respBody))
	}

	result, err := readJSONRPCResult(resp)
	if err != nil {
		return nil, err
	}

	var callResult struct {
		Content []toolContent `json:"content"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w: %s", err, string(result))
	}
	return callResult.Content, nil
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// extractBackendSession finds the backend Mcp-Session-Id from tool content
func extractBackendSession(content []toolContent) string {
	for _, c := range content {
		if c.Type == "text" && strings.HasPrefix(c.Text, "Mcp-Session-Id") {
			return c.Text
		}
	}
	return ""
}

// readJSONRPCResult reads a JSON-RPC result from an HTTP response, handling both JSON and SSE content types
func readJSONRPCResult(resp *http.Response) (json.RawMessage, error) {
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		return parseSSEResult(rawBody)
	}

	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rawBody, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w: %s", err, string(rawBody))
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s", string(msg.Error))
	}
	return msg.Result, nil
}

// parseSSEResult extracts the JSON-RPC result from an SSE response body
func parseSSEResult(body []byte) (json.RawMessage, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var msg struct {
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal([]byte(data), &msg) == nil && msg.Result != nil {
			return msg.Result, nil
		}
	}
	return nil, fmt.Errorf("no result found in SSE response: %s", string(body))
}
