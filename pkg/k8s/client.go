// Package k8s provides Kubernetes client management and configuration.
// This package encapsulates all Kubernetes-specific client setup and configuration,
// following cloud-native best practices for API client management.
package k8s

import (
	"fmt"
	"net/http"
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
// These values are optimized for large-scale cluster operations while being considerate
// of API server resources in distributed environments.
func configureClientDefaults(config *rest.Config) {
	// QPS and Burst control client-side rate limiting to the API server
	// For large-scale operations, these values are significantly higher than default
	// Reference: https://kubernetes.io/docs/concepts/cluster-administration/system-traces/
	config.QPS = 300.0 // Allow up to 300 requests per second for large clusters
	config.Burst = 600 // Allow bursts up to 600 requests for pagination efficiency

	// Timeout controls how long to wait for individual API calls
	// Increased for large result sets that may take longer to process
	config.Timeout = 60 * time.Second

	// UserAgent helps with debugging and monitoring in distributed environments
	// It allows cluster administrators to identify traffic from this tool
	config.UserAgent = "kusage/1.0"

	// Configure connection pool settings for better performance
	// Use the WrapTransport field to customize the underlying transport
	// This approach is compatible with TLS and authentication handling
	if config.WrapTransport == nil {
		config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			// Only wrap if we get the default transport
			if transport, ok := rt.(*http.Transport); ok {
				// Clone the transport to avoid modifying the original
				newTransport := transport.Clone()
				// Optimize for large-scale operations
				newTransport.MaxIdleConns = 100
				newTransport.MaxIdleConnsPerHost = 20
				newTransport.IdleConnTimeout = 90 * time.Second
				newTransport.DisableCompression = false // Enable compression for large payloads
				return newTransport
			}
			// Return the original transport if it's not an *http.Transport
			return rt
		}
	}
}
