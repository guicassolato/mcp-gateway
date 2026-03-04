# MCPGatewayExtension Spec Fields Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace annotation-based configuration on MCPGatewayExtension with proper spec fields, following Kubernetes API conventions.

**Architecture:** Add four optional spec fields (`publicHost`, `privateHost`, `backendPingIntervalSeconds`, `httpRouteManagement`) to `MCPGatewayExtensionSpec`. Remove the four `kuadrant.io/alpha-*` annotation constants and their helper methods. Update the reconciler, tests, Helm chart, and docs to use spec fields instead of annotations.

**Tech Stack:** Go, controller-gen, Kubebuilder markers, Helm templates

---

### Task 1: Add spec fields to MCPGatewayExtensionSpec

**Files:**
- Modify: `api/v1alpha1/mcpgatewayextension_types.go:37-42`
- Test: `api/v1alpha1/mcpgatewayextension_types_test.go`

**Step 1: Write the failing tests**

Replace the existing annotation-based tests in `api/v1alpha1/mcpgatewayextension_types_test.go`. The new tests verify spec field access and the updated `InternalHost` method (which now checks `privateHost` override).

Tests to write:

```go
func TestMCPGatewayExtension_InternalHost(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		targetRef   MCPGatewayExtensionTargetReference
		privateHost string
		want        string
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
		{
			name:      "privateHost overrides computed value",
			namespace: "team-a",
			targetRef: MCPGatewayExtensionTargetReference{
				Name:      "my-gateway",
				Namespace: "gateway-system",
			},
			privateHost: "custom-gateway.custom-ns.svc.cluster.local:9090",
			want:        "custom-gateway.custom-ns.svc.cluster.local:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tt.namespace,
				},
				Spec: MCPGatewayExtensionSpec{
					TargetRef:   tt.targetRef,
					PrivateHost: tt.privateHost,
				},
			}
			if got := m.InternalHost(8080); got != tt.want {
				t.Errorf("InternalHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPGatewayExtension_HTTPRouteManagementDisabled(t *testing.T) {
	tests := []struct {
		name               string
		httpRouteManagement HTTPRouteManagementPolicy
		want               bool
	}{
		{
			name:               "empty defaults to not disabled",
			httpRouteManagement: "",
			want:               false,
		},
		{
			name:               "Auto is not disabled",
			httpRouteManagement: HTTPRouteManagementAuto,
			want:               false,
		},
		{
			name:               "Disabled is disabled",
			httpRouteManagement: HTTPRouteManagementDisabled,
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPGatewayExtension{
				Spec: MCPGatewayExtensionSpec{
					HTTPRouteManagement: tt.httpRouteManagement,
				},
			}
			if got := m.HTTPRouteDisabled(); got != tt.want {
				t.Errorf("HTTPRouteDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

Remove the old `TestMCPGatewayExtension_PublicHost`, `TestMCPGatewayExtension_PollInterval`, `TestHTTPRouteDisabled`, and `TestMCPGatewayExtension_ListenerPort` tests entirely.

**Step 2: Run tests to verify they fail**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go test ./api/v1alpha1/ -run "TestMCPGatewayExtension_InternalHost|TestMCPGatewayExtension_HTTPRouteManagementDisabled" -v`
Expected: Compilation failure — `PrivateHost`, `HTTPRouteManagementPolicy`, etc. not defined yet.

**Step 3: Update the types**

In `api/v1alpha1/mcpgatewayextension_types.go`:

1. Remove the annotation constants: `AnnotationPublicHost`, `AnnotationPollInterval`, `AnnotationListenerPort`, `AnnotationDisableHTTPRoute`, `DefaultListenerPort`.

2. Add the `HTTPRouteManagementPolicy` type and constants:

```go
// HTTPRouteManagementPolicy defines how the operator manages the gateway HTTPRoute
// +kubebuilder:validation:Enum=Auto;Disabled
type HTTPRouteManagementPolicy string

const (
	// HTTPRouteManagementAuto means the operator creates and manages the HTTPRoute
	HTTPRouteManagementAuto HTTPRouteManagementPolicy = "Auto"
	// HTTPRouteManagementDisabled means the operator does not create an HTTPRoute
	HTTPRouteManagementDisabled HTTPRouteManagementPolicy = "Disabled"
)
```

3. Update `MCPGatewayExtensionSpec`:

