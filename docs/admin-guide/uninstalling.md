# Uninstalling

This guide covers the complete removal of the VPC Network Operator from your cluster, including cleanup of all associated VPC resources.

## Before You Uninstall

**Warning:** The VPC Network Operator manages VPC resources (subnets, VNIs, VLAN attachments, floating IPs) that are tied to your KubeVirt VMs. If you uninstall the operator without first deleting these resources, they will become orphaned in your IBM Cloud account and continue to incur charges. You must delete all VMs and CUDNs before uninstalling.

The recommended order is:

1. Delete all VirtualMachines
2. Delete all ClusterUserDefinedNetworks
3. Verify VPC resource cleanup
4. Uninstall the Helm release
5. Remove CRDs
6. Remove the namespace

## Step 1: Delete All VMs

Delete all VirtualMachines that use VPC networking. The VM reconciler's finalizer (`vpc.roks.ibm.com/vm-cleanup`) ensures that each VM's VNI and floating IP are deleted from the VPC before the VM object is removed.

### List all VMs

```bash
kubectl get virtualmachines --all-namespaces
```

### Delete VMs namespace by namespace

```bash
kubectl delete virtualmachines --all -n <namespace>
```

### Delete VMs across all namespaces

```bash
kubectl delete virtualmachines --all --all-namespaces
```

### Wait for deletion to complete

```bash
kubectl get virtualmachines --all-namespaces --watch
```

Wait until no VirtualMachines remain. If a VM is stuck in terminating state, check the operator logs for errors:

```bash
kubectl logs -n roks-vpc-network-operator deployment/roks-vpc-network-operator \
  --tail=100
```

A stuck finalizer usually indicates a VPC API error. Resolve the underlying issue or, as a last resort, manually remove the finalizer:

```bash
kubectl patch virtualmachine <vm-name> -n <namespace> \
  --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]'
```

If you manually remove a finalizer, you must clean up the associated VPC resources yourself (see "Cleaning Up Orphaned VPC Resources" below).

## Step 2: Delete All CUDNs

Delete all ClusterUserDefinedNetworks managed by the operator. The CUDN reconciler's finalizer (`vpc.roks.ibm.com/cudn-cleanup`) ensures that VLAN attachments and the VPC subnet are deleted before the CUDN object is removed.

### List all CUDNs

```bash
kubectl get clusteruserdefinednetworks
```

### Delete CUDNs

```bash
kubectl delete clusteruserdefinednetwork --all
```

### Wait for deletion to complete

```bash
kubectl get clusteruserdefinednetworks --watch
```

Wait until no CUDNs remain. If a CUDN is stuck in terminating state, check operator logs and follow the same approach as for stuck VMs above.

## Step 3: Verify VPC Resource Cleanup

Before uninstalling the operator, verify that all VPC resources have been cleaned up.

### Check Kubernetes CRDs

```bash
# All of these should return "No resources found"
kubectl get vpcsubnets --all-namespaces
kubectl get virtualnetworkinterfaces --all-namespaces
kubectl get vlanattachments --all-namespaces
kubectl get floatingips --all-namespaces
```

### Verify in IBM Cloud

```bash
# Check for subnets tagged with your cluster ID
ibmcloud is subnets --vpc <vpc-id> --output json | \
  jq '.[] | select(.tags[]? | contains("<cluster-id>"))'

# Check for floating IPs tagged with your cluster ID
ibmcloud is floating-ips --output json | \
  jq '.[] | select(.tags[]? | contains("<cluster-id>"))'

# Check for VNIs tagged with your cluster ID
ibmcloud is virtual-network-interfaces --output json | \
  jq '.[] | select(.tags[]? | contains("<cluster-id>"))'
```

If any tagged resources remain, wait for the orphan GC to clean them up (up to the GC interval plus grace period), or delete them manually.

## Step 4: Uninstall the Helm Release

Once all VPC resources are confirmed deleted, uninstall the Helm release:

```bash
helm uninstall roks-vpc-network-operator --namespace roks-vpc-network-operator
```

This removes:

