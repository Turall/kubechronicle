package admin

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// NewKubernetesClient creates a Kubernetes client.
// It tries in-cluster config first, then falls back to kubeconfig file.
func NewKubernetesClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first (when running in Kubernetes)
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig file (for local development)
		klog.V(2).Infof("In-cluster config not available, trying kubeconfig: %v", err)
		kubeconfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config and failed to load kubeconfig: %w", err)
		}
		config, err = clientcmd.NewDefaultClientConfig(*kubeconfig, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}