```go
type MCPGatewayExtensionSpec struct {
	// TargetRef specifies the Gateway to extend with MCP protocol support.
	TargetRef MCPGatewayExtensionTargetReference `json:"targetRef"`

	// PublicHost overrides the public host derived from the listener hostname.
	// Use when the listener has a wildcard and you need a specific host.
	// +optional
	PublicHost string `json:"publicHost,omitempty"`

	// PrivateHost overrides the internal host used for hair-pinning requests
	// back through the gateway. Defaults to <gateway>-istio.<ns>.svc.cluster.local:<port>.
	// +optional
	PrivateHost string `json:"privateHost,omitempty"`

	// BackendPingIntervalSeconds specifies how often the broker pings upstream MCP servers.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	BackendPingIntervalSeconds *int32 `json:"backendPingIntervalSeconds,omitempty"`

	// HTTPRouteManagement controls whether the operator manages the gateway HTTPRoute.
	// Auto: creates and manages the HTTPRoute (default).
	// Disabled: does not create an HTTPRoute.
	// +optional
	// +kubebuilder:default=Auto
	HTTPRouteManagement HTTPRouteManagementPolicy `json:"httpRouteManagement,omitempty"`
}
```

Note: `BackendPingIntervalSeconds` uses `*int32` (pointer) so zero-value is distinguishable from unset, per K8s conventions for optional numeric fields.

4. Replace the annotation-based helper methods:

Remove `PublicHost()`, `PollInterval()`, `ListenerPort()` methods entirely.

Update `HTTPRouteDisabled()`:

```go
// HTTPRouteDisabled returns true if httpRouteManagement is set to Disabled
func (m *MCPGatewayExtension) HTTPRouteDisabled() bool {
	return m.Spec.HTTPRouteManagement == HTTPRouteManagementDisabled
}
```

Update `InternalHost()` to check `PrivateHost` first:

```go
func (m *MCPGatewayExtension) InternalHost(port uint32) string {
	if m.Spec.PrivateHost != "" {
		return m.Spec.PrivateHost
	}
	gatewayNamespace := m.Spec.TargetRef.Namespace
	if gatewayNamespace == "" {
		gatewayNamespace = m.Namespace
	}
	return fmt.Sprintf(m.Spec.TargetRef.Name+"-istio."+gatewayNamespace+".svc.cluster.local:%v", port)
}
```

Remove unused `strconv` import.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go test ./api/v1alpha1/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add api/v1alpha1/mcpgatewayextension_types.go api/v1alpha1/mcpgatewayextension_types_test.go
git commit -m "feat: add spec fields to MCPGatewayExtensionSpec, remove annotations"
```

---

### Task 2: Update the reconciler to use spec fields

**Files:**
- Modify: `internal/controller/broker_router.go:53-73, 200-226, 308-309`
- Test: `internal/controller/deployment_test.go`

**Step 1: Update deployment_test.go**

Update `TestBuildBrokerRouterDeployment_PollInterval` to use spec fields instead of annotations. The test cases should set `BackendPingIntervalSeconds` on the spec:

```go
{
	name:               "poll interval from spec",
	backendPingInterval: ptr.To(int32(30)),
	reconcilerInterval: "",
	wantFlag:           "--mcp-check-interval=30",
},
{
	name:               "poll interval from reconciler when spec not set",
	backendPingInterval: nil,
	reconcilerInterval: "60s",
	wantFlag:           "--mcp-check-interval=60",
},
{
	name:               "no poll interval flag when both empty",
	backendPingInterval: nil,
	reconcilerInterval: "",
	wantAbsent:         true,
},
{
	name:               "spec takes precedence over reconciler",
	backendPingInterval: ptr.To(int32(15)),
	reconcilerInterval: "60s",
	wantFlag:           "--mcp-check-interval=15",
},
```

Set the field on the MCPGatewayExtension spec instead of annotations:
```go
mcpExt := &mcpv1alpha1.MCPGatewayExtension{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-ext",
		Namespace: "test-ns",
	},
	Spec: mcpv1alpha1.MCPGatewayExtensionSpec{
		BackendPingIntervalSeconds: tt.backendPingInterval,
		TargetRef: mcpv1alpha1.MCPGatewayExtensionTargetReference{...},
	},
}
```

Also update `TestDerivePublicHost` to pass a string directly (the function signature changes from taking `annotationOverride` to `specOverride` — same type, different semantics).

**Step 2: Run tests to verify they fail**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go test ./internal/controller/ -run "TestBuildBrokerRouterDeployment_PollInterval|TestDerivePublicHost" -v`
Expected: Compilation failure or test failure.

**Step 3: Update broker_router.go**

