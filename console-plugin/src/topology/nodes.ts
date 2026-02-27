import {
  NodeModel,
  NodeShape,
  NodeStatus,
} from '@patternfly/react-topology';
import {
  GlobeRouteIcon,
  NetworkIcon,
  VirtualMachineIcon,
  ShieldAltIcon,
  CloudUploadAltIcon,
  ProjectDiagramIcon,
} from '@patternfly/react-icons';

export const NODE_TYPES = {
  VPC: 'vpc',
  SUBNET: 'subnet',
  VNI: 'vni',
  VM: 'vm',
  SECURITY_GROUP: 'security-group',
  ACL: 'network-acl',
  FLOATING_IP: 'floating-ip',
  CUDN: 'cudn',
  UDN: 'udn',
} as const;

export type NodeType = typeof NODE_TYPES[keyof typeof NODE_TYPES];

/** Short badge labels displayed on nodes */
export const NODE_BADGES: Record<NodeType, string> = {
  [NODE_TYPES.VPC]: 'VPC',
  [NODE_TYPES.SUBNET]: 'Subnet',
  [NODE_TYPES.VNI]: 'VNI',
  [NODE_TYPES.VM]: 'VM',
  [NODE_TYPES.SECURITY_GROUP]: 'SG',
  [NODE_TYPES.ACL]: 'ACL',
  [NODE_TYPES.FLOATING_IP]: 'FIP',
  [NODE_TYPES.CUDN]: 'CUDN',
  [NODE_TYPES.UDN]: 'UDN',
};

export interface CustomNodeData {
  id: string;
  label: string;
  nodeType: NodeType;
  resourceId?: string;
  icon?: React.ComponentType<any>;
  color?: string;
  badge?: string;
  badgeColor?: string;
  status?: NodeStatus;
  details?: Record<string, string>;
}

export const NODE_COLORS: Record<NodeType, string> = {
  [NODE_TYPES.VPC]: '#0066CC',
  [NODE_TYPES.SUBNET]: '#00A651',
  [NODE_TYPES.VNI]: '#9900CC',
  [NODE_TYPES.VM]: '#FF6D00',
  [NODE_TYPES.SECURITY_GROUP]: '#DC143C',
  [NODE_TYPES.ACL]: '#FFD700',
  [NODE_TYPES.FLOATING_IP]: '#00BCD4',
  [NODE_TYPES.CUDN]: '#2196F3',
  [NODE_TYPES.UDN]: '#4CAF50',
};

export const NODE_ICONS: Record<NodeType, React.ComponentType<any>> = {
  [NODE_TYPES.VPC]: GlobeRouteIcon,
  [NODE_TYPES.SUBNET]: NetworkIcon,
  [NODE_TYPES.VNI]: NetworkIcon,
  [NODE_TYPES.VM]: VirtualMachineIcon,
  [NODE_TYPES.SECURITY_GROUP]: ShieldAltIcon,
  [NODE_TYPES.ACL]: ShieldAltIcon,
  [NODE_TYPES.FLOATING_IP]: CloudUploadAltIcon,
  [NODE_TYPES.CUDN]: ProjectDiagramIcon,
  [NODE_TYPES.UDN]: ProjectDiagramIcon,
};

export const NODE_SHAPES: Record<NodeType, NodeShape> = {
  [NODE_TYPES.VPC]: NodeShape.rect,
  [NODE_TYPES.SUBNET]: NodeShape.rect,
  [NODE_TYPES.VNI]: NodeShape.ellipse,
  [NODE_TYPES.VM]: NodeShape.rect,
  [NODE_TYPES.SECURITY_GROUP]: NodeShape.hexagon,
  [NODE_TYPES.ACL]: NodeShape.octagon,
  [NODE_TYPES.FLOATING_IP]: NodeShape.stadium,
  [NODE_TYPES.CUDN]: NodeShape.trapezoid,
  [NODE_TYPES.UDN]: NodeShape.trapezoid,
};

/** Map BFF status strings to PatternFly NodeStatus enum values */
export const mapBFFStatus = (status?: string): NodeStatus | undefined => {
  switch (status) {
    case 'available':
      return NodeStatus.success;
    case 'error':
      return NodeStatus.danger;
    case 'pending':
      return NodeStatus.warning;
    default:
      return undefined;
  }
};

export interface NodeConfig {
  id: string;
  label: string;
  nodeType: NodeType;
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  resourceId?: string;
  status?: NodeStatus;
  details?: Record<string, string>;
  children?: string[];
  parent?: string;
  group?: boolean;
}

/**
 * Create a node model from configuration
 */
export const createNodeModel = (config: NodeConfig): NodeModel => {
  const nodeType = config.nodeType;
  const shape = NODE_SHAPES[nodeType];
  const icon = NODE_ICONS[nodeType];

  const nodeData: CustomNodeData = {
    id: config.id,
    label: config.label,
    nodeType,
    resourceId: config.resourceId,
    icon,
    color: NODE_COLORS[nodeType],
    badge: NODE_BADGES[nodeType],
    badgeColor: NODE_COLORS[nodeType],
    status: config.status,
    details: config.details,
  };

  const model: NodeModel = {
    id: config.id,
    type: nodeType,
    label: config.label,
    x: config.x ?? 0,
    y: config.y ?? 0,
    width: config.width ?? 60,
    height: config.height ?? 60,
    shape,
    data: nodeData,
    children: config.children,
    group: config.group ?? false,
  };

  return model;
};

/**
 * Create a group node (container) for hierarchical topology
 */
export const createGroupNode = (config: Omit<NodeConfig, 'group'>): NodeModel => {
  const nodeType = config.nodeType;
  const model: NodeModel = {
    id: config.id,
    type: `${nodeType}-group`,
    label: config.label,
    x: config.x ?? 0,
    y: config.y ?? 0,
    width: config.width ?? 400,
    height: config.height ?? 300,
    shape: NodeShape.rect,
    data: {
      id: config.id,
      label: config.label,
      nodeType,
      resourceId: config.resourceId,
      icon: NODE_ICONS[nodeType],
      color: NODE_COLORS[nodeType],
      badge: NODE_BADGES[nodeType],
      badgeColor: NODE_COLORS[nodeType],
      status: config.status,
      details: config.details,
    },
    children: config.children,
    group: true,
    collapsed: false,
  };

  return model;
};

/**
 * Get display label for a node type
 */
export const getNodeTypeLabel = (nodeType: NodeType): string => {
  const labels: Record<NodeType, string> = {
    [NODE_TYPES.VPC]: 'VPC',
    [NODE_TYPES.SUBNET]: 'Subnet',
    [NODE_TYPES.VNI]: 'Virtual Network Interface',
    [NODE_TYPES.VM]: 'Virtual Machine',
    [NODE_TYPES.SECURITY_GROUP]: 'Security Group',
    [NODE_TYPES.ACL]: 'Network ACL',
    [NODE_TYPES.FLOATING_IP]: 'Floating IP',
    [NODE_TYPES.CUDN]: 'Cluster Network',
    [NODE_TYPES.UDN]: 'Namespace Network',
  };

  return labels[nodeType];
};
