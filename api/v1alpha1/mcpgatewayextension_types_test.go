package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMCPGatewayExtension_PublicHost(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name:        "nil annotations returns empty string",
			annotations: nil,
			want:        "",
		},
		{
			name:        "empty annotations returns empty string",
			annotations: map[string]string{},
			want:        "",
		},
		{
			name: "annotation not present returns empty string",
			annotations: map[string]string{
				"other-annotation": "value",
			},
			want: "",
		},
		{
			name: "annotation present returns value",
			annotations: map[string]string{
				AnnotationPublicHost: "mcp.example.com",
			},
			want: "mcp.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if got := m.PublicHost(); got != tt.want {
				t.Errorf("PublicHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPGatewayExtension_InternalHost(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		targetRef MCPGatewayExtensionTargetReference
		want      string
	}{
		{
			name:      "uses targetRef namespace when specified",
			namespace: "ext-namespace",
			targetRef: MCPGatewayExtensionTargetReference{
				Name:      "my-gateway",
				Namespace: "gateway-system",
			},
			want: "my-gateway-istio.gateway-system.svc.cluster.local:8080",
		},
		{
			name:      "falls back to extension namespace when targetRef namespace empty",
			namespace: "team-a",
			targetRef: MCPGatewayExtensionTargetReference{
				Name: "my-gateway",
			},
			want: "my-gateway-istio.team-a.svc.cluster.local:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tt.namespace,
				},
				Spec: MCPGatewayExtensionSpec{
					TargetRef: tt.targetRef,
				},
			}
			if got := m.InternalHost(8080); got != tt.want {
				t.Errorf("InternalHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPGatewayExtension_PollInterval(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name:        "nil annotations returns empty string",
			annotations: nil,
			want:        "",
		},
		{
			name:        "empty annotations returns empty string",
			annotations: map[string]string{},
			want:        "",
		},
		{
			name: "annotation not present returns empty string",
			annotations: map[string]string{
				"other-annotation": "value",
			},
			want: "",
		},
		{
			name: "annotation present returns value",
			annotations: map[string]string{
				AnnotationPollInterval: "30",
			},
			want: "30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if got := m.PollInterval(); got != tt.want {
				t.Errorf("PollInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPRouteDisabled(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "no annotations returns false",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations map returns false",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "annotation set to true returns true",
			annotations: map[string]string{
				AnnotationDisableHTTPRoute: "true",
			},
			want: true,
		},
		{
			name: "annotation set to false returns false",
			annotations: map[string]string{
				AnnotationDisableHTTPRoute: "false",
			},
			want: false,
		},
		{
			name: "annotation set to empty string returns false",
			annotations: map[string]string{
				AnnotationDisableHTTPRoute: "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if got := m.HTTPRouteDisabled(); got != tt.want {
				t.Errorf("HTTPRouteDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPGatewayExtension_ListenerPort(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        uint32
	}{
		{
			name:        "nil annotations returns default port",
			annotations: nil,
			want:        DefaultListenerPort,
		},
		{
			name:        "empty annotations returns default port",
			annotations: map[string]string{},
			want:        DefaultListenerPort,
		},
		{
			name: "annotation not present returns default port",
			annotations: map[string]string{
				"other-annotation": "value",
			},
			want: DefaultListenerPort,
		},
		{
			name: "annotation present returns parsed value",
			annotations: map[string]string{
				AnnotationListenerPort: "8443",
			},
			want: 8443,
		},
		{
			name: "empty annotation value returns default port",
			annotations: map[string]string{
				AnnotationListenerPort: "",
			},
			want: DefaultListenerPort,
		},
		{
			name: "invalid annotation value returns default port",
			annotations: map[string]string{
				AnnotationListenerPort: "not-a-number",
			},
			want: DefaultListenerPort,
		},
		{
			name: "negative number returns default port",
			annotations: map[string]string{
				AnnotationListenerPort: "-1",
			},
			want: DefaultListenerPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if got := m.ListenerPort(); got != tt.want {
				t.Errorf("ListenerPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
