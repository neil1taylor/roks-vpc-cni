import { k8sCreate, k8sUpdate, k8sDelete, k8sGet, k8sList } from '@openshift-console/dynamic-plugin-sdk';
import {
  VPCSubnet,
  VirtualNetworkInterfaceResource,
  VLANAttachmentResource,
  FloatingIPResource,
} from './types';

// Model definitions for Kubernetes resources
export const VPCSubnetModel = {
  apiVersion: 'vpc.roks.ibm.com/v1alpha1',
  apiGroup: 'vpc.roks.ibm.com',
  kind: 'VPCSubnet',
  plural: 'vpcsubnets',
  label: 'VPC Subnet',
  labelPlural: 'VPC Subnets',
  abbr: 'VPS',
};

export const VirtualNetworkInterfaceModel = {
  apiVersion: 'vpc.roks.ibm.com/v1alpha1',
  apiGroup: 'vpc.roks.ibm.com',
  kind: 'VirtualNetworkInterface',
  plural: 'virtualnetworkinterfaces',
  label: 'Virtual Network Interface',
  labelPlural: 'Virtual Network Interfaces',
  abbr: 'VNI',
};

export const VLANAttachmentModel = {
  apiVersion: 'vpc.roks.ibm.com/v1alpha1',
  apiGroup: 'vpc.roks.ibm.com',
  kind: 'VLANAttachment',
  plural: 'vlanattachments',
  label: 'VLAN Attachment',
  labelPlural: 'VLAN Attachments',
  abbr: 'VLAN',
};

export const FloatingIPModel = {
  apiVersion: 'vpc.roks.ibm.com/v1alpha1',
  apiGroup: 'vpc.roks.ibm.com',
  kind: 'FloatingIP',
  plural: 'floatingips',
  label: 'Floating IP',
  labelPlural: 'Floating IPs',
  abbr: 'FIP',
};

// VPCSubnet operations
export async function createVPCSubnet(namespace: string, subnet: VPCSubnet): Promise<VPCSubnet> {
  const resource = {
    apiVersion: VPCSubnetModel.apiVersion,
    kind: VPCSubnetModel.kind,
    metadata: {
      name: subnet.name,
      namespace,
      ...(subnet.tags ? { labels: { tags: subnet.tags.join(',') } } : {}),
    },
    spec: subnet.spec,
  };
  return k8sCreate({ model: VPCSubnetModel, data: resource }) as unknown as Promise<VPCSubnet>;
}

export async function getVPCSubnet(namespace: string, name: string): Promise<VPCSubnet> {
  return k8sGet({ model: VPCSubnetModel, ns: namespace, name }) as Promise<VPCSubnet>;
}

export async function listVPCSubnets(namespace?: string): Promise<VPCSubnet[]> {
  const resources = await k8sList({
    model: VPCSubnetModel,
    queryParams: namespace ? { ns: namespace } : {},
  });
  return ((resources as any)?.items || resources || []) as VPCSubnet[];
}

export async function updateVPCSubnet(namespace: string, subnet: VPCSubnet): Promise<VPCSubnet> {
  return k8sUpdate({ model: VPCSubnetModel, data: subnet, ns: namespace }) as Promise<VPCSubnet>;
}

export async function deleteVPCSubnet(namespace: string, name: string): Promise<void> {
  await k8sDelete({
    model: VPCSubnetModel,
    resource: { metadata: { name, namespace } } as any,
  });
}

// VirtualNetworkInterface operations
export async function createVNI(namespace: string, vni: VirtualNetworkInterfaceResource): Promise<VirtualNetworkInterfaceResource> {
  const resource = {
    apiVersion: VirtualNetworkInterfaceModel.apiVersion,
    kind: VirtualNetworkInterfaceModel.kind,
    metadata: {
      name: vni.name,
      namespace,
      ...(vni.tags ? { labels: { tags: vni.tags.join(',') } } : {}),
    },
    spec: vni.spec,
  };
  return k8sCreate({ model: VirtualNetworkInterfaceModel, data: resource }) as unknown as Promise<VirtualNetworkInterfaceResource>;
}

