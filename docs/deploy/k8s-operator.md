# K8s Operator Deployment

Deploy Lattice as a Kubernetes operator with CRD-based network management.

## Prerequisites

- Kubernetes 1.24+
- kubectl

## Install CRDs

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/crd
```

## Deploy the Operator

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
```

This deploys:
- `latticed` all-in-one control plane (NATS + API + Dashboard)
- CRDs: `LatticeNetwork`, `LatticePeer`, `LatticePolicy`, `LatticeGlobalIPPool`
- Required RBAC roles and bindings
- Dashboard web UI

## Verify

```bash
kubectl get pods -n lattice-system
kubectl get latticenetworks
```

## Access Dashboard

```bash
kubectl port-forward -n lattice-system svc/lattice 8080:8080
```

## Deploy with MariaDB (production)

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/production
```
