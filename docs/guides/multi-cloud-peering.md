# Multi-Cloud Cluster Peering

Connect Kubernetes clusters across AWS and GCP with WireGuard.

## Prerequisites

- Two Kubernetes clusters (any cloud or on-prem)
- `kubectl` configured for both clusters
- `helm` installed

## Step 1: Install Lattice Operator

```bash
helm repo add lattice https://charts.alattice.io
helm install lattice lattice/lattice-operator -n lattice --create-namespace
```

Repeat on both clusters.

## Step 2: Configure Network CIDRs

Create a `LatticeNetwork` CRD on each cluster:

```yaml
apiVersion: lattice.alattice.io/v1alpha1
kind: LatticeNetwork
metadata:
  name: cluster-peering
spec:
  cidr: 10.0.0.0/16
  peering:
    enabled: true
    remoteCIDR: 10.1.0.0/16
```

## Step 3: Verify Connectivity

```bash
kubectl exec -it deploy/sample-app -- curl http://10.1.2.3:8080/health
```

## Next Steps

- [Network Policies](/features/policies) — Control which services can communicate
- [Topology Viewer](/features/topology) — Visualize the cross-cluster mesh
