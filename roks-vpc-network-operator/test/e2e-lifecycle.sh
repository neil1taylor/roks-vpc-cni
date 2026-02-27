#!/usr/bin/env bash
# e2e-lifecycle.sh — End-to-end lifecycle test for the ROKS VPC Network Operator.
#
# Exercises the full VPC resource lifecycle:
#   CUDN → VPC Subnet → VLAN Attachments → VM → VNI → Cleanup
#
# Required environment variables:
#   E2E_VPC_ID          — VPC ID (e.g. r010-abcdef...)
#   E2E_ZONE            — VPC zone (e.g. eu-de-1)
#   E2E_CIDR            — VPC subnet CIDR (e.g. 10.242.128.0/24)
#   E2E_VLAN_ID         — VLAN tag (e.g. 200)
#   E2E_SG_IDS          — Comma-separated security group IDs
#   E2E_ACL_ID          — Network ACL ID
#
# Optional:
#   E2E_NAMESPACE       — Test namespace (default: e2e-vpc-test)
#   E2E_CUDN_NAME       — CUDN name (default: e2e-localnet)
#   E2E_VM_NAME         — VM name (default: e2e-vm)
#   E2E_TIMEOUT         — Wait timeout in seconds (default: 300)
#   E2E_POLL_INTERVAL   — Poll interval in seconds (default: 5)
#   E2E_BM_NODE_COUNT   — Expected bare metal node count (default: 5)

set -euo pipefail

# ── Configuration ──

: "${E2E_VPC_ID:?E2E_VPC_ID must be set}"
: "${E2E_ZONE:?E2E_ZONE must be set}"
: "${E2E_CIDR:?E2E_CIDR must be set}"
: "${E2E_VLAN_ID:?E2E_VLAN_ID must be set}"
: "${E2E_SG_IDS:?E2E_SG_IDS must be set}"
: "${E2E_ACL_ID:?E2E_ACL_ID must be set}"

NAMESPACE="${E2E_NAMESPACE:-e2e-vpc-test}"
CUDN_NAME="${E2E_CUDN_NAME:-e2e-localnet}"
VM_NAME="${E2E_VM_NAME:-e2e-vm}"
TIMEOUT="${E2E_TIMEOUT:-300}"
POLL="${E2E_POLL_INTERVAL:-5}"
BM_NODE_COUNT="${E2E_BM_NODE_COUNT:-5}"

OC="oc"
PASSED=0
FAILED=0
START_TIME=$(date +%s)

# ── Helpers ──

log_step() {
  local step="$1"
  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  STEP: $step"
  echo "════════════════════════════════════════════════════════════════"
}

log_info() {
  echo "  [INFO] $*"
}

log_pass() {
  echo "  [PASS] $*"
  PASSED=$((PASSED + 1))
}

log_fail() {
  echo "  [FAIL] $*"
  FAILED=$((FAILED + 1))
}

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    log_pass "$desc (expected=$expected)"
  else
    log_fail "$desc (expected=$expected, got=$actual)"
  fi
}

assert_not_empty() {
  local desc="$1" value="$2"
  if [[ -n "$value" ]]; then
    log_pass "$desc (value=$value)"
  else
    log_fail "$desc (value is empty)"
  fi
}

assert_ge() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$actual" -ge "$expected" ]]; then
    log_pass "$desc (expected>=$expected, got=$actual)"
  else
    log_fail "$desc (expected>=$expected, got=$actual)"
  fi
}

# Wait for an annotation to appear on a resource.
# Usage: wait_for_annotation <resource-type> <name> <annotation-key> [namespace]
wait_for_annotation() {
  local resource="$1" name="$2" key="$3" ns="${4:-}"
  local ns_flag=""
  [[ -n "$ns" ]] && ns_flag="-n $ns"

  local elapsed=0
  while [[ $elapsed -lt $TIMEOUT ]]; do
    local value
    value=$($OC get "$resource" "$name" $ns_flag -o jsonpath="{.metadata.annotations['${key//\//\\/}']}" 2>/dev/null || echo "")
    if [[ -n "$value" ]]; then
      echo "$value"
      return 0
    fi
    sleep "$POLL"
    elapsed=$((elapsed + POLL))
  done
  echo ""
  return 1
}

