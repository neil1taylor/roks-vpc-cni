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
} from '@patternfly/react-icons';

export const NODE_TYPES = {
  VPC: 'vpc',
  SUBNET: 'subnet',
  VNI: 'vni',
  VM: 'vm',
  SECURITY_GROUP: 'security-group',
  ACL: 'acl',
} as const;

export type NodeType = typeof NODE_TYPES[keyof typeof NODE_TYPES];

export interface CustomNodeData {
  id: string;
  label: string;
  nodeType: NodeType;
  resourceId?: string;
  icon?: React.ComponentType<any>;
  color?: string;
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
};

export const NODE_ICONS: Record<NodeType, React.ComponentType<any>> = {
  [NODE_TYPES.VPC]: GlobeRouteIcon,
  [NODE_TYPES.SUBNET]: NetworkIcon,
  [NODE_TYPES.VNI]: NetworkIcon,
  [NODE_TYPES.VM]: VirtualMachineIcon,
  [NODE_TYPES.SECURITY_GROUP]: ShieldAltIcon,
  [NODE_TYPES.ACL]: ShieldAltIcon,
};

export const NODE_SHAPES: Record<NodeType, NodeShape> = {
  [NODE_TYPES.VPC]: NodeShape.circle,
  [NODE_TYPES.SUBNET]: NodeShape.circle,
  [NODE_TYPES.VNI]: NodeShape.circle,
  [NODE_TYPES.VM]: NodeShape.circle,
  [NODE_TYPES.SECURITY_GROUP]: NodeShape.rect,
  [NODE_TYPES.ACL]: NodeShape.rect,
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
    type: nodeType,
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
      details: config.details,
    },
    children: config.children,
    group: true,
    collapsed: false,
  };

  return model;
};

/**
 * Calculate initial node positions based on node type
 */
export const calculateNodePosition = (
  _nodeType: NodeType,
  index: number,
  containerWidth: number = 800,
  containerHeight: number = 600
): { x: number; y: number } => {
  const margin = 50;
  const nodeWidth = containerWidth - 2 * margin;
  const nodeHeight = containerHeight - 2 * margin;

  // Position nodes in a grid based on type
  const rows = Math.ceil(Math.sqrt(index + 1));
  const cols = Math.ceil((index + 1) / rows);

  const x = margin + ((index % cols) * nodeWidth) / cols + nodeWidth / (cols * 2);
  const y = margin + (Math.floor(index / cols) * nodeHeight) / rows + nodeHeight / (rows * 2);

  return { x, y };
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
  };

  return labels[nodeType];
};
