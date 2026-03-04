package credentials

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadCredentials loads VPC API credentials.
// If CSI_MOUNT_PATH is set, tries the CSI-mounted file first.
// Falls back to reading from a Kubernetes Secret.
func LoadCredentials(ctx context.Context) (string, error) {
	if csiPath := os.Getenv("CSI_MOUNT_PATH"); csiPath != "" {
		apiKey, err := loadFromFile(ctx)
		if err == nil {
			return apiKey, nil
		}
		slog.WarnContext(ctx, "CSI credentials unavailable, falling back to Secret", "error", err)
	}

	return loadFromSecret(ctx)
}

// loadFromFile reads the API key from a CSI-mounted file
func loadFromFile(ctx context.Context) (string, error) {
	csiPath := os.Getenv("CSI_MOUNT_PATH")
	if csiPath == "" {
		csiPath = "/etc/vpc-credentials"
	}

	filePath := csiPath + "/apikey"
	slog.InfoContext(ctx, "loading credentials from file", "path", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read API key file: %w", err)
	}

	apiKey := strings.TrimSpace(string(data))
	if apiKey == "" {
		return "", fmt.Errorf("API key file is empty")
	}

	return apiKey, nil
}

// loadFromSecret reads the API key from a Kubernetes Secret
func loadFromSecret(ctx context.Context) (string, error) {
	secretName := os.Getenv("BFF_SECRET_NAME")
	if secretName == "" {
		secretName = "vpc-api-credentials"
	}

	namespace := os.Getenv("BFF_SECRET_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	slog.InfoContext(ctx, "loading credentials from secret",
		"secret", secretName, "namespace", namespace)

	// Create Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for development
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home dir: %w", err)
			}
			kubeconfig = home + "/.kube/config"
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return "", fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create K8s client: %w", err)
	}

	// Fetch the secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Extract API key from secret data (try common key names)
	apiKeyData, exists := secret.Data["apikey"]
	if !exists {
		apiKeyData, exists = secret.Data["IBMCLOUD_API_KEY"]
	}
	if !exists {
		return "", fmt.Errorf("apikey/IBMCLOUD_API_KEY not found in secret %s/%s", namespace, secretName)
	}

	apiKey := strings.TrimSpace(string(apiKeyData))
	if apiKey == "" {
		return "", fmt.Errorf("apikey in secret is empty")
	}

	return apiKey, nil
}
