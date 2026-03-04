# Troubleshooting

Common issues organized by category, with symptoms, causes, and solutions.

---

## Installation Issues

### Operator pod is in CrashLoopBackOff

**Symptoms:** The operator manager pod repeatedly crashes.

**Possible Causes:**
1. Missing or invalid API key
2. Missing required environment variables

**Solution:**
```bash
# Check the logs
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager

# Verify the secret exists
oc get secret roks-vpc-network-operator-credentials -n roks-vpc-network-operator

# Verify the API key is set
oc get secret roks-vpc-network-operator-credentials -n roks-vpc-network-operator \
  -o jsonpath='{.data.IBMCLOUD_API_KEY}' | base64 -d | wc -c
# Should show a non-zero character count
```

### CRDs not found

**Symptoms:** `error: the server doesn't have a resource type "vpcsubnets"`

**Solution:**
```bash
# Verify CRDs are installed
oc get crd | grep vpc.roks.ibm.com

# If missing, reinstall the Helm chart
helm upgrade --install vpc-network-operator deploy/helm/roks-vpc-network-operator \
  --namespace roks-vpc-network-operator
```

### Console plugin pages not appearing

**Symptoms:** No VPC Networking pages in the OpenShift Console sidebar.

**Solution:**
```bash
# Verify the plugin is enabled
oc get consoles.operator.openshift.io cluster -o jsonpath='{.spec.plugins}'

# Enable if not present
oc patch consoles.operator.openshift.io cluster \
  --type=merge \
  --patch '{"spec":{"plugins":["vpc-network-console-plugin"]}}'

# Verify plugin pods are running
oc get pods -n roks-vpc-network-operator -l app=vpc-network-console-plugin
```

---

## CUDN and Subnet Issues

### CUDN stuck without status annotations

**Symptoms:** The CUDN is created but `vpc.roks.ibm.com/subnet-id` annotation never appears.

**Possible Causes:**
1. Missing or invalid annotations on the CUDN
2. VPC API authentication failure
3. CIDR overlaps with existing subnet

**Solution:**
```bash
# Check operator logs for errors
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager | grep -i "cudn\|subnet\|error"

# Verify all required annotations are present
oc get cudn <name> -o yaml | grep 'vpc.roks.ibm.com'

# Required: zone, cidr, vpc-id, vlan-id, security-group-ids, acl-id
```

### VPC subnet creation fails with "CIDR conflict"

**Symptoms:** Operator logs show a CIDR overlap error.

**Solution:**
```bash
# List existing subnets in your VPC
ibmcloud is subnets --vpc <vpc-id>

# Choose a non-overlapping CIDR and update the CUDN annotation
oc annotate cudn <name> vpc.roks.ibm.com/cidr="10.240.128.0/24" --overwrite
```

### CUDN deletion blocked

**Symptoms:** `oc delete cudn <name>` hangs.

**Cause:** The finalizer is blocking deletion because VMs still reference this CUDN, or the VPC API delete is failing.

**Solution:**
```bash
# Check if VMs are still using this network
oc get vm --all-namespaces -o json | jq '.items[] | select(.spec.template.spec.networks[].multus.networkName == "<cudn-name>") | .metadata.name'

# Delete those VMs first
oc delete vm <vm-name> -n <namespace>

# If stuck on VPC API failure, check logs
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager | grep "delete\|finalizer"
```

---

## VM and Webhook Issues

### VM creation fails with "webhook timeout"

**Symptoms:** `error: Internal error occurred: failed calling webhook: context deadline exceeded`

**Possible Causes:**
1. VPC API is slow or unreachable
2. Operator manager pod is not running
3. Webhook service is not reachable

**Solution:**
```bash
# Check if operator is running
oc get pods -n roks-vpc-network-operator

# Check webhook configuration
oc get mutatingwebhookconfigurations | grep roks-vpc

# Check operator logs for VPC API errors
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager | tail -50
```

### VM created but no VPC annotations

**Symptoms:** VM is running but has no `vpc.roks.ibm.com/*` annotations.

**Cause:** The VM does not reference a LocalNet-backed CUDN, so the webhook passed it through unmodified.

**Solution:**
```bash
# Verify the VM references a CUDN with LocalNet topology
oc get vm <name> -o jsonpath='{.spec.template.spec.networks}'

# Verify the CUDN exists and has topology: LocalNet
oc get cudn <network-name> -o yaml
```

### VM has VPC annotations but cannot reach the network

**Symptoms:** VM is running with VPC annotations, but cannot ping or connect to VPC resources.

**Possible Causes:**
1. Security group rules are too restrictive
2. Network ACL is blocking traffic
3. VLAN attachment is missing on the node where the VM runs

**Solution:**
```bash
# Check which node the VM is on
oc get vmi <name> -o jsonpath='{.status.nodeName}'

# Verify VLAN attachment exists for that node
oc get cudn <network-name> -o yaml | grep 'vlan-attachments'

# Check security group rules
ibmcloud is security-group <sg-id>

# Check ACL rules
ibmcloud is network-acl <acl-id>
```

---

## VPC API Issues

### "Unauthorized" errors in operator logs

**Symptoms:** Logs show `401 Unauthorized` or `IAM token expired`.

**Solution:**
```bash
# Verify the API key is valid
ibmcloud iam api-key-verify --apikey <your-api-key>

# Recreate the secret if the key was rotated
oc delete secret roks-vpc-network-operator-credentials -n roks-vpc-network-operator
oc create secret generic roks-vpc-network-operator-credentials \
  --namespace roks-vpc-network-operator \
  --from-literal=IBMCLOUD_API_KEY=<new-api-key>

# Restart the operator to pick up the new key
oc rollout restart deployment/vpc-network-operator-manager -n roks-vpc-network-operator
```

### "Rate limited" errors

**Symptoms:** Logs show `429 Too Many Requests` from the VPC API.

**Cause:** The operator is making too many concurrent API calls, exceeding VPC API rate limits.

**Solution:** The built-in rate limiter (10 concurrent max) should prevent this. If it persists:
1. Check if multiple operator instances are running (leader election issue)
2. Reduce the number of simultaneous CUDN/VM operations
3. The operator will automatically retry with backoff

### VPC quota exceeded

**Symptoms:** Subnet or VNI creation fails with quota error.

**Solution:**
```bash
# Check your VPC quotas
ibmcloud is quotas

# Request a quota increase if needed via IBM Cloud support
```

---

## Drift and Orphan Issues

### "Drift detected" warning events

**Symptoms:** Kubernetes events on VMs warn about VNI drift.

**Cause:** Someone deleted or modified a VPC resource (VNI, subnet) outside the operator (e.g., via IBM Cloud console or Terraform).

**Solution:**
- If intentional (e.g., Terraform manages the resource), consider disabling drift detection for that resource
- If accidental, recreate the VPC resource by deleting and recreating the VM

### Orphaned VPC resources

**Symptoms:** VPC resources tagged with the cluster ID exist but have no corresponding Kubernetes object.

**Cause:** The orphan GC has not yet cleaned them up (15-minute grace period), or the GC itself has issues.

**Solution:**
```bash
# Check orphan GC logs
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager | grep "orphan-gc"

# Manually list VPC resources tagged by the operator
ibmcloud is virtual-network-interfaces --output json | \
  jq '.[] | select(.tags[]? == "roks-operator:true") | select(.tags[]? == "roks-cluster:<cluster-id>")'

# If needed, manually delete orphaned resources
ibmcloud is virtual-network-interface-delete <vni-id>
```

---

## Console Plugin Issues

### BFF service returning errors

**Symptoms:** Console pages show error messages or empty tables.

**Solution:**
```bash
# Check BFF pod logs
oc logs -n roks-vpc-network-operator deployment/vpc-network-bff

# Verify BFF is running
oc get pods -n roks-vpc-network-operator -l app=vpc-network-bff

# Test the BFF health endpoint
oc exec -n roks-vpc-network-operator deployment/vpc-network-bff -- curl -s localhost:8443/healthz
```

### "Forbidden" errors on create/delete actions

**Symptoms:** The console shows "forbidden" when trying to create or delete security groups/ACLs.

**Cause:** The user lacks RBAC permissions.

**Solution:** See [RBAC](admin-guide/rbac.md) for configuring permissions. The administrator needs to create appropriate ClusterRoleBindings or RoleBindings.

---

## Getting Help

If you cannot resolve an issue:

1. Collect diagnostics: `oc logs`, `oc describe`, `oc get events`
2. Check the operator version: `helm list -n roks-vpc-network-operator`
3. Review the [FAQ](faq.md) for additional questions
4. File an issue with logs and resource YAMLs
