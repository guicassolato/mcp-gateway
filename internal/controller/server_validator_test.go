package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kuadrant/mcp-gateway/internal/broker"
	"github.com/Kuadrant/mcp-gateway/internal/broker/upstream"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServerValidator_getStatusFromEndpoint(t *testing.T) {
	testCases := []struct {
		name           string
		responseCode   int
		responseBody   interface{}
		expectErr      bool
		errContains    string
		expectedStatus *broker.StatusResponse
	}{
		{
			name:         "successful response",
			responseCode: http.StatusOK,
			responseBody: broker.StatusResponse{
				OverallValid: true,
				Servers: []upstream.ServerValidationStatus{
					{Name: "server1", Ready: true},
				},
				Timestamp: time.Now(),
			},
			expectErr: false,
			expectedStatus: &broker.StatusResponse{
				OverallValid: true,
				Servers: []upstream.ServerValidationStatus{
					{Name: "server1", Ready: true},
				},
			},
		},
		{
			name:         "server error response",
			responseCode: http.StatusInternalServerError,
			responseBody: nil,
			expectErr:    true,
			errContains:  "received status 500",
		},
		{
			name:         "invalid JSON response",
			responseCode: http.StatusOK,
			responseBody: "invalid json",
			expectErr:    true,
			errContains:  "failed to decode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.responseCode)
				if tc.responseBody != nil {
					switch v := tc.responseBody.(type) {
					case string:
						_, _ = w.Write([]byte(v))
					default:
						_ = json.NewEncoder(w).Encode(v)
					}
				}
			}))
			defer server.Close()

			validator := &ServerValidator{
				httpClient: server.Client(),
			}

			status, err := validator.getStatusFromEndpoint(context.Background(), server.URL)

			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, status)
				require.Equal(t, tc.expectedStatus.OverallValid, status.OverallValid)
				require.Len(t, status.Servers, len(tc.expectedStatus.Servers))
			}
		})
	}
}

func TestNewServerValidator(t *testing.T) {
	scheme := runtime.NewScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	validator := NewServerValidator(k8sClient)
	require.NotNil(t, validator)
	require.NotNil(t, validator.httpClient)
}
