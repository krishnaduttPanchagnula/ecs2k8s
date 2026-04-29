# ecs2k8s - AWS ECS to Kubernetes Migration Tool

![Go](https://img.shields.io/badge/Go-1.21%2B-blue?logo=go)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**ecs2k8s** converts AWS ECS clusters and task definitions into Kubernetes manifests (Deployments, Services, ConfigMaps, Secrets, ServiceAccounts), with optional Helm chart and Kustomize structure generation.

## Table of Contents

- [Installation](#installation)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
- [How the Conversion Works](#how-the-conversion-works)
- [Before & After: ECS to Kubernetes](#before--after-ecs-to-kubernetes)
  - [Single Container](#single-container)
  - [Multi-Container Task](#multi-container-task)
  - [IAM Roles (IRSA)](#iam-roles-irsa)
  - [Sensitive vs Non-Sensitive Environment Variables](#sensitive-vs-non-sensitive-environment-variables)
- [Output Structure](#output-structure)
- [Helm Chart Generation](#helm-chart-generation)
- [Kustomize Generation](#kustomize-generation)
- [ECS to Kubernetes Mapping Reference](#ecs-to-kubernetes-mapping-reference)
- [Validation & Deployment](#validation--deployment)
- [Troubleshooting](#troubleshooting)
- [Running Tests](#running-tests)
- [Contributing](#contributing)
- [License](#license)

## Installation

### Homebrew (macOS & Linux)

```bash
brew tap krishnaduttPanchagnula/ecs2k8s
brew install ecs2k8s
```

### WinGet (Windows)

```powershell
winget install KrishnaDuttPanchagnula.ecs2k8s
```

### Go Install

```bash
go install github.com/krishnaduttPanchagnula/ecs2k8s@latest
```

### Binary Download

Download from [Releases](https://github.com/krishnaduttPanchagnula/ecs2k8s/releases) for your platform (linux/darwin/windows, amd64/arm64).

## Prerequisites

- **AWS credentials** configured (`aws configure`, environment variables, or IAM role)
- **kubectl** installed (for applying and verifying manifests)
- **Go 1.21+** (only if building from source)
- IAM permissions: `ecs:ListClusters`, `ecs:ListServices`, `ecs:DescribeServices`, `ecs:DescribeTaskDefinition`

## Usage

```
ecs2k8s --region <aws-region> [--create-helm] [--create-kustomize]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--region` | `-r` | AWS region (required) |
| `--create-helm` | `-H` | Generate a Helm chart alongside raw manifests |
| `--create-kustomize` | `-K` | Generate a Kustomize structure with base and overlays |

### Examples

```bash
# Basic conversion - generates raw K8s YAML manifests
ecs2k8s --region us-east-1

# With Helm chart
ecs2k8s --region us-east-1 --create-helm

# With Kustomize structure
ecs2k8s --region us-east-1 --create-kustomize

# Both Helm and Kustomize
ecs2k8s --region us-east-1 --create-helm --create-kustomize
```

The tool will:
1. List all ECS clusters in the region
2. Present an interactive prompt to select a cluster
3. Discover all services and their task definitions
4. Convert each task definition to Kubernetes manifests
5. Write output to `./<cluster-name>/`

## How the Conversion Works

```
ECS Task Definition
        |
        v
+------------------+
| Container Defs   | ---> K8s Deployment (one pod, N containers)
| Port Mappings    | ---> K8s Service (ClusterIP, per container with ports)
| Environment Vars | ---> K8s ConfigMap (non-sensitive) + Secret (sensitive)
| IAM Roles        | ---> K8s ServiceAccount (with IRSA annotation)
| CPU / Memory     | ---> K8s resource requests & limits
+------------------+
```

**Sensitive detection**: Environment variables with names starting with `AWS`, `SECRET`, `PASSWORD`, `TOKEN`, `KEY`, `PRIVATE`, `ACCESS`, `AUTH`, or `CERT` are placed into a Secret. Everything else goes into a ConfigMap.

## Before & After: ECS to Kubernetes

### Single Container

**ECS Task Definition (input):**

```json
{
  "family": "my-web-app",
  "taskDefinitionArn": "arn:aws:ecs:us-east-1:123456789:task-definition/my-web-app:1",
  "executionRoleArn": "arn:aws:iam::123456789:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::123456789:role/myAppRole",
  "containerDefinitions": [
    {
      "name": "web",
      "image": "nginx:latest",
      "cpu": 512,
      "memory": 1024,
      "portMappings": [
        { "containerPort": 8080, "protocol": "tcp" }
      ],
      "environment": [
        { "name": "APP_ENV", "value": "production" },
        { "name": "LOG_LEVEL", "value": "info" },
        { "name": "AWS_REGION", "value": "us-east-1" },
        { "name": "SECRET_KEY", "value": "mysecret123" }
      ]
    }
  ]
}
```

**Generated Kubernetes manifests (output):**

`my-web-app-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
    labels:
        app: my-web-app
    name: my-web-app
    namespace: default
spec:
    replicas: 1
    selector:
        matchLabels:
            app: my-web-app
    template:
        metadata:
            labels:
                app: my-web-app
        spec:
            containers:
                - env:
                    - name: APP_ENV
                      value: production
                    - name: LOG_LEVEL
                      value: info
                    - name: AWS_REGION
                      value: us-east-1
                    - name: SECRET_KEY
                      value: mysecret123
                  image: nginx:latest
                  name: web
                  ports:
                    - containerPort: 8080
                      protocol: TCP
                  resources:
                    limits:
                        cpu: 512m
                        memory: 1Gi
                    requests:
                        cpu: 512m
                        memory: 1Gi
            serviceAccountName: default-sa
```

`my-web-app-service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
    name: web
spec:
    ports:
        - port: 8080
          protocol: TCP
          targetPort: 8080
    selector:
        app: my-web-app
    type: ClusterIP
```

`my-web-app-configmap.yaml` (non-sensitive env vars only):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
    name: web-config
data:
    APP_ENV: production
    LOG_LEVEL: info
```

`my-web-app-secret.yaml` (sensitive env vars only):

```yaml
apiVersion: v1
kind: Secret
metadata:
    name: web-secret
type: Opaque
stringData:
    AWS_REGION: us-east-1
    SECRET_KEY: mysecret123
```

`my-web-app-serviceaccount.yaml`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
    annotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/myAppRole
    name: default-sa
    namespace: default
```

### Multi-Container Task

ECS task definitions with multiple containers are converted to a single Kubernetes Deployment with all containers in the same pod, and separate Services for each container that exposes ports.

**ECS input** (2 containers: `frontend` on port 8080, `backend` on port 3000):

```json
{
  "family": "multi-app",
  "containerDefinitions": [
    {
      "name": "frontend",
      "image": "nginx:latest",
      "cpu": 256,
      "memory": 512,
      "portMappings": [{ "containerPort": 8080 }],
      "environment": [{ "name": "APP_NAME", "value": "frontend" }]
    },
    {
      "name": "backend",
      "image": "node:18-alpine",
      "cpu": 128,
      "memory": 256,
      "portMappings": [{ "containerPort": 3000 }],
      "environment": [{ "name": "APP_NAME", "value": "backend" }]
    }
  ]
}
```

**Generated output** - one Deployment with both containers, two Services, two ConfigMaps:

`multi-app-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
    labels:
        app: multi-app
    name: multi-app
    namespace: default
spec:
    replicas: 1
    selector:
        matchLabels:
            app: multi-app
    template:
        metadata:
            labels:
                app: multi-app
        spec:
            containers:
                - env:
                    - name: APP_NAME
                      value: frontend
                  image: nginx:latest
                  name: frontend
                  ports:
                    - containerPort: 8080
                      protocol: TCP
                  resources:
                    limits:
                        cpu: 256m
                        memory: 512Mi
                    requests:
                        cpu: 256m
                        memory: 512Mi
                - env:
                    - name: APP_NAME
                      value: backend
                  image: node:18-alpine
                  name: backend
                  ports:
                    - containerPort: 3000
                      protocol: TCP
                  resources:
                    limits:
                        cpu: 128m
                        memory: 256Mi
                    requests:
                        cpu: 128m
                        memory: 256Mi
            serviceAccountName: default-sa
```

`multi-app-service-frontend.yaml` and `multi-app-service-backend.yaml`:

```yaml
# frontend service
apiVersion: v1
kind: Service
metadata:
    name: frontend
spec:
    ports:
        - port: 8080
          protocol: TCP
          targetPort: 8080
    selector:
        app: multi-app
    type: ClusterIP
---
# backend service
apiVersion: v1
kind: Service
metadata:
    name: backend
spec:
    ports:
        - port: 3000
          protocol: TCP
          targetPort: 3000
    selector:
        app: multi-app
    type: ClusterIP
```

### IAM Roles (IRSA)

ECS `taskRoleArn` and `executionRoleArn` are converted to a Kubernetes ServiceAccount with the `eks.amazonaws.com/role-arn` annotation for [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).

The task role is preferred over the execution role (since the task role represents application-level permissions).

```
ECS taskRoleArn: arn:aws:iam::123456789:role/myAppRole
                        |
                        v
K8s ServiceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/myAppRole
```

### Sensitive vs Non-Sensitive Environment Variables

The tool automatically separates environment variables:

| Prefix | Destination | Rationale |
|--------|-------------|-----------|
| `AWS*` | Secret | AWS credentials |
| `SECRET*` | Secret | Explicit secrets |
| `PASSWORD*` | Secret | Passwords |
| `TOKEN*` | Secret | Tokens |
| `KEY*` | Secret | API keys |
| `PRIVATE*` | Secret | Private data |
| `ACCESS*` | Secret | Access credentials |
| `AUTH*` | Secret | Auth data |
| `CERT*` | Secret | Certificates |
| Everything else | ConfigMap | Non-sensitive config |

## Output Structure

### Raw manifests (default)

```
<cluster-name>/
  <task-def>-deployment.yaml
  <task-def>-service.yaml
  <task-def>-configmap.yaml
  <task-def>-secret.yaml
  <task-def>-serviceaccount.yaml
```

### With `--create-helm`

```
<cluster-name>/
  <task-def>-deployment.yaml          # Raw manifests (always generated)
  <task-def>-service.yaml
  ...
  helm/<cluster-name>/
    Chart.yaml
    values.yaml
    templates/
      _helpers.tpl
      deployment/deployment.yaml
      service/service.yaml
      configmap/configmap.yaml
      secret/
      serviceaccount/serviceaccount.yaml
```

### With `--create-kustomize`

```
<cluster-name>/
  <task-def>-deployment.yaml          # Raw manifests (always generated)
  ...
  kustomize/<cluster-name>/
    kustomization.yaml                # Root kustomization
    base/
      kustomization.yaml
      deployments/<task>-deployment.yaml
      services/<task>-service.yaml
      configmaps/<task>-configmap-0.yaml
      secrets/<task>-secret-0.yaml
      serviceaccounts/<task>-serviceaccount.yaml
    overlays/
      dev/
        kustomization.yaml            # namespace: development
        patches/
      staging/
        kustomization.yaml            # namespace: staging
        patches/
      prod/
        kustomization.yaml            # namespace: production
        patches/
```

## Helm Chart Generation

With `--create-helm`, the tool generates a complete Helm chart with all services combined in a single `values.yaml`:

```yaml
# values.yaml (generated)
defaultNamespace: default
defaultReplicas: 1
services:
    api-service:
        containers:
            - image: myrepo/api-service:v2.1.0
              name: api
              ports:
                - 8080
              resources:
                limits:
                    cpu: 512m
                    memory: 1Gi
                requests:
                    cpu: 512m
                    memory: 1Gi
              env:
                - name: APP_ENV
                  value: production
        iamRoleArn: arn:aws:iam::123456789:role/apiServiceRole
        namespace: default
        replicas: 1
        service:
            name: api
            port: 8080
            type: ClusterIP
        serviceAccount:
            annotations:
                eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/apiServiceRole
```

### Using the Helm chart

```bash
# Install
helm install my-release ./<cluster>/helm/<cluster>/

# Override values
helm install my-release ./<cluster>/helm/<cluster>/ \
  --set services.api-service.replicas=3

# Deploy to a specific namespace
helm install my-release ./<cluster>/helm/<cluster>/ -n production --create-namespace

# Dry-run
helm template my-release ./<cluster>/helm/<cluster>/
```

## Kustomize Generation

With `--create-kustomize`, the tool generates a base + overlays structure with three environments (dev, staging, prod), each applying a different namespace.

### Using Kustomize

```bash
# Preview dev overlay
kubectl kustomize ./<cluster>/kustomize/<cluster>/overlays/dev/

# Apply to dev
kubectl apply -k ./<cluster>/kustomize/<cluster>/overlays/dev/

# Apply to production
kubectl apply -k ./<cluster>/kustomize/<cluster>/overlays/prod/
```

## ECS to Kubernetes Mapping Reference

| ECS Field | Kubernetes Field | Notes |
|-----------|-----------------|-------|
| `containerDefinitions[].name` | `containers[].name` | Direct mapping |
| `containerDefinitions[].image` | `containers[].image` | Direct mapping |
| `containerDefinitions[].cpu` (units) | `resources.limits.cpu` | ECS CPU units = Kubernetes millicores (e.g., 512 -> `512m`) |
| `containerDefinitions[].memory` (MiB) | `resources.limits.memory` | Converted to binary bytes (e.g., 1024 MiB -> `1Gi`) |
| `containerDefinitions[].portMappings` | `containerPort` + `Service` | Creates a ClusterIP Service per container |
| `containerDefinitions[].environment` | `ConfigMap` / `Secret` | Split by sensitivity prefix |
| `taskRoleArn` | `ServiceAccount` annotation | `eks.amazonaws.com/role-arn` for IRSA |
| `executionRoleArn` | `ServiceAccount` annotation (fallback) | Used if taskRoleArn is absent |
| Multiple containers | Single Pod, multiple containers | All containers in one Deployment pod |

## Validation & Deployment

```bash
# Validate manifests without applying (server-side dry-run)
kubectl apply -f <cluster>/ --dry-run=server

# Apply to cluster
kubectl apply -f <cluster>/

# Verify pods are running
kubectl get pods -l app=<task-def-name>

# Check Service endpoints
kubectl get endpoints <service-name>

# View logs
kubectl logs -l app=<task-def-name>
```

## Troubleshooting

### AWS Credentials Not Found

```bash
aws configure
# or
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

### No Clusters Found

```bash
# Verify ECS clusters exist in the region
aws ecs list-clusters --region us-east-1
```

### Pod Not Starting

```bash
# Check events
kubectl describe pod <pod-name>

# Check resource constraints
kubectl describe nodes
```

### Pod Running But Service Has No Endpoints

Verify the Service selector matches the Deployment's pod labels:

```bash
kubectl get svc <name> -o jsonpath='{.spec.selector}'
kubectl get pods --show-labels
```

## Running Tests

```bash
# All tests
go test ./...

# Verbose
go test ./... -v

# Validators only with benchmarks
go test ./validators -bench=. -benchtime=5s

# With coverage
go test ./... -cover
```

## Supported Regions

us-east-1, us-east-2, us-west-1, us-west-2, eu-west-1, eu-west-2, eu-west-3, eu-central-1, eu-north-1, ap-south-1, ap-southeast-1, ap-southeast-2, ap-northeast-1, ap-northeast-2, ap-northeast-3, ca-central-1, sa-east-1

Other regions may work but will produce a warning.

## Contributing

Contributions are welcome. Please open an issue or pull request on [GitHub](https://github.com/krishnaduttPanchagnula/ecs2k8s).

## License

MIT - see [LICENSE](LICENSE)