# Wait for a resource to be deleted.
# Usage: wait_for_resource_deleted <resource-type> <name> [namespace]
wait_for_resource_deleted() {
  local resource="$1" name="$2" ns="${3:-}"
  local ns_flag=""
  [[ -n "$ns" ]] && ns_flag="-n $ns"

  local elapsed=0
  while [[ $elapsed -lt $TIMEOUT ]]; do
    if ! $OC get "$resource" "$name" $ns_flag &>/dev/null; then
      return 0
    fi
    sleep "$POLL"
    elapsed=$((elapsed + POLL))
  done
  return 1
}

# Wait for CRD resource count to match.
# Usage: wait_for_crd_count <resource-type> <label-selector> <expected-count>
wait_for_crd_count() {
  local resource="$1" selector="$2" expected="$3"
  local elapsed=0
  while [[ $elapsed -lt $TIMEOUT ]]; do
    local count
    count=$($OC get "$resource" -A -l "$selector" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$count" -ge "$expected" ]]; then
      echo "$count"
      return 0
    fi
    sleep "$POLL"
    elapsed=$((elapsed + POLL))
  done
  local final_count
  final_count=$($OC get "$resource" -A -l "$selector" --no-headers 2>/dev/null | wc -l | tr -d ' ')
  echo "$final_count"
  return 1
}

# ── Cleanup trap ──

cleanup() {
  log_step "Cleanup"

  log_info "Deleting VM $VM_NAME (if exists)..."
  $OC delete vm "$VM_NAME" -n "$NAMESPACE" --ignore-not-found --timeout=60s 2>/dev/null || true

  log_info "Waiting for VNI cleanup..."
  sleep 10

  log_info "Deleting CUDN $CUDN_NAME (if exists)..."
  $OC delete clusteruserdefinednetwork "$CUDN_NAME" --ignore-not-found --timeout=120s 2>/dev/null || true

  log_info "Waiting for VPC resource cleanup..."
  sleep 15

  log_info "Deleting namespace $NAMESPACE (if exists)..."
  $OC delete namespace "$NAMESPACE" --ignore-not-found --timeout=60s 2>/dev/null || true

  local end_time
  end_time=$(date +%s)
  local duration=$((end_time - START_TIME))

  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "  E2E LIFECYCLE TEST RESULTS"
  echo "════════════════════════════════════════════════════════════════"
  echo "  Passed:   $PASSED"
  echo "  Failed:   $FAILED"
  echo "  Duration: ${duration}s"
  echo "════════════════════════════════════════════════════════════════"

  if [[ $FAILED -gt 0 ]]; then
    exit 1
  fi
}

trap cleanup EXIT

# ── Step 0: Preflight ──

log_step "0 — Preflight checks"

log_info "Checking oc connectivity..."
$OC whoami >/dev/null 2>&1 || { log_fail "oc not logged in"; exit 1; }
log_pass "oc authenticated as $($OC whoami)"

log_info "Checking operator pods..."
OPERATOR_PODS=$($OC get pods -n roks-vpc-network-operator -l app.kubernetes.io/name=roks-vpc-network-operator --no-headers 2>/dev/null | grep -c Running || echo 0)
assert_ge "Operator pods running" 1 "$OPERATOR_PODS"

BFF_PODS=$($OC get pods -n roks-vpc-network-operator -l app.kubernetes.io/component=bff --no-headers 2>/dev/null | grep -c Running || echo 0)
assert_ge "BFF pods running" 1 "$BFF_PODS"

log_info "Checking CRDs installed..."
for crd in vpcsubnets.vpc.roks.ibm.com virtualnetworkinterfaces.vpc.roks.ibm.com vlanattachments.vpc.roks.ibm.com floatingips.vpc.roks.ibm.com; do
  if $OC get crd "$crd" &>/dev/null; then
    log_pass "CRD $crd exists"
  else
    log_fail "CRD $crd not found"
  fi
done

log_info "Counting bare metal nodes..."
BM_NODES=$($OC get nodes -l node.kubernetes.io/instance-type=bx2-metal-96x384 --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$BM_NODES" -eq 0 ]]; then
  # Try alternative label
  BM_NODES=$($OC get nodes -l node-role.kubernetes.io/worker --no-headers 2>/dev/null | wc -l | tr -d ' ')
fi
assert_ge "Bare metal worker nodes" "$BM_NODE_COUNT" "$BM_NODES"

# ── Step 1: Create test namespace ──

log_step "1 — Create test namespace"

$OC create namespace "$NAMESPACE" --dry-run=client -o yaml | $OC apply -f -
log_pass "Namespace $NAMESPACE created"

# ── Step 2: Create LocalNet CUDN ──

log_step "2 — Create LocalNet CUDN with VPC annotations"

cat <<EOF | $OC apply -f -
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ${CUDN_NAME}
  annotations:
    vpc.roks.ibm.com/vpc-id: "${E2E_VPC_ID}"
    vpc.roks.ibm.com/zone: "${E2E_ZONE}"
    vpc.roks.ibm.com/cidr: "${E2E_CIDR}"
    vpc.roks.ibm.com/vlan-id: "${E2E_VLAN_ID}"
    vpc.roks.ibm.com/security-group-ids: "${E2E_SG_IDS}"
    vpc.roks.ibm.com/acl-id: "${E2E_ACL_ID}"
spec:
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: ${NAMESPACE}
  network:
    topology: LocalNet
    localNet:
      role: Primary
EOF

log_pass "CUDN $CUDN_NAME created"

# ── Step 3: Verify VPC subnet created ──

log_step "3 — Verify VPC subnet created (waiting for subnet-id annotation)"

SUBNET_ID=$(wait_for_annotation "clusteruserdefinednetwork" "$CUDN_NAME" "vpc.roks.ibm.com/subnet-id")
if [[ -n "$SUBNET_ID" ]]; then
  log_pass "VPC subnet created: $SUBNET_ID"
else
  log_fail "Timed out waiting for subnet-id annotation on CUDN $CUDN_NAME"
fi

# Check VPCSubnet CRD
log_info "Checking VPCSubnet CRD..."
VSN_COUNT=$($OC get vpcsubnet -A --no-headers 2>/dev/null | grep -c "$CUDN_NAME" || echo 0)
assert_ge "VPCSubnet CRDs for $CUDN_NAME" 1 "$VSN_COUNT"

# ── Step 4: Verify VLAN attachments ──

log_step "4 — Verify VLAN attachments created on bare metal nodes"

VLAN_ATTACHMENTS=$(wait_for_annotation "clusteruserdefinednetwork" "$CUDN_NAME" "vpc.roks.ibm.com/vlan-attachments")
if [[ -n "$VLAN_ATTACHMENTS" ]]; then
  # Count comma-separated entries
  ATTACHMENT_COUNT=$(echo "$VLAN_ATTACHMENTS" | tr ',' '\n' | wc -l | tr -d ' ')
  log_pass "VLAN attachments annotation set ($ATTACHMENT_COUNT entries)"
  assert_ge "VLAN attachment count >= node count" "$BM_NODE_COUNT" "$ATTACHMENT_COUNT"
else
  log_fail "Timed out waiting for vlan-attachments annotation on CUDN $CUDN_NAME"
fi

# Check VLANAttachment CRDs
log_info "Checking VLANAttachment CRDs..."
VLA_COUNT=$($OC get vlanattachment -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
assert_ge "VLANAttachment CRDs total" "$BM_NODE_COUNT" "$VLA_COUNT"

# ── Step 5: Verify CRD overview ──

log_step "5 — Verify CRD resources"

log_info "VPCSubnets:"
$OC get vpcsubnet -A 2>/dev/null || log_info "(none)"

log_info "VLANAttachments:"
$OC get vlanattachment -A 2>/dev/null || log_info "(none)"

# ── Step 6: Create VM ──

log_step "6 — Create VirtualMachine referencing CUDN"

# Generate the network-attachment name from the CUDN
# For CUDN "e2e-localnet", the NAD is typically in the test namespace
NAD_NAME="${CUDN_NAME}"

cat <<EOF | $OC apply -f -
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: ${VM_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: e2e-test
spec:
  running: false
  template:
    metadata:
      labels:
        app: e2e-test
    spec:
      domain:
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
          interfaces:
          - name: localnet
            bridge: {}
        resources:
          requests:
            memory: 128Mi
      networks:
      - name: localnet
        multus:
          networkName: ${NAMESPACE}/${NAD_NAME}
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo:latest
EOF

log_pass "VM $VM_NAME created in namespace $NAMESPACE"

# ── Step 7: Verify webhook mutations ──

log_step "7 — Verify webhook injected VNI annotations"

VNI_ID=$(wait_for_annotation "vm" "$VM_NAME" "vpc.roks.ibm.com/vni-id" "$NAMESPACE")
assert_not_empty "VNI ID annotation set on VM" "$VNI_ID"

MAC_ADDR=$($OC get vm "$VM_NAME" -n "$NAMESPACE" -o jsonpath="{.metadata.annotations['vpc\.roks\.ibm\.com/mac-address']}" 2>/dev/null || echo "")
assert_not_empty "MAC address annotation set on VM" "$MAC_ADDR"

RESERVED_IP=$($OC get vm "$VM_NAME" -n "$NAMESPACE" -o jsonpath="{.metadata.annotations['vpc\.roks\.ibm\.com/reserved-ip']}" 2>/dev/null || echo "")
assert_not_empty "Reserved IP annotation set on VM" "$RESERVED_IP"

# ── Step 8: Verify VNI CRD ──

log_step "8 — Verify VirtualNetworkInterface CRD"

VNI_COUNT=$($OC get virtualnetworkinterface -A --no-headers 2>/dev/null | grep -c "$VM_NAME" || echo 0)
assert_ge "VNI CRDs for VM $VM_NAME" 1 "$VNI_COUNT"

if [[ "$VNI_COUNT" -ge 1 ]]; then
  VNI_CRD_NAME=$($OC get virtualnetworkinterface -A --no-headers 2>/dev/null | grep "$VM_NAME" | awk '{print $2}' | head -1)
  VNI_STATUS=$($OC get virtualnetworkinterface "$VNI_CRD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.syncStatus}' 2>/dev/null || echo "")
  assert_eq "VNI sync status" "Synced" "$VNI_STATUS"
fi

# ── Step 9: Delete VM and verify VNI cleanup ──

log_step "9 — Delete VM and verify VNI cleanup"

$OC delete vm "$VM_NAME" -n "$NAMESPACE" --timeout=120s
log_info "VM deleted, waiting for VNI cleanup..."

if wait_for_resource_deleted "virtualnetworkinterface" "${VNI_CRD_NAME:-none}" "$NAMESPACE" 2>/dev/null; then
  log_pass "VNI CRD cleaned up after VM deletion"
else
  VNI_REMAINING=$($OC get virtualnetworkinterface -A --no-headers 2>/dev/null | grep -c "$VM_NAME" || echo 0)
  if [[ "$VNI_REMAINING" -eq 0 ]]; then
    log_pass "VNI CRD cleaned up after VM deletion"
  else
    log_fail "VNI CRD still exists after VM deletion"
  fi
fi

# ── Step 10: Delete CUDN and verify VPC resource cleanup ──

log_step "10 — Delete CUDN and verify VPC resource cleanup"

$OC delete clusteruserdefinednetwork "$CUDN_NAME" --timeout=120s
log_info "CUDN deleted, waiting for VLAN attachment + subnet cleanup..."

sleep 15

VLA_REMAINING=$($OC get vlanattachment -A --no-headers 2>/dev/null | grep -c "$CUDN_NAME" || echo 0)
assert_eq "VLAN attachments remaining after CUDN delete" "0" "$VLA_REMAINING"

VSN_REMAINING=$($OC get vpcsubnet -A --no-headers 2>/dev/null | grep -c "$CUDN_NAME" || echo 0)
assert_eq "VPC subnets remaining after CUDN delete" "0" "$VSN_REMAINING"

# ── Step 11: Cleanup namespace (done in trap) ──

log_step "11 — Test complete (cleanup in trap)"
log_info "All lifecycle steps executed. See results summary below."

# Disable the cleanup trap — we already cleaned up the VM and CUDN above
# The trap will just remove the namespace and print results
