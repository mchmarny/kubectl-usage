# kusage

Rank pods/containers by resource usage-to-limit ratio

Features: 

* Memory-bounded streaming (processes k8s data through bounded channels)
* Adaptive pagination with API client/call optimization (chunked data retrieval)
* Circuit breaker with resource pools (prevent cascading failures)

## Usage

```bash
# Analyze pod-level memory usage with custom namespace filtering
kusage pods -A --resource memory --nx '^(observability|kube-system)$' --top 10 --sort pct

# Container analysis with custom sort and limit
kusage containers -n production --resource=memory --sort limit --top 5
```

## Requirements

- **Kubernetes Permissions**: 
  - `pods` (get, list) in target namespaces
  - `pods/metrics` (get, list) via `metrics.k8s.io` API group
- **Cluster Components**: 
  - [metrics-server](https://github.com/kubernetes-sigs/metrics-server) must be installed and running

## Installation 

You can install `kusage` CLI using one of the following ways:

* [Homebrew](#homebrew)
* [Go](#go)
* [Binary](#binary)

See the [release section](https://github.com/mchmarny/kusage/releases/latest) for `kusage` checksums and SBOMs.

### Homebrew

On Mac or Linux, you can install `kusage` with [Homebrew](https://brew.sh/):

```shell
brew tap mchmarny/kusage
brew install kusage
```

New release will be automatically picked up when you run `brew upgrade`

### Go

If you have Go 1.17 or newer, you can install latest `vimp` using:

```shell
go install github.com/mchmarny/kusage/cmd/kusage@latest
```

### Binary 

You can also download the [latest release](https://github.com/mchmarny/vimp/releases/latest) version of `kusage` for your operating system/architecture from [here](https://github.com/mchmarny/kusage/releases/latest). Put the binary somewhere in your $PATH, and make sure it has that executable bit.

> The official `kusage` releases include SBOMs

## Disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.