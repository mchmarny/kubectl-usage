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

## Installation 

You can install `kusage` CLI using one of the following ways:

* [Go](#go)
* [Homebrew](#homebrew)
* [Binary](#binary)

See the [release section](https://github.com/mchmarny/kusage/releases/latest) for `kusage` checksums and SBOMs.

### Go

If you have Go 1.17 or newer, you can install latest `vimp` using:

```shell
go install github.com/mchmarny/kusage/cmd/cli@latest
```

### Homebrew

On Mac or Linux, you can install `kusage` with [Homebrew](https://brew.sh/):

```shell
brew tap mchmarny/kusage
brew install kusage
```

New release will be automatically picked up when you run `brew upgrade`

### Binary 

You can also download the [latest release](https://github.com/mchmarny/vimp/releases/latest) version of `kusage` for your operating system/architecture from [here](https://github.com/mchmarny/kusage/releases/latest). Put the binary somewhere in your $PATH, and make sure it has that executable bit.

> The official `kusage` releases include SBOMs

## Disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.