1. Update `buildBrokerRouterDeployment` (lines 60-71): Replace annotation-based poll interval with spec field:

```go
// spec takes precedence over reconciler default
var pollInterval string
if mcpExt.Spec.BackendPingIntervalSeconds != nil {
	pollInterval = fmt.Sprintf("%d", *mcpExt.Spec.BackendPingIntervalSeconds)
} else if r.BrokerPollInterval != "" {
	pollInterval = r.BrokerPollInterval
	if d, err := time.ParseDuration(pollInterval); err == nil {
		pollInterval = fmt.Sprintf("%d", int64(d.Seconds()))
	}
}
if pollInterval != "" {
	command = append(command, "--mcp-check-interval="+pollInterval)
}
```

2. Update `derivePublicHost` (line 204): Rename parameter for clarity:

```go
func derivePublicHost(listenerConfig *mcpv1alpha1.ListenerConfig, specOverride string) string {
	if specOverride != "" {
		return stripPort(specOverride)
	}
	// ... rest unchanged
}
```

3. Update `reconcileBrokerRouter` (line 222): Read from spec instead of annotation:

```go
publicHost := derivePublicHost(listenerConfig, mcpExt.Spec.PublicHost)
```

4. Update comment on line 308 and the check on line 309 — the method `HTTPRouteDisabled()` signature hasn't changed, just its internal implementation. Update the comment:

```go
// reconcile gateway HTTPRoute (unless disabled by spec)
```

5. Update the error message on line 224 to reference spec field instead of annotation:

```go
return false, fmt.Errorf("unable to derive public host: listener %q has no hostname and spec.publicHost is not set", listenerConfig.Name)
```

**Step 4: Update mcpgatewayextension_controller.go**

Find the reference to `mcpExt.PublicHost()` and `AnnotationPublicHost` at line 259-262 and update:

```go
if listenerConfig.Hostname == "" && mcpExt.Spec.PublicHost == "" {
	// ... update error message to reference spec.publicHost
}
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go test ./internal/controller/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/controller/broker_router.go internal/controller/deployment_test.go internal/controller/mcpgatewayextension_controller.go
git commit -m "feat: update reconciler to read from spec fields instead of annotations"
```

---

### Task 3: Update controller integration tests

**Files:**
- Modify: `internal/controller/mcpgatewayextension_controller_test.go:1053-1071`

**Step 1: Update the HTTPRoute disabled test**

Change from annotation-based to spec-field-based:

```go
It("should not create HTTPRoute when disabled by spec", func() {
	ext := &mcpv1alpha1.MCPGatewayExtension{}
	Expect(testK8sClient.Get(ctx, mcpExtNamespacedName, ext)).To(Succeed())
	ext.Spec.HTTPRouteManagement = mcpv1alpha1.HTTPRouteManagementDisabled
	Expect(testK8sClient.Update(ctx, ext)).To(Succeed())

	reconciler := newTestReconciler()
	waitForCacheSync(ctx, mcpExtNamespacedName)

	Eventually(func(g Gomega) {
		cached := &mcpv1alpha1.MCPGatewayExtension{}
		g.Expect(testIndexedClient.Get(ctx, mcpExtNamespacedName, cached)).To(Succeed())
		g.Expect(cached.Spec.HTTPRouteManagement).To(Equal(mcpv1alpha1.HTTPRouteManagementDisabled))
	}, testTimeout, testRetryInterval).Should(Succeed())
	// ... rest of the test unchanged
```

**Step 2: Run the integration tests**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go test ./internal/controller/ -v -run "HTTPRoute"`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/controller/mcpgatewayextension_controller_test.go
git commit -m "test: update integration tests to use spec fields"
```

---

### Task 4: Regenerate CRD and update Helm chart

**Files:**
- Modify: `config/crd/mcp.kagenti.com_mcpgatewayextensions.yaml` (generated)
- Modify: `charts/mcp-gateway/templates/mcpgatewayextension.yaml`
- Modify: `charts/mcp-gateway/values.yaml`
- Modify: `charts/README.md`
- Modify: `config/mcp-gateway/base/mcpgatewayextension.yaml`

