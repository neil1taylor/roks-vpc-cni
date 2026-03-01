---
name: vpc-resources
description: List all VPC resources managed by the operator — subnets, VNIs, VLAN attachments, floating IPs, gateways, and routers with sync status
allowed-tools: Bash(kubectl *), Bash(oc *)
---

List all VPC resources managed by the VPC Network Operator and summarize their status.

```bash
echo "=== VPC Subnets ==="
kubectl get vsn -A -o wide 2>/dev/null || echo "No VPCSubnet resources found"

echo ""
echo "=== Virtual Network Interfaces ==="
kubectl get vni -A -o wide 2>/dev/null || echo "No VirtualNetworkInterface resources found"

echo ""
echo "=== VLAN Attachments ==="
kubectl get vla -A -o wide 2>/dev/null || echo "No VLANAttachment resources found"

echo ""
echo "=== Floating IPs ==="
kubectl get fip -A -o wide 2>/dev/null || echo "No FloatingIP resources found"

echo ""
echo "=== VPC Gateways ==="
kubectl get vgw -A -o wide 2>/dev/null || echo "No VPCGateway resources found"

echo ""
echo "=== VPC Routers ==="
kubectl get vrt -A -o wide 2>/dev/null || echo "No VPCRouter resources found"
```

After running all commands, provide a summary:

| Resource Type | Total | Ready | Failed | Pending |
|---------------|-------|-------|--------|---------|
| VPC Subnets | | | | |
| VNIs | | | | |
| VLAN Attachments | | | | |
| Floating IPs | | | | |
| VPC Gateways | | | | |
| VPC Routers | | | | |

If any resources are in Failed state, list them with their error message from `.status.conditions`.
