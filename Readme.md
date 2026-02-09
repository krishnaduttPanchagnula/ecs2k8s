
# ecs2k8s

![Go](https://img.shields.io/badge/Go-1.21%2B-blue?logo=go)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**ecs2k8s** converts AWS ECS clusters and task definitions into equivalent Kubernetes manifests (Deployment, ConfigMap, Service).

## ✨ Features

- ✅ Interactive ECS cluster discovery & selection
- ✅ Automatic task definition listing & conversion
- ✅ Generates valid Kubernetes YAML (Deployment, ConfigMap, Service)
- ✅ AWS SDK v2 with standard auth (IAM, env vars, config)
- ✅ Production-ready error handling & logging
- ✅ Extensible architecture for future features

## 📋 Prerequisites

- Go 1.21+
- AWS credentials configured (`aws configure` or IAM role)
- `kubectl` for testing generated manifests

## 🚀 Quick Start

```bash
# Clone & install
git clone <repo>
cd ecs2k8s
go mod tidy
go install

# Convert ECS cluster (interactive)
ecs2k8s --region us-east-1
```

**Interactive flow:**
```
1. Lists all ECS clusters in region
2. Select cluster (arrow keys + Enter)
3. Auto-discovers all task definitions
4. Generates K8s manifests in `<cluster-name>/`
```

## 📁 Example Output

```
my-cluster/
├── webapp-deployment.yaml     # Main workload
├── webapp-configmap.yaml     # Non-sensitive env vars
└── webapp-service.yaml       # Load balancer service
```

**Sample Deployment YAML:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: webapp
  template:
    metadata:
      labels:
        app: webapp
    spec:
      containers:
      - name: webapp
        image: nginx:1.21
        resources:
          limits:
            cpu: "1024m"
            memory: "512Mi"
        ports:
        - containerPort: 80
```

## 🛠️ Conversion Rules

| ECS → Kubernetes | Mapping |
|------------------|---------|
| **CPU** | ECS units → millicores (1 unit = 1024m) [web:11] |
| **Memory** | MB → MiB |
| **Ports** | First port → Service targetPort |
| **Env Vars** | Non-AWS vars → ConfigMap |
| **Container** | First container → primary Deployment |

## 🔧 Usage

```bash
ecs2k8s --region us-east-1 -r us-west-2
```

**Flags:**
- `--region, -r` *required* - AWS region

## 🧪 Testing Manifests

```bash
cd my-cluster
kubectl apply -f . --dry-run=client -o yaml
kubectl kustomize .  # If using kustomization.yaml
```

## 🏗️ Development

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build
go build -o ecs2k8s ./cmd/ecs2k8s

# Lint
golangci-lint run
```

## 🚀 Future Features

- [ ] Multi-container support
- [ ] Helm chart generation
- [ ] `--dry-run` mode
- [ ] Volume/PVC conversion
- [ ] HPA autoscaling manifests


## 📄 License

MIT License - see [LICENSE](LICENSE) © 2026
```
