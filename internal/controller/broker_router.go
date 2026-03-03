package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	mcpv1alpha1 "github.com/Kuadrant/mcp-gateway/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// broker-router deployment constants
	brokerRouterName     = "mcp-gateway"
	gatewayHTTPRouteName = "mcp-gateway-route"

	// DefaultBrokerRouterImage is the default image for the broker-router deployment
	DefaultBrokerRouterImage = "ghcr.io/kuadrant/mcp-gateway:latest"

	// broker-router ports
	brokerHTTPPort   = 8080
	brokerGRPCPort   = 50051
	brokerConfigPort = 8181
)

// flags that can be changed directly on the deployment without triggering an update
var ignoredCommandFlags = []string{
	"--cache-connection-string",
	"--log-level",
	"--log-format",
	"--session-length",
}

func brokerRouterLabels() map[string]string {
	return map[string]string{
		labelAppName:   brokerRouterName,
		labelManagedBy: labelManagedByValue,
	}
}

func (r *MCPGatewayExtensionReconciler) buildBrokerRouterDeployment(mcpExt *mcpv1alpha1.MCPGatewayExtension, publicHost, internalHost string) *appsv1.Deployment {
	labels := brokerRouterLabels()
	replicas := int32(1)

	command := []string{"./mcp_gateway", fmt.Sprintf("--mcp-broker-public-address=0.0.0.0:%d", brokerHTTPPort),
		"--mcp-gateway-private-host=" + internalHost,
		"--mcp-gateway-config=/config/config.yaml"}
	// annotation takes precedence over reconciler default
	pollInterval := mcpExt.PollInterval()
	if pollInterval == "" {
		pollInterval = r.BrokerPollInterval
	}
	if pollInterval != "" {
		// the flag expects seconds as a plain number; parse duration strings like "60s" or "5m"
		if d, err := time.ParseDuration(pollInterval); err == nil {
			pollInterval = fmt.Sprintf("%d", int64(d.Seconds()))
		}
		command = append(command, "--mcp-check-interval="+pollInterval)
	}
	command = append(command, "--mcp-gateway-public-host="+publicHost)
	command = append(command, "--mcp-router-key="+routerKey(mcpExt))
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerRouterName,
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:           brokerRouterName,
					AutomountServiceAccountToken: ptr.To(false),
					Containers: []corev1.Container{
						{
							Name:            brokerRouterName,
							Image:           r.BrokerRouterImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         command,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: brokerHTTPPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "grpc",
									ContainerPort: brokerGRPCPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "config",
									ContainerPort: brokerConfigPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/config",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "mcp-gateway-config",
									DefaultMode: ptr.To(int32(420)), // 0644 octal
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *MCPGatewayExtensionReconciler) buildBrokerRouterServiceAccount(mcpExt *mcpv1alpha1.MCPGatewayExtension) *corev1.ServiceAccount {
	labels := brokerRouterLabels()
	automount := false

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerRouterName,
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		AutomountServiceAccountToken: &automount,
	}
}

func (r *MCPGatewayExtensionReconciler) buildBrokerRouterService(mcpExt *mcpv1alpha1.MCPGatewayExtension) *corev1.Service {
	labels := brokerRouterLabels()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerRouterName,
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				labelAppName: brokerRouterName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       brokerHTTPPort,
					TargetPort: intstr.FromInt(brokerHTTPPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       brokerGRPCPort,
					TargetPort: intstr.FromInt(brokerGRPCPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// routerKey generates a deterministic key for hair-pinning requests based on the extension's UID
func routerKey(mcpExt *mcpv1alpha1.MCPGatewayExtension) string {
	hash := sha256.Sum256([]byte(mcpExt.UID))
	return hex.EncodeToString(hash[:16])
}

// stripPort removes port suffix from a host string (e.g. "example.com:8001" -> "example.com")
func stripPort(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return h
}

// derivePublicHost determines the public host for the MCP Gateway.
// Priority: annotation override > listener hostname.
// For wildcard hostnames (*.example.com), we use mcp.example.com as the default subdomain.
// Any port suffix is stripped since HTTPRoute hostnames don't allow ports.
func derivePublicHost(listenerConfig *mcpv1alpha1.ListenerConfig, annotationOverride string) (string, error) {
	var hostname string
	// annotation takes precedence for backwards compatibility
	if annotationOverride != "" {
		hostname = stripPort(annotationOverride)
	} else if listenerConfig != nil && listenerConfig.Hostname != "" {
		hostname = listenerConfig.Hostname
		// handle wildcard hostnames: *.example.com -> mcp.example.com
		if strings.HasPrefix(hostname, "*.") {
			hostname = "mcp" + hostname[1:]
		}
	}
	if hostname == "" {
		return "", fmt.Errorf("unable to derive public host: no hostname available")
	}
	if _, err := url.ParseRequestURI("http://" + hostname); err != nil {
		return "", fmt.Errorf("invalid public host %q: %w", hostname, err)
	}
	return hostname, nil
}

func (r *MCPGatewayExtensionReconciler) reconcileBrokerRouter(ctx context.Context, mcpExt *mcpv1alpha1.MCPGatewayExtension, listenerConfig *mcpv1alpha1.ListenerConfig) (bool, error) {
	// derive values from listener config before building resources
	publicHost, err := derivePublicHost(listenerConfig, mcpExt.PublicHost())
	if err != nil {
		return false, newValidationError(mcpv1alpha1.ConditionReasonInvalid, err.Error())
	}
	internalHost := mcpExt.InternalHost(listenerConfig.Port)

	// reconcile service account (must exist before deployment)
	serviceAccount := r.buildBrokerRouterServiceAccount(mcpExt)
	if err := controllerutil.SetControllerReference(mcpExt, serviceAccount, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set controller reference on service account: %w", err)
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	err = r.Get(ctx, client.ObjectKeyFromObject(serviceAccount), existingServiceAccount)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("creating broker-router service account", "namespace", mcpExt.Namespace)
			if err := r.Create(ctx, serviceAccount); err != nil {
				return false, fmt.Errorf("failed to create service account: %w", err)
			}
		} else {
			return false, fmt.Errorf("failed to get service account: %w", err)
		}
	} else if needsUpdate, reason := serviceAccountNeedsUpdate(serviceAccount, existingServiceAccount); needsUpdate {
		r.log.Info("updating broker-router service account", "namespace", mcpExt.Namespace, "reason", reason)
		existingServiceAccount.AutomountServiceAccountToken = serviceAccount.AutomountServiceAccountToken
		if err := r.Update(ctx, existingServiceAccount); err != nil {
			return false, fmt.Errorf("failed to update service account: %w", err)
		}
	}

	// reconcile deployment
	deployment := r.buildBrokerRouterDeployment(mcpExt, publicHost, internalHost)
	if err := controllerutil.SetControllerReference(mcpExt, deployment, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set controller reference on deployment: %w", err)
	}

	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKeyFromObject(deployment), existingDeployment)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("creating broker-router deployment", "namespace", mcpExt.Namespace)
			if err := r.Create(ctx, deployment); err != nil {
				return false, fmt.Errorf("failed to create deployment: %w", err)
			}
			return false, nil // deployment just created, not ready yet
		}
		return false, fmt.Errorf("failed to get deployment: %w", err)
	}

	if needsUpdate, reason := deploymentNeedsUpdate(deployment, existingDeployment); needsUpdate {
		r.log.Info("updating broker-router deployment", "namespace", mcpExt.Namespace, "reason", reason)
		existingDeployment.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
		existingDeployment.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes
		if err := r.Update(ctx, existingDeployment); err != nil {
			return false, fmt.Errorf("failed to update deployment: %w", err)
		}
		return false, nil // deployment updated, requeue to get fresh status
	}

	// reconcile service
	service := r.buildBrokerRouterService(mcpExt)
	if err := controllerutil.SetControllerReference(mcpExt, service, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set controller reference on service: %w", err)
	}

	existingService := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKeyFromObject(service), existingService)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("creating broker-router service", "namespace", mcpExt.Namespace)
			if err := r.Create(ctx, service); err != nil {
				return false, fmt.Errorf("failed to create service: %w", err)
			}
		} else {
			return false, fmt.Errorf("failed to get service: %w", err)
		}
	} else if needsUpdate, reason := serviceNeedsUpdate(service, existingService); needsUpdate {
		r.log.Info("updating broker-router service", "namespace", mcpExt.Namespace, "reason", reason)
		existingService.Spec.Ports = service.Spec.Ports
		existingService.Spec.Selector = service.Spec.Selector
		if err := r.Update(ctx, existingService); err != nil {
			return false, fmt.Errorf("failed to update service: %w", err)
		}
	}

	// reconcile gateway HTTPRoute (unless disabled by annotation)
	if !mcpExt.HTTPRouteDisabled() {
		httpRoute := r.buildGatewayHTTPRoute(mcpExt, publicHost)
		if err := controllerutil.SetControllerReference(mcpExt, httpRoute, r.Scheme); err != nil {
			return false, fmt.Errorf("failed to set controller reference on httproute: %w", err)
		}

		existingHTTPRoute := &gatewayv1.HTTPRoute{}
		err = r.Get(ctx, client.ObjectKeyFromObject(httpRoute), existingHTTPRoute)
		if err != nil {
			if apierrors.IsNotFound(err) {
				r.log.Info("creating gateway httproute", "namespace", mcpExt.Namespace)
				if err := r.Create(ctx, httpRoute); err != nil {
					return false, fmt.Errorf("failed to create httproute: %w", err)
				}
			} else {
				return false, fmt.Errorf("failed to get httproute: %w", err)
			}
		} else if needsUpdate, reason := httpRouteNeedsUpdate(httpRoute, existingHTTPRoute); needsUpdate {
			r.log.Info("updating gateway httproute", "namespace", mcpExt.Namespace, "reason", reason)
			existingHTTPRoute.Spec.ParentRefs = httpRoute.Spec.ParentRefs
			existingHTTPRoute.Spec.Hostnames = httpRoute.Spec.Hostnames
			existingHTTPRoute.Spec.Rules = httpRoute.Spec.Rules
			if err := r.Update(ctx, existingHTTPRoute); err != nil {
				return false, fmt.Errorf("failed to update httproute: %w", err)
			}
		}
	}

	// check deployment readiness
	deploymentReady := existingDeployment.Status.ReadyReplicas > 0 &&
		existingDeployment.Status.ReadyReplicas == existingDeployment.Status.Replicas

	return deploymentReady, nil
}

