package broker

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kagenti/mcp-gateway/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestStatusHandlerNotGet(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mcpBroker := NewBroker(logger, brokerOpts)
	sh := NewStatusHandler(mcpBroker, *logger)

	w := httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/status", nil))
	res := w.Result()
	require.Equal(t, 405, res.StatusCode)
}

func TestStatusHandlerGetSingleServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mcpBroker := NewBroker(logger, brokerOpts)
	sh := NewStatusHandler(mcpBroker, *logger)

	// At first, no server known for this name
	w := httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status/dummyServer", nil))
	res := w.Result()
	require.Equal(t, 404, res.StatusCode)

	w = httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status/?server=dummyServer", nil))
	res = w.Result()
	require.Equal(t, 404, res.StatusCode)

	// Add a server
	brokerImpl, ok := mcpBroker.(*mcpBrokerImpl)
	require.True(t, ok)
	brokerImpl.mcpServers["dummyServer"] = &upstreamMCP{
		MCPServer: config.MCPServer{Name: "dummyServer"},
		initializeResult: &mcp.InitializeResult{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		},
		toolsResult: &mcp.ListToolsResult{
			Tools: []mcp.Tool{
				{
					Name: "dummyTool",
				},
			},
		},
	}

	w = httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status/dummyServer", nil))
	res = w.Result()
	require.Equal(t, 200, res.StatusCode)

	w = httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status/?server=dummyServer", nil))
	res = w.Result()
	require.Equal(t, 200, res.StatusCode)
}

func TestStatusHandlerGetAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mcpBroker := NewBroker(logger, brokerOpts)
	sh := NewStatusHandler(mcpBroker, *logger)

	// Add a server
	brokerImpl, ok := mcpBroker.(*mcpBrokerImpl)
	require.True(t, ok)
	brokerImpl.mcpServers["dummyServer"] = &upstreamMCP{
		MCPServer: config.MCPServer{Name: "dummyServer"},
		initializeResult: &mcp.InitializeResult{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		},
		toolsResult: &mcp.ListToolsResult{
			Tools: []mcp.Tool{
				{
					Name: "dummyTool",
				},
			},
		},
	}

	w := httptest.NewRecorder()
	sh.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status", nil))
	res := w.Result()
	require.Equal(t, 200, res.StatusCode)
	data, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	m := make(map[string]interface{})
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)
}
