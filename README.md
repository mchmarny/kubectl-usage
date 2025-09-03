# kubectl-usage

A kubectl plugin for analyzing Kubernetes resource usage by comparing actual metrics against resource limits, designed with distributed systems best practices.

## Usage

```bash
# Analyze pod-level memory usage across all namespaces
kubectl saturation pods -A

# Analyze container-level CPU usage with custom filtering
kubectl saturation containers -A --resource=cpu --exclude-ns='^(kube-system|monitoring)$'

# Show top 10 results sorted by raw usage
kubectl saturation pods --sort=usage --top=10

# Filter by label selector and sort by percentage
kubectl saturation pods -l app=frontend --sort=pct

# Container analysis with custom sort and limit
kubectl saturation containers -n production --resource=memory --sort=limit --top=5
```

## Requirements

- **Kubernetes Permissions**: 
  - `pods` (get, list) in target namespaces
  - `pods/metrics` (get, list) via `metrics.k8s.io` API group
- **Cluster Components**: 
  - [metrics-server](https://github.com/kubernetes-sigs/metrics-server) must be installed and running

## Installation

```bash
# Build the binary
go build -o kubectl-saturation ./cmd/cli

# Install as a kubectl plugin (optional)
sudo mv kubectl-saturation /usr/local/bin/kubectl-saturation
```

## References

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [client-go Documentation](https://pkg.go.dev/k8s.io/client-go)
- [Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
- [kubectl Plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)

## Disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.