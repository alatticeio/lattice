# Network Policy

## Default Deny

**Wireflow's zero trust policy is default deny all. a policy will be created automatically when you create a namespace.**

* **ingress deny**
```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
  namespace: my-namespace
spec:
  peerSelector: {} # all peers when leave empty
  policyTypes:
  - Ingress
  # 不写 ingress 规则，意味着没有任何流量被允许
```

* **deny all**
```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: my-namespace
spec:
  peerSelector: {} #all peers when leave empty
  policyTypes:
    - Ingress
    - Egress
```

## Acls rules introduction


