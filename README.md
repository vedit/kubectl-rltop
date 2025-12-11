# kubectl-rl-top

[![CI](https://github.com/veditoid/kubectl-rl-top/workflows/CI/badge.svg)](https://github.com/veditoid/kubectl-rl-top/actions)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org/)
[![Krew](https://img.shields.io/badge/krew-plugin-blue.svg)](https://krew.sigs.k8s.io/)

A kubectl krew plugin that displays pod resource usage (CPU and memory) along with resource requests and limits. It works like `kubectl top pods` but also shows the resource requests and limits defined in the pod specifications.

## Features

- Display pod CPU and memory usage (from Metrics API)
- Display CPU and memory requests and limits (from pod specs)
- Support namespace filtering (`-n` or `--namespace`)
- Support label selector filtering (`-l` or `--selector`)
- Works with all namespaces by default
- Handles pods without defined requests/limits gracefully

## Prerequisites

- Go 1.25 or later (required for building from source)
- kubectl installed and configured
- Access to a Kubernetes cluster
- metrics-server installed in your cluster (required for Metrics API)

## Installation

### Using Krew (Recommended)

```bash
kubectl krew install rl-top
```

### Manual Installation

1. Download the latest release for your platform from the [Releases](https://github.com/veditoid/kubectl-rl-top/releases) page
2. Extract the archive
3. Make the binary executable:
   ```bash
   chmod +x kubectl-rltop
   ```
4. Move it to a directory in your PATH:
   ```bash
   sudo mv kubectl-rltop /usr/local/bin/
   ```

### Building from Source

**Note:** Building from source requires Go 1.25 or later due to dependencies on the Kubernetes client libraries.

```bash
git clone https://github.com/veditoid/kubectl-rl-top.git
cd kubectl-rl-top
go build -o kubectl-rltop
sudo mv kubectl-rltop /usr/local/bin/
```

## Usage

You can use `pod`, `pods`, or `po` as the command name, just like kubectl:

```bash
kubectl rltop pod    # Full form
kubectl rltop pods   # Plural form
kubectl rltop po     # Short form
```

### Basic Usage

Display resource usage and requests/limits for all pods:

```bash
kubectl rltop pod
# or
kubectl rltop pods
# or
kubectl rltop po
```

### Filter by Namespace

```bash
kubectl rltop pod -n default
# or
kubectl rltop pods --namespace kube-system
```

### Filter by Label Selector

```bash
kubectl rltop pod -l app=myapp
# or
kubectl rltop pods --selector app=myapp,version=v1
```

### Combine Filters

```bash
kubectl rltop pod -n production -l app=backend
```

## Output Format

The output displays a table with the following columns:

- **NAME**: Pod name
- **CPU(cores)**: Current CPU usage
- **CPU REQUEST**: Requested CPU resources
- **CPU LIMIT**: CPU limit
- **MEMORY(bytes)**: Current memory usage
- **MEMORY REQUEST**: Requested memory resources
- **MEMORY LIMIT**: Memory limit

Example output:

```
NAME                          CPU(cores)  CPU REQUEST  CPU LIMIT  MEMORY(bytes)  MEMORY REQUEST  MEMORY LIMIT
my-pod-abc123                 100m        200m         500m       128Mi          256Mi           512Mi
another-pod-xyz789            50m         100m         200m       64Mi           128Mi           256Mi
pod-without-limits            25m         50m          <none>     32Mi           64Mi            <none>
```

## How It Works

1. Connects to your Kubernetes cluster using the kubeconfig
2. Queries the Metrics API for pod CPU/memory usage (same as `kubectl top pods`)
3. Fetches pod specifications to extract resource requests and limits
4. Combines and formats the data in a table

## Troubleshooting

### Metrics API not available

If you see an error about the Metrics API not being available:

```
Error: metrics API not available: ...
Please ensure metrics-server is installed in your cluster
```

Install metrics-server in your cluster. For example, on minikube:

```bash
minikube addons enable metrics-server
```

### No pods found

If you see "No pods found", check:
- Your namespace filter is correct
- Your label selector is correct
- You have the necessary permissions to list pods

## Development

### Prerequisites

- Go 1.25 or later
- Make (optional, but recommended)

### Building

Using Make:
```bash
make build
```

Or manually:
```bash
go build -o kubectl-rltop
```

### Building for All Platforms

```bash
make build-all
```

This will create binaries in the `dist/` directory for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Testing

Run tests:
```bash
make test
```

Run tests with coverage:
```bash
make test-coverage
```

### Linting

Run linters:
```bash
make lint
```

Fix linting issues automatically:
```bash
make lint-fix
```

### Other Make Targets

- `make clean` - Clean build artifacts
- `make install` - Install the binary locally
- `make release` - Create release artifacts
- `make krew-manifest` - Validate Krew plugin manifest
- `make verify` - Run all verification checks (test + lint)
- `make version` - Show version information

### Testing Locally

```bash
# Test locally
./kubectl-rltop pod

# Test with namespace
./kubectl-rltop pod -n default

# Test with label selector
./kubectl-rltop pod -l app=myapp

# Check version
./kubectl-rltop version
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Ensure all checks pass (`make verify`)
6. Submit a pull request

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a list of changes and version history.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by `kubectl top pods`
- Built with [k8s.io/client-go](https://github.com/kubernetes/client-go)
- Uses [spf13/cobra](https://github.com/spf13/cobra) for CLI