**Step 1: Regenerate CRD**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && make generate manifests`
Expected: CRD YAML updated with new spec fields.

**Step 2: Update Helm template**

Replace annotations with spec fields in `charts/mcp-gateway/templates/mcpgatewayextension.yaml`:

```yaml
{{- if .Values.mcpGatewayExtension.create }}
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPGatewayExtension
metadata:
  name: {{ include "mcp-gateway.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: {{ .Values.mcpGatewayExtension.gatewayRef.name }}
    {{- if .Values.mcpGatewayExtension.gatewayRef.namespace }}
    namespace: {{ .Values.mcpGatewayExtension.gatewayRef.namespace }}
    {{- end }}
    sectionName: {{ .Values.mcpGatewayExtension.gatewayRef.sectionName }}
  {{- if .Values.gateway.publicHost }}
  publicHost: {{ .Values.gateway.publicHost | quote }}
  {{- end }}
  {{- if .Values.broker.pollInterval }}
  backendPingIntervalSeconds: {{ .Values.broker.pollInterval }}
  {{- end }}
{{- end }}
```

Note: `broker.pollInterval` in values.yaml should change from string `"60"` to integer `60`.

**Step 3: Update values.yaml**

Change `broker.pollInterval` from string to integer:
```yaml
broker:
  # How often (in seconds) the broker pings upstream MCP servers
  pollInterval: 60
```

**Step 4: Update config/mcp-gateway/base/mcpgatewayextension.yaml**

Move any annotation-based config to spec fields.

**Step 5: Update charts/README.md**

Update the parameter documentation to reflect spec fields instead of annotations.

**Step 6: Run the sync-chart skill if available, then lint**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && make lint`
Expected: PASS

**Step 7: Commit**

```bash
git add config/crd/ charts/ config/mcp-gateway/
git commit -m "feat: update CRD, Helm chart, and base manifests for spec fields"
```

---

### Task 5: Update E2E test builders

**Files:**
- Modify: `tests/e2e/builders.go:563-593`

**Step 1: Update the Build method**

Change the `Build()` method to set spec fields instead of annotations:

```go
func (s *MCPGatewayExtensionSetup) Build() *MCPGatewayExtensionSetup {
	spec := mcpv1alpha1.MCPGatewayExtensionSpec{
		TargetRef: mcpv1alpha1.MCPGatewayExtensionTargetReference{
			Group:       "gateway.networking.k8s.io",
			Kind:        "Gateway",
			Name:        s.gatewayName,
			Namespace:   s.gatewayNamespace,
			SectionName: s.sectionName,
		},
	}
	if s.publicHost != "" {
		spec.PublicHost = s.publicHost
	}
	if s.pollInterval != "" {
		interval, _ := strconv.Atoi(s.pollInterval)
		spec.BackendPingIntervalSeconds = ptr.To(int32(interval))
	}
	if s.disableHTTPRoute {
		spec.HTTPRouteManagement = mcpv1alpha1.HTTPRouteManagementDisabled
	}

	s.extension = &mcpv1alpha1.MCPGatewayExtension{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: spec,
	}
	// ... rest unchanged
```

**Step 2: Run E2E tests (compile check)**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && go build ./tests/e2e/...`
Expected: Compiles successfully.

**Step 3: Commit**

```bash
git add tests/e2e/builders.go
git commit -m "test: update e2e builders to use spec fields"
```

---

### Task 6: Update documentation

**Files:**
- Modify: `docs/guides/migrating-mcpgatewayextension.md`
- Modify: `docs/guides/how-to-install-and-configure.md`
- Modify: `docs/guides/configure-mcp-gateway-listener-and-router.md`
- Modify: `CLAUDE.md`

**Step 1: Update migration guide**

Add a section explaining annotation-to-spec-field migration with before/after examples.

**Step 2: Update installation and configuration guides**

Replace annotation references with spec field usage.

**Step 3: Update CLAUDE.md**

Replace references to annotations with spec fields in the MCPGatewayExtension and authentication sections.

**Step 4: Commit**

```bash
git add docs/ CLAUDE.md
git commit -m "docs: update guides for spec field migration"
```

---

### Task 7: Run full lint and test suite

**Step 1: Run linter**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && make lint`
Expected: PASS

**Step 2: Run unit tests**

Run: `cd /Users/maleck13/projects/src/github.com/kuadrant/mcp-gateway && make test-unit`
Expected: PASS

**Step 3: Verify no remaining annotation references**

Run: `grep -r "alpha-gateway-public-host\|alpha-gateway-poll-interval\|alpha-gateway-listener-port\|alpha-disable-httproute\|AnnotationPublicHost\|AnnotationPollInterval\|AnnotationListenerPort\|AnnotationDisableHTTPRoute" --include="*.go" --include="*.yaml" .`
Expected: No matches (except possibly in design doc change log or migration guide as historical references).
