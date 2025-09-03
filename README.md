# kusage

Rank Kubernetes pods/containers by CPU/memory usage (usage ÷ limit)

## Usage

```bash
# Analyze pod-level memory usage across all namespaces
kusage pods -A

# Analyze container-level CPU usage with custom namespace filtering
kusage containers -A --resource=cpu --ns='^(kube-system|monitoring)$'

# Show top 10 results sorted by raw usage
kusage pods --sort=usage --top=10

# Filter by label selector and sort by percentage
kusage pods -l app=frontend --sort=pct

# Container analysis with custom sort and limit
kusage containers -n production --resource=memory --sort=limit --top=5
```

## Requirements

- **Kubernetes Permissions**: 
  - `pods` (get, list) in target namespaces
  - `pods/metrics` (get, list) via `metrics.k8s.io` API group
- **Cluster Components**: 
  - [metrics-server](https://github.com/kubernetes-sigs/metrics-server) must be installed and running

## Disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.