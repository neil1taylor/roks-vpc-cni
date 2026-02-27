import { EdgeModel } from '@patternfly/react-topology';

/**
 * BFF edge types. These match the `type` field returned by GET /api/v1/topology.
 */
export const BFF_EDGE_TYPES = {
  CONTAINS: 'contains',       // VPC→Subnet, VPC→SG, VPC→ACL (structural)
  CONNECTED: 'connected',     // Subnet→VNI (structural)
  TARGETS: 'targets',         // FIP→VNI (visible)
  ASSOCIATES: 'associates',   // CUDN/UDN→Subnet (visible, dashed)
  PROTECTED_BY: 'protected-by', // Subnet→ACL, VNI→SG (visible)
} as const;

export type BFFEdgeType = typeof BFF_EDGE_TYPES[keyof typeof BFF_EDGE_TYPES];

/** Structural edges build parent-child hierarchy; they are NOT rendered as visible edges. */
export const STRUCTURAL_EDGE_TYPES = new Set<string>([
  BFF_EDGE_TYPES.CONTAINS,
  BFF_EDGE_TYPES.CONNECTED,
]);

/** Visible edges are rendered as lines/arrows on the topology diagram. */
export const VISIBLE_EDGE_TYPES = new Set<string>([
  BFF_EDGE_TYPES.TARGETS,
  BFF_EDGE_TYPES.ASSOCIATES,
  BFF_EDGE_TYPES.PROTECTED_BY,
]);

/** Edge styling by type */
const EDGE_STYLES: Record<string, { stroke: string; strokeWidth: number; strokeDasharray?: string }> = {
  [BFF_EDGE_TYPES.CONTAINS]: { stroke: '#4a6785', strokeWidth: 1.5 },
  [BFF_EDGE_TYPES.TARGETS]: { stroke: '#00BCD4', strokeWidth: 2 },
  [BFF_EDGE_TYPES.ASSOCIATES]: { stroke: '#2196F3', strokeWidth: 2, strokeDasharray: '6,3' },
  [BFF_EDGE_TYPES.PROTECTED_BY]: { stroke: '#DC143C', strokeWidth: 1.5, strokeDasharray: '3,3' },
};

/**
 * Create an EdgeModel for a visible edge.
 */
export const createVisibleEdge = (
  id: string,
  source: string,
  target: string,
  edgeType: string
): EdgeModel => {
  const style = EDGE_STYLES[edgeType] ?? { stroke: '#666', strokeWidth: 1.5 };

  return {
    id,
    type: 'edge',
    source,
    target,
    style: style as any,
    data: { edgeType },
  };
};