export async function getVNI(namespace: string, name: string): Promise<VirtualNetworkInterfaceResource> {
  return k8sGet({ model: VirtualNetworkInterfaceModel, ns: namespace, name }) as Promise<VirtualNetworkInterfaceResource>;
}

export async function listVNIs(namespace?: string): Promise<VirtualNetworkInterfaceResource[]> {
  const resources = await k8sList({
    model: VirtualNetworkInterfaceModel,
    queryParams: namespace ? { ns: namespace } : {},
  });
  return ((resources as any)?.items || resources || []) as VirtualNetworkInterfaceResource[];
}

export async function updateVNI(namespace: string, vni: VirtualNetworkInterfaceResource): Promise<VirtualNetworkInterfaceResource> {
  return k8sUpdate({ model: VirtualNetworkInterfaceModel, data: vni, ns: namespace }) as Promise<VirtualNetworkInterfaceResource>;
}

export async function deleteVNI(namespace: string, name: string): Promise<void> {
  await k8sDelete({
    model: VirtualNetworkInterfaceModel,
    resource: { metadata: { name, namespace } } as any,
  });
}

// VLANAttachment operations
export async function createVLANAttachment(namespace: string, attachment: VLANAttachmentResource): Promise<VLANAttachmentResource> {
  const resource = {
    apiVersion: VLANAttachmentModel.apiVersion,
    kind: VLANAttachmentModel.kind,
    metadata: {
      name: attachment.name,
      namespace,
    },
    spec: attachment.spec,
  };
  return k8sCreate({ model: VLANAttachmentModel, data: resource }) as unknown as Promise<VLANAttachmentResource>;
}

export async function getVLANAttachment(namespace: string, name: string): Promise<VLANAttachmentResource> {
  return k8sGet({ model: VLANAttachmentModel, ns: namespace, name }) as Promise<VLANAttachmentResource>;
}

export async function listVLANAttachments(namespace?: string): Promise<VLANAttachmentResource[]> {
  const resources = await k8sList({
    model: VLANAttachmentModel,
    queryParams: namespace ? { ns: namespace } : {},
  });
  return ((resources as any)?.items || resources || []) as VLANAttachmentResource[];
}

export async function updateVLANAttachment(namespace: string, attachment: VLANAttachmentResource): Promise<VLANAttachmentResource> {
  return k8sUpdate({ model: VLANAttachmentModel, data: attachment, ns: namespace }) as Promise<VLANAttachmentResource>;
}

export async function deleteVLANAttachment(namespace: string, name: string): Promise<void> {
  await k8sDelete({
    model: VLANAttachmentModel,
    resource: { metadata: { name, namespace } } as any,
  });
}

// FloatingIP operations
export async function createFloatingIP(namespace: string, fip: FloatingIPResource): Promise<FloatingIPResource> {
  const resource = {
    apiVersion: FloatingIPModel.apiVersion,
    kind: FloatingIPModel.kind,
    metadata: {
      name: fip.name,
      namespace,
    },
    spec: fip.spec,
  };
  return k8sCreate({ model: FloatingIPModel, data: resource }) as unknown as Promise<FloatingIPResource>;
}

export async function getFloatingIP(namespace: string, name: string): Promise<FloatingIPResource> {
  return k8sGet({ model: FloatingIPModel, ns: namespace, name }) as Promise<FloatingIPResource>;
}

export async function listFloatingIPs(namespace?: string): Promise<FloatingIPResource[]> {
  const resources = await k8sList({
    model: FloatingIPModel,
    queryParams: namespace ? { ns: namespace } : {},
  });
  return ((resources as any)?.items || resources || []) as FloatingIPResource[];
}

export async function updateFloatingIP(namespace: string, fip: FloatingIPResource): Promise<FloatingIPResource> {
  return k8sUpdate({ model: FloatingIPModel, data: fip, ns: namespace }) as Promise<FloatingIPResource>;
}

export async function deleteFloatingIP(namespace: string, name: string): Promise<void> {
  await k8sDelete({
    model: FloatingIPModel,
    resource: { metadata: { name, namespace } } as any,
  });
}
