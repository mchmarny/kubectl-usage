// Package k8s provides Kubernetes client management and configuration.
// This package encapsulates all Kubernetes-specific client setup and configuration,
// following cloud-native best practices for API client management.
package k8s

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// ClientManager manages Kubernetes API clients with proper configuration.
// This design follows the dependency injection pattern common in distributed systems
// and encapsulates client lifecycle management.
type ClientManager struct {
	config  *rest.Config
	core    *kubernetes.Clientset
	metrics *metricsv.Clientset
}

// NewClientManager creates a new Kubernetes client manager with production-ready defaults.
// This function implements the factory pattern and handles the complex client configuration
// logic required for reliable operation in various Kubernetes environments.
//
// The configuration follows Kubernetes client-go best practices documented at:
// https://pkg.go.dev/k8s.io/client-go/rest#Config
func NewClientManager() (*ClientManager, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Apply production-ready defaults
	configureClientDefaults(config)

	core, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create core client: %w", err)
	}

	metrics, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	return &ClientManager{
		config:  config,
		core:    core,
		metrics: metrics,
	}, nil
}

// CoreClient returns the core Kubernetes API client.
func (cm *ClientManager) CoreClient() *kubernetes.Clientset {
	return cm.core
}

// MetricsClient returns the metrics API client.
func (cm *ClientManager) MetricsClient() *metricsv.Clientset {
	return cm.metrics
}

// Config returns the underlying REST config.
func (cm *ClientManager) Config() *rest.Config {
	return cm.config
}

// loadConfig attempts to load Kubernetes configuration using the standard precedence:
// 1. kubeconfig file (standard kubectl configuration)
// 2. in-cluster configuration (when running inside a pod)
//
// This follows the same precedence as kubectl and other Kubernetes tools.
// Reference: https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/
func loadConfig() (*rest.Config, error) {
	// Try standard kubeconfig chain (works for kubectl plugins)
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err == nil {
		return config, nil
	}

	// Fallback to in-cluster configuration (if running inside a pod)
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot load kubeconfig: %w", err)
	}
	return config, nil
}

// configureClientDefaults sets production-ready defaults for Kubernetes clients.
// These values are optimized for CLI tools that need responsive UX while being considerate
// of API server resources in distributed environments.
//
// Configuration follows Kubernetes API machinery best practices:
// https://kubernetes.io/docs/reference/config-api/apiserver-config.v1alpha1/
func configureClientDefaults(config *rest.Config) {
	// QPS and Burst control client-side rate limiting to the API server
	// For CLI tools, these should be moderate to avoid overwhelming the API server
	// while still providing responsive UX
	config.QPS = 50.0  // Allow up to 50 requests per second
	config.Burst = 100 // Allow bursts up to 100 requests

	// Timeout controls how long to wait for individual API calls
	// 30s is reasonable for CLI tools - long enough for most operations but not infinite
	config.Timeout = 30 * time.Second

	// UserAgent helps with debugging and monitoring in distributed environments
	// It allows cluster administrators to identify traffic from this tool
	config.UserAgent = "kusage/1.0"
}
