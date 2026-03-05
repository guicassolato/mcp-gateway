package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mcpv1alpha1 "github.com/Kuadrant/mcp-gateway/api/v1alpha1"
)

// buildTrustedHeadersSecrets generates an ECDSA key pair and returns public and private key secrets
func buildTrustedHeadersSecrets(mcpExt *mcpv1alpha1.MCPGatewayExtension) (*corev1.Secret, *corev1.Secret, error) {
	pubPEM, privPEM, err := generateECDSAKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	labels := map[string]string{
		labelManagedBy: labelManagedByValue,
	}

	pubSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcpExt.Spec.TrustedHeadersKey.SecretName,
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			"key": pubPEM,
		},
	}

	privSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcpExt.Spec.TrustedHeadersKey.SecretName + "-private",
			Namespace: mcpExt.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			"key": privPEM,
		},
	}

	return pubSecret, privSecret, nil
}

// validateTrustedHeadersSecret checks that a secret has the required "key" data entry
func validateTrustedHeadersSecret(secret *corev1.Secret, secretName string) *validationError {
	if secret.Data == nil {
		return newValidationError(mcpv1alpha1.ConditionReasonSecretInvalid,
			fmt.Sprintf("secret %s has no data", secretName))
	}
	if _, ok := secret.Data["key"]; !ok {
		return newValidationError(mcpv1alpha1.ConditionReasonSecretInvalid,
			fmt.Sprintf("secret %s is missing required data entry \"key\"", secretName))
	}
	return nil
}

func (r *MCPGatewayExtensionReconciler) reconcileTrustedHeaders(ctx context.Context, mcpExt *mcpv1alpha1.MCPGatewayExtension) error {
	if mcpExt.Spec.TrustedHeadersKey == nil {
		return nil
	}

	if mcpExt.Spec.TrustedHeadersKey.Generate == mcpv1alpha1.KeyGenerationEnabled {
		if err := r.reconcileGeneratedTrustedHeaders(ctx, mcpExt); err != nil {
			return err
		}
		return r.setTrustedHeadersCondition(ctx, mcpExt, metav1.ConditionTrue,
			mcpv1alpha1.ConditionReasonTrustedHeadersConfigured, "trusted headers key pair generated")
	}

	// BYO secret: validate it exists and has the required key
	secretName := mcpExt.Spec.TrustedHeadersKey.SecretName
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: mcpExt.Namespace}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setTrustedHeadersCondition(ctx, mcpExt, metav1.ConditionFalse,
				mcpv1alpha1.ConditionReasonSecretNotFound,
				fmt.Sprintf("secret %s not found in namespace %s", secretName, mcpExt.Namespace))
		}
		return fmt.Errorf("failed to get trusted headers secret: %w", err)
	}

	if valErr := validateTrustedHeadersSecret(secret, secretName); valErr != nil {
		return r.setTrustedHeadersCondition(ctx, mcpExt, metav1.ConditionFalse,
			valErr.reason, valErr.message)
	}

	return r.setTrustedHeadersCondition(ctx, mcpExt, metav1.ConditionTrue,
		mcpv1alpha1.ConditionReasonTrustedHeadersConfigured, "trusted headers secret validated")
}

func (r *MCPGatewayExtensionReconciler) reconcileGeneratedTrustedHeaders(ctx context.Context, mcpExt *mcpv1alpha1.MCPGatewayExtension) error {
	secretName := mcpExt.Spec.TrustedHeadersKey.SecretName
	existing := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: mcpExt.Namespace}, existing)
	if err == nil {
		// public key secret already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check trusted headers secret: %w", err)
	}

	pubSecret, privSecret, err := buildTrustedHeadersSecrets(mcpExt)
	if err != nil {
		if setErr := r.setTrustedHeadersCondition(ctx, mcpExt, metav1.ConditionFalse,
			mcpv1alpha1.ConditionReasonKeyGenerationFailed, err.Error()); setErr != nil {
			return setErr
		}
		return err
	}

	if err := controllerutil.SetControllerReference(mcpExt, pubSecret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on public key secret: %w", err)
	}
	if err := controllerutil.SetControllerReference(mcpExt, privSecret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on private key secret: %w", err)
	}

	r.log.Info("creating trusted headers key pair", "public", pubSecret.Name, "private", privSecret.Name)
	if err := r.Create(ctx, pubSecret); err != nil {
		return fmt.Errorf("failed to create public key secret: %w", err)
	}
	if err := r.Create(ctx, privSecret); err != nil {
		return fmt.Errorf("failed to create private key secret: %w", err)
	}

	return nil
}

func (r *MCPGatewayExtensionReconciler) setTrustedHeadersCondition(_ context.Context, mcpExt *mcpv1alpha1.MCPGatewayExtension, status metav1.ConditionStatus, reason, message string) error {
	meta.SetStatusCondition(&mcpExt.Status.Conditions, metav1.Condition{
		Type:               mcpv1alpha1.ConditionTypeTrustedHeadersReady,
		Status:             status,
		ObservedGeneration: mcpExt.Generation,
		Reason:             reason,
		Message:            message,
	})
	return nil
}
