# Helm Chart Deployment

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+

## Quick Install

```bash
helm repo add lattice https://alatticeio.github.io/lattice
helm repo update

helm install lattice lattice/lattice \
  --namespace lattice-system \
  --create-namespace
```

## Configuration

Create a `values.yaml` file:

```yaml
config:
  app:
    name: "My Lattice"
    env: "production"
  jwt:
    secret: "your-32-byte-random-secret"

ingress:
  enabled: true
  host: lattice.mycompany.com

persistence:
  size: 50Gi
```

Then install with:

```bash
helm install lattice lattice/lattice \
  --namespace lattice-system \
  --values values.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image | `ghcr.io/alatticeio/latticed` |
| `image.tag` | Image tag | `latest` |
| `replicaCount` | Number of replicas | `1` |
| `service.type` | Service type | `ClusterIP` |
| `persistence.enabled` | Enable PVC for SQLite data | `true` |
| `persistence.size` | Storage size | `10Gi` |
| `config.jwt.secret` | JWT signing secret (required) | `""` |
| `config.database.dsn` | Database DSN | `data/lattice.db` |
| `ingress.enabled` | Enable ingress | `false` |
| `license.enabled` | Enable Pro license | `false` |

## Pro Edition

```yaml
license:
  enabled: true
  fileContents: "eyJhbGciOiJFZERTQSJ9..."
```

## Access the Dashboard

```bash
kubectl port-forward -n lattice-system svc/lattice 8080:8080
```

Then open `http://localhost:8080`.
