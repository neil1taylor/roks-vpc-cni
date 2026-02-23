# Upgrading

This guide covers the process of upgrading the VPC Network Operator, including the BFF service and console plugin.

## Upgrade Process

The operator is deployed via Helm, so upgrades are performed with `helm upgrade`. All three components (operator, BFF, console plugin) are upgraded together as part of the same Helm release.

```bash
helm upgrade roks-vpc-network-operator \
  deploy/helm/roks-vpc-network-operator/ \
  --namespace roks-vpc-network-operator \
  --values my-values.yaml
```

To upgrade to a specific image version:

```bash
helm upgrade roks-vpc-network-operator \
  deploy/helm/roks-vpc-network-operator/ \
  --namespace roks-vpc-network-operator \
  --values my-values.yaml \
  --set image.tag=v1.2.0 \
  --set bff.image.tag=v1.2.0 \
  --set consolePlugin.image.tag=v1.2.0
```

## Pre-Upgrade Checklist

Before upgrading, complete the following steps:

### 1. Review release notes

Check the release notes for the target version for breaking changes, new required configuration, or changes to CRD schemas.

### 2. Back up your configuration

Save your current Helm values:

```bash
helm get values roks-vpc-network-operator \
  --namespace roks-vpc-network-operator \
  --output yaml > values-backup.yaml
```

### 3. Check the current state of the operator

Verify the operator is healthy and not in the middle of reconciling resources:

```bash
# Check operator pod status
kubectl get pods -n roks-vpc-network-operator

# Check for any pending reconciliation
kubectl get vpcsubnets -o wide
kubectl get virtualnetworkinterfaces -o wide
kubectl get vlanattachments -o wide
kubectl get floatingips -o wide
```

Ensure all resources show a ready or stable status before proceeding.

### 4. Check running VMs

List all running VirtualMachines to be aware of what workloads are active. The upgrade should not disrupt running VMs, but it is good practice to know the current state:

```bash
kubectl get virtualmachines --all-namespaces
```

### 5. Verify the target chart version

If upgrading from a chart repository:

```bash
helm repo update
helm search repo roks-vpc-network-operator --versions
```

## CRD Updates

Custom Resource Definitions (CRDs) are cluster-scoped and are not always updated automatically by `helm upgrade`. If the new version includes CRD schema changes, you must apply CRD updates manually before upgrading the Helm release.

### Check for CRD changes

Compare the CRDs in the new chart version with the currently installed CRDs:

```bash
# View currently installed CRDs
kubectl get crds | grep vpc.roks.ibm.com

# View CRD details
kubectl get crd vpcsubnets.vpc.roks.ibm.com -o yaml
kubectl get crd virtualnetworkinterfaces.vpc.roks.ibm.com -o yaml
kubectl get crd vlanattachments.vpc.roks.ibm.com -o yaml
kubectl get crd floatingips.vpc.roks.ibm.com -o yaml
```

### Apply CRD updates

If the release notes indicate CRD changes, apply them before running `helm upgrade`:

```bash
kubectl apply -f deploy/helm/roks-vpc-network-operator/crds/
```

This is a non-destructive operation. Existing custom resources are preserved; only the CRD schema definition is updated.

### Verify CRD updates

```bash
kubectl get crds | grep vpc.roks.ibm.com
```

Confirm that the CRD versions and stored versions match the expected values from the new release.

## Rolling Updates

The operator supports zero-downtime upgrades through Kubernetes rolling update strategy and leader election.

### How it works

1. Helm triggers a rolling update of the operator Deployment.
2. A new operator pod starts and attempts to acquire the leader election lease.
3. The old operator pod continues reconciling until the new pod is ready.
4. Once the new pod acquires the lease, it becomes the active reconciler.
5. The old pod shuts down gracefully.

Leader election (`leaderElection.enabled: true` by default) ensures that only one operator instance is actively reconciling at any time, preventing conflicts during the rollover.

### BFF and console plugin

The BFF and console plugin run with multiple replicas (`bff.replicas: 2`, `consolePlugin.replicas: 2` by default). Their Deployments use a rolling update strategy, so at least one replica of each remains available during the upgrade.

### Monitoring the rollout

```bash
# Watch the operator rollout
kubectl rollout status deployment/roks-vpc-network-operator \
  -n roks-vpc-network-operator

# Watch the BFF rollout
kubectl rollout status deployment/vpc-network-bff \
  -n roks-vpc-network-operator

# Watch the console plugin rollout
kubectl rollout status deployment/vpc-network-console-plugin \
  -n roks-vpc-network-operator
```

### Verify post-upgrade

After the rollout completes:

```bash
# Check all pods are running
kubectl get pods -n roks-vpc-network-operator

# Check operator logs for errors
kubectl logs -n roks-vpc-network-operator deployment/roks-vpc-network-operator \
  --tail=50

# Verify the operator version
kubectl get deployment roks-vpc-network-operator \
  -n roks-vpc-network-operator \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Rollback

If the upgrade causes issues, roll back to the previous Helm release revision.

### Check revision history

```bash
helm history roks-vpc-network-operator --namespace roks-vpc-network-operator
```

### Roll back to the previous revision

```bash
helm rollback roks-vpc-network-operator <revision-number> \
  --namespace roks-vpc-network-operator
```

### Roll back CRDs

If CRDs were updated and the rollback requires the previous CRD schemas, reapply the CRDs from the previous chart version:

```bash
# Check out the previous chart version
# Apply the old CRDs
kubectl apply -f deploy/helm/roks-vpc-network-operator/crds/
```

Note that rolling back CRDs can fail if existing custom resources use fields that were added in the newer schema. In that case, you may need to manually edit those resources before the CRD rollback.

### Verify rollback

```bash
kubectl get pods -n roks-vpc-network-operator
kubectl logs -n roks-vpc-network-operator deployment/roks-vpc-network-operator \
  --tail=50
```

---

**See also:**

- [Configuration](configuration.md) -- Helm values reference
- [Uninstalling](uninstalling.md) -- full removal of the operator
- [RBAC and Access Control](rbac.md) -- permissions that may change between versions