// serviceNeedsUpdate checks if the service needs to be updated
// returns (needsUpdate, reason) where reason describes what changed
func serviceNeedsUpdate(desired, existing *corev1.Service) (bool, string) {
	if !equality.Semantic.DeepEqual(desired.Spec.Ports, existing.Spec.Ports) {
		return true, fmt.Sprintf("ports changed: %+v -> %+v", existing.Spec.Ports, desired.Spec.Ports)
	}
	if !equality.Semantic.DeepEqual(desired.Spec.Selector, existing.Spec.Selector) {
		return true, fmt.Sprintf("selector changed: %v -> %v", existing.Spec.Selector, desired.Spec.Selector)
	}
	return false, ""
}

// serviceAccountNeedsUpdate checks if the service account needs to be updated
// returns (needsUpdate, reason) where reason describes what changed
func serviceAccountNeedsUpdate(desired, existing *corev1.ServiceAccount) (bool, string) {
	if !equality.Semantic.DeepEqual(desired.AutomountServiceAccountToken, existing.AutomountServiceAccountToken) {
		return true, fmt.Sprintf("automountServiceAccountToken changed: %v -> %v",
			existing.AutomountServiceAccountToken, desired.AutomountServiceAccountToken)
	}
	return false, ""
}

// deploymentNeedsUpdate checks if the deployment needs to be updated
// returns (needsUpdate, reason) where reason describes what changed
func deploymentNeedsUpdate(desired, existing *appsv1.Deployment) (bool, string) {
	if len(desired.Spec.Template.Spec.Containers) == 0 || len(existing.Spec.Template.Spec.Containers) == 0 {
		return false, ""
	}
	desiredContainer := desired.Spec.Template.Spec.Containers[0]
	existingContainer := existing.Spec.Template.Spec.Containers[0]

	if desiredContainer.Image != existingContainer.Image {
		return true, fmt.Sprintf("image changed: %q -> %q", existingContainer.Image, desiredContainer.Image)
	}
	// filter out flags that can be changed directly on the deployment
	desiredCmd := filterIgnoredFlags(desiredContainer.Command)
	existingCmd := filterIgnoredFlags(existingContainer.Command)
	if !equality.Semantic.DeepEqual(desiredCmd, existingCmd) {
		return true, fmt.Sprintf("command changed: %v -> %v", existingCmd, desiredCmd)
	}
	if !equality.Semantic.DeepEqual(desiredContainer.Ports, existingContainer.Ports) {
		return true, fmt.Sprintf("ports changed: %+v -> %+v", existingContainer.Ports, desiredContainer.Ports)
	}
	if !equality.Semantic.DeepEqual(desiredContainer.VolumeMounts, existingContainer.VolumeMounts) {
		return true, fmt.Sprintf("volumeMounts changed: %+v -> %+v", existingContainer.VolumeMounts, desiredContainer.VolumeMounts)
	}
	if !equality.Semantic.DeepEqual(desired.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
		return true, fmt.Sprintf("volumes changed: %+v -> %+v", existing.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes)
	}
	return false, ""
}

