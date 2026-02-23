import { EdgeModel, EdgeTerminalType } from '@patternfly/react-topology';

export const EDGE_TYPES = {
  CONTAINMENT: 'containment',
  MEMBERSHIP: 'membership',
  BINDING: 'binding',
  ASSOCIATION: 'association',
  MANAGEMENT: 'management',
} as const;

export type EdgeType = typeof EDGE_TYPES[keyof typeof EDGE_TYPES];

export interface EdgeConfig {
  id: string;
  source: string;
  target: string;
  edgeType: EdgeType;
  label?: string;
  animated?: boolean;
}

/**
 * Edge styling configuration based on edge type
 */
const EDGE_STYLES: Record<EdgeType, Partial<EdgeModel>> = {
  [EDGE_TYPES.CONTAINMENT]: {
    style: {
      strokeWidth: 2,
      stroke: '#0066CC',
    } as any,
  },
  [EDGE_TYPES.MEMBERSHIP]: {
    style: {
      strokeWidth: 2,
      stroke: '#00A651',
    } as any,
  },
  [EDGE_TYPES.BINDING]: {
    style: {
      strokeWidth: 2,
      stroke: '#FF6D00',
    } as any,
  },
  [EDGE_TYPES.ASSOCIATION]: {
    style: {
      strokeWidth: 2,
      stroke: '#9900CC',
      strokeDasharray: '5,5',
    } as any,
  },
  [EDGE_TYPES.MANAGEMENT]: {
    style: {
      strokeWidth: 1.5,
      stroke: '#666666',
      strokeDasharray: '3,3',
    } as any,
  },
};

/**
 * Create an edge model from configuration
 */
export const createEdgeModel = (config: EdgeConfig): EdgeModel => {
  const baseStyle = EDGE_STYLES[config.edgeType];

  const model: EdgeModel = {
    id: config.id,
    type: config.edgeType,
    source: config.source,
    target: config.target,
    label: config.label,
    data: { animated: config.animated ?? false },
    ...baseStyle,
  };

  return model;
};

/**
 * Get description for edge type
 */
export const getEdgeTypeDescription = (edgeType: EdgeType): string => {
  const descriptions: Record<EdgeType, string> = {
    [EDGE_TYPES.CONTAINMENT]: 'VPC contains Subnet',
    [EDGE_TYPES.MEMBERSHIP]: 'Subnet contains VNI',
    [EDGE_TYPES.BINDING]: 'VNI bound to VM',
    [EDGE_TYPES.ASSOCIATION]: 'Resource associated with SG/ACL',
    [EDGE_TYPES.MANAGEMENT]: 'Management relationship',
  };

  return descriptions[edgeType];
};

/**
 * Determine edge type based on source and target node types
 */
export const determineEdgeType = (
  sourceNodeType: string,
  targetNodeType: string
): EdgeType => {
  const key = `${sourceNodeType}:${targetNodeType}`;

  const edgeTypeMap: Record<string, EdgeType> = {
    'vpc:subnet': EDGE_TYPES.CONTAINMENT,
    'subnet:vni': EDGE_TYPES.MEMBERSHIP,
    'vni:vm': EDGE_TYPES.BINDING,
    'security-group:vni': EDGE_TYPES.ASSOCIATION,
    'security-group:vm': EDGE_TYPES.ASSOCIATION,
    'acl:subnet': EDGE_TYPES.ASSOCIATION,
  };

  return edgeTypeMap[key] || EDGE_TYPES.MANAGEMENT;
};

/**
 * Style configuration for different edge rendering
 */
export const EDGE_TERMINAL_STYLES = {
  [EDGE_TYPES.CONTAINMENT]: {
    startTerminalType: EdgeTerminalType.none,
    endTerminalType: EdgeTerminalType.none,
  },
  [EDGE_TYPES.MEMBERSHIP]: {
    startTerminalType: EdgeTerminalType.none,
    endTerminalType: EdgeTerminalType.none,
  },
  [EDGE_TYPES.BINDING]: {
    startTerminalType: EdgeTerminalType.none,
    endTerminalType: EdgeTerminalType.none,
  },
  [EDGE_TYPES.ASSOCIATION]: {
    startTerminalType: EdgeTerminalType.none,
    endTerminalType: EdgeTerminalType.circle,
  },
  [EDGE_TYPES.MANAGEMENT]: {
    startTerminalType: EdgeTerminalType.none,
    endTerminalType: EdgeTerminalType.none,
  },
} as const;