- The operator Deployment and its pods
- The BFF Deployment and its pods
- The console plugin Deployment and its pods
- All Services, ServiceAccounts, ClusterRoles, ClusterRoleBindings, Roles, and RoleBindings created by the chart
- The MutatingWebhookConfiguration
- ConfigMaps and other chart-managed resources

Verify the pods are terminated:

```bash
kubectl get pods -n roks-vpc-network-operator
```

## Step 5: Remove CRDs

Helm does not delete CRDs on uninstall (this is by design to protect custom resource data). Remove the CRDs manually:

```bash
kubectl delete crd vpcsubnets.vpc.roks.ibm.com
kubectl delete crd virtualnetworkinterfaces.vpc.roks.ibm.com
kubectl delete crd vlanattachments.vpc.roks.ibm.com
kubectl delete crd floatingips.vpc.roks.ibm.com
```

**Warning:** Deleting a CRD also deletes all custom resources of that type. This is why you must ensure all CRD instances are already deleted in the previous steps. If any remain, deleting the CRD will remove them from Kubernetes without triggering finalizers or VPC cleanup.

Verify CRDs are removed:

```bash
kubectl get crds | grep vpc.roks.ibm.com
```

## Step 6: Remove Namespace

If the namespace is no longer needed:

```bash
kubectl delete namespace roks-vpc-network-operator
```

This removes any remaining resources in the namespace, including the credentials Secret.

Verify:

```bash
kubectl get namespace roks-vpc-network-operator
```

## Cleaning Up Orphaned VPC Resources

If VPC resources were not properly cleaned up (for example, due to manually removed finalizers, operator crashes, or the operator being uninstalled before deleting VMs and CUDNs), you must delete them manually from IBM Cloud.

### Find orphaned resources

All VPC resources created by the operator are tagged with the cluster ID. Use this to identify orphaned resources:

```bash
# Find orphaned subnets
ibmcloud is subnets --vpc <vpc-id> --output json | \
  jq -r '.[] | select(.tags[]? | contains("<cluster-id>")) | "\(.id)\t\(.name)\t\(.ipv4_cidr_block)"'

# Find orphaned floating IPs
ibmcloud is floating-ips --output json | \
  jq -r '.[] | select(.tags[]? | contains("<cluster-id>")) | "\(.id)\t\(.name)\t\(.address)"'

# Find orphaned virtual network interfaces
ibmcloud is virtual-network-interfaces --output json | \
  jq -r '.[] | select(.tags[]? | contains("<cluster-id>")) | "\(.id)\t\(.name)"'
```

### Delete orphaned resources

Delete resources in the correct order to avoid dependency errors. Floating IPs and VNIs must be deleted before subnets.

**1. Delete floating IPs:**

```bash
ibmcloud is floating-ip-release <floating-ip-id> --force
```

**2. Delete virtual network interfaces:**

```bash
ibmcloud is virtual-network-interface-delete <vni-id> --force
```

**3. Delete VLAN attachments (bare metal NICs):**

VLAN attachments are network interfaces on bare metal servers. List and delete them:

```bash
# List network interfaces on a bare metal server
ibmcloud is bare-metal-server-network-interfaces <server-id>

# Delete a VLAN attachment
ibmcloud is bare-metal-server-network-interface-delete <server-id> <nic-id> --force
```

**4. Delete subnets:**

```bash
ibmcloud is subnet-delete <subnet-id> --force
```

A subnet cannot be deleted if it still has attached resources (VNIs, load balancers, etc.). If the delete fails, check for remaining resources in the subnet:

```bash
ibmcloud is subnet <subnet-id> --output json
```

### Revoke the Service ID API key

If you are fully decommissioning the operator, also clean up the IBM Cloud Service ID:

```bash
# List API keys for the service ID
ibmcloud iam service-api-keys vpc-network-operator

# Delete the API key
ibmcloud iam service-api-key-delete <key-id> vpc-network-operator --force

# Optionally delete the service ID
ibmcloud iam service-id-delete vpc-network-operator --force
```

---

**See also:**

- [Network Setup](network-setup.md) -- deleting individual CUDNs
- [Configuration](configuration.md) -- orphan GC settings
- [VPC Prerequisites](vpc-prerequisites.md) -- the resources that were created before installation