func filterIgnoredFlags(command []string) []string {
	filtered := make([]string, 0, len(command))
	for _, arg := range command {
		ignore := false
		for _, flag := range ignoredCommandFlags {
			if strings.HasPrefix(arg, flag) {
				ignore = true
				break
			}
		}
		if !ignore {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func (r *MCPGatewayExtensionReconciler) buildGatewayHTTPRoute(mcpExt *mcpv1alpha1.MCPGatewayExtension, publicHost string) *gatewayv1.HTTPRoute {
	labels := brokerRouterLabels()
	pathType := gatewayv1.PathMatchPathPrefix
	pathValue := "/mcp"
	port := gatewayv1.PortNumber(brokerHTTPPort)
	gatewayNamespace := gatewayv1.Namespace(mcpExt.Spec.TargetRef.Namespace)
	sectionName := gatewayv1.SectionName(mcpExt.Spec.TargetRef.SectionName)

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayHTTPRouteName,
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Group:       ptr.To(gatewayv1.Group("gateway.networking.k8s.io")),
						Kind:        ptr.To(gatewayv1.Kind("Gateway")),
						Name:        gatewayv1.ObjectName(mcpExt.Spec.TargetRef.Name),
						Namespace:   &gatewayNamespace,
						SectionName: &sectionName,
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{
				gatewayv1.Hostname(publicHost),
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathType,
								Value: &pathValue,
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(brokerRouterName),
									Port: &port,
								},
							},
						},
					},
				},
			},
		},
	}
}

// httpRouteNeedsUpdate checks if the HTTPRoute needs to be updated
func httpRouteNeedsUpdate(desired, existing *gatewayv1.HTTPRoute) (bool, string) {
	if !equality.Semantic.DeepEqual(desired.Spec.ParentRefs, existing.Spec.ParentRefs) {
		return true, "parentRefs changed"
	}
	if !equality.Semantic.DeepEqual(desired.Spec.Hostnames, existing.Spec.Hostnames) {
		return true, fmt.Sprintf("hostnames changed: %v -> %v", existing.Spec.Hostnames, desired.Spec.Hostnames)
	}
	if !equality.Semantic.DeepEqual(desired.Spec.Rules, existing.Spec.Rules) {
		return true, "rules changed"
	}
	return false, ""
}
