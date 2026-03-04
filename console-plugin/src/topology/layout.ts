import {
  NodeModel,
  EdgeModel,
  Model,
} from '@patternfly/react-topology';
import { createNodeModel, createGroupNode, NODE_TYPES, mapBFFStatus, HealthStatus } from './nodes';
import { BFF_EDGE_TYPES, STRUCTURAL_EDGE_TYPES, createVisibleEdge } from './edges';

export interface NodeHealthData {
  status: HealthStatus;
  metrics?: Record<string, number>;
}

/**
 * Topology data structure from BFF API
 */
export interface TopologyNode {
  id: string;
  name: string;
  type: string;
  status?: string;
  resourceId?: string;
  details?: Record<string, string>;
  health?: NodeHealthData;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  type?: string;
}

export interface TopologyData {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

/**
 * Normalise the BFF node type to one of our NODE_TYPES values.
 * CUDNs and UDNs come from the BFF with type "subnet" but metadata.resource_type set.
 */
const normalizeNodeType = (node: TopologyNode): string => {
  if (node.details?.resource_type === 'cudn') return NODE_TYPES.CUDN;
  if (node.details?.resource_type === 'udn') return NODE_TYPES.UDN;
  // Map BFF types to our types
  switch (node.type) {
    case 'vpc': return NODE_TYPES.VPC;
    case 'subnet': return NODE_TYPES.SUBNET;
    case 'vni': return NODE_TYPES.VNI;
    case 'vm': return NODE_TYPES.VM;
    case 'security-group': return NODE_TYPES.SECURITY_GROUP;
    case 'network-acl': return NODE_TYPES.ACL;
    case 'floating-ip': return NODE_TYPES.FLOATING_IP;
    default: return node.type;
  }
};

/**
 * Transform BFF TopologyData into PatternFly graph nodes and edges.
 *
 * Layout strategy (Cola force-directed with groups):
 *   - `contains` and `connected` edges are STRUCTURAL: build parent→child groups
 *     (VPC groups containing SGs/ACLs/Subnets; Subnet groups containing VNIs)
 *   - `targets` (FIP→VNI): co-locate FIP into the VNI's subnet group + visible edge
 *   - `associates` / `protected-by`: visible edges only (no grouping)
 *
 * Cola naturally handles disconnected subgraphs (multiple VPCs) by pushing
 * groups apart and filling the viewport space.
 */
export const transformTopologyData = (
  data: TopologyData
): { nodes: NodeModel[]; edges: EdgeModel[] } => {
  // Lookup maps
  const nodeById = new Map<string, TopologyNode>();
  const resolvedType = new Map<string, string>();
  data.nodes.forEach((n) => {
    nodeById.set(n.id, n);
    resolvedType.set(n.id, normalizeNodeType(n));
  });

  // parent→children mapping (built from structural edges)
  const children = new Map<string, Set<string>>();
  const parentOf = new Map<string, string>();

  const addChild = (parentId: string, childId: string) => {
    if (!children.has(parentId)) children.set(parentId, new Set());
    children.get(parentId)!.add(childId);
    parentOf.set(childId, parentId);
  };

  const visibleEdges: EdgeModel[] = [];

  // ── Pass 1: structural edges → group hierarchy ──
  for (const edge of data.edges) {
    if (!STRUCTURAL_EDGE_TYPES.has(edge.type ?? '')) continue;
    if (!nodeById.has(edge.source) || !nodeById.has(edge.target)) continue;
    addChild(edge.source, edge.target);
  }

  // ── Pass 2: visible edges + co-location ──
  for (const edge of data.edges) {
    if (STRUCTURAL_EDGE_TYPES.has(edge.type ?? '')) continue;
    if (!nodeById.has(edge.source) || !nodeById.has(edge.target)) continue;

    const sourceType = resolvedType.get(edge.source)!;
    const targetType = resolvedType.get(edge.target)!;

    // targets: FIP→VNI — place FIP in the same parent as the VNI
    if (edge.type === BFF_EDGE_TYPES.TARGETS &&
        sourceType === NODE_TYPES.FLOATING_IP && targetType === NODE_TYPES.VNI) {
      const vniParent = parentOf.get(edge.target);
      if (vniParent && !parentOf.has(edge.source)) {
        addChild(vniParent, edge.source);
      }
    }

    visibleEdges.push(
      createVisibleEdge(edge.id, edge.source, edge.target, edge.type ?? 'targets')
    );
  }

  // ── Build NodeModels ──
  const models: NodeModel[] = [];

  for (const node of data.nodes) {
    const type = resolvedType.get(node.id)!;
    const status = mapBFFStatus(node.status);
    const healthStatus = node.health?.status as HealthStatus | undefined;
    const childSet = children.get(node.id);
    const childIds = childSet ? Array.from(childSet) : undefined;
    const isGroupable = type === NODE_TYPES.VPC || type === NODE_TYPES.SUBNET;

    if (isGroupable && childIds && childIds.length > 0) {
      models.push(
        createGroupNode({
          id: node.id,
          label: node.name,
          nodeType: type as any,
          resourceId: node.resourceId,
          status,
          details: node.details,
          children: childIds,
          healthStatus,
        })
      );
    } else {
      models.push(
        createNodeModel({
          id: node.id,
          label: node.name,
          nodeType: type as any,
          resourceId: node.resourceId,
          status,
          details: node.details,
          width: 75,
          height: 75,
          healthStatus,
        })
      );
    }
  }

  return { nodes: models, edges: visibleEdges };
};

/**
 * Create the full PatternFly Model with DagreGroups layout
 */
export const createTopologyGraph = (
  nodes: NodeModel[],
  edges: EdgeModel[]
): Model => {
  return {
    nodes,
    edges,
    graph: {
      id: 'vpc-topology',
      type: 'graph',
      layout: 'Cola',
    },
  };
};

/**
 * Find node by ID in topology data
 */
export const findTopologyNode = (
  data: TopologyData,
  nodeId: string
): TopologyNode | undefined => {
  return data.nodes.find((n) => n.id === nodeId);
};

/**
 * Get all connected nodes (direct and indirect)
 */
export const getConnectedNodes = (
  data: TopologyData,
  nodeId: string,
  direction: 'outbound' | 'inbound' | 'both' = 'both'
): string[] => {
  const connected = new Set<string>();
  const queue = [nodeId];
  const visited = new Set<string>();

  while (queue.length > 0) {
    const current = queue.shift();
    if (!current || visited.has(current)) {
      continue;
    }

    visited.add(current);

    if (direction === 'outbound' || direction === 'both') {
      data.edges
        .filter((e) => e.source === current)
        .forEach((e) => {
          connected.add(e.target);
          queue.push(e.target);
        });
    }

    if (direction === 'inbound' || direction === 'both') {
      data.edges
        .filter((e) => e.target === current)
        .forEach((e) => {
          connected.add(e.source);
          queue.push(e.source);
        });
    }
  }

  return Array.from(connected);
};

/**
 * Filter nodes by type
 */
export const filterNodesByType = (
  data: TopologyData,
  types: string[]
): TopologyNode[] => {
  return data.nodes.filter((node) => types.includes(node.type));
};

/**
 * Calculate topology statistics
 */
export interface TopologyStats {
  totalNodes: number;
  nodesByType: Record<string, number>;
  totalEdges: number;
  depth: number;
}

export const calculateTopologyStats = (data: TopologyData): TopologyStats => {
  const nodesByType: Record<string, number> = {};

  data.nodes.forEach((node) => {
    const type = normalizeNodeType(node);
    nodesByType[type] = (nodesByType[type] || 0) + 1;
  });

  return {
    totalNodes: data.nodes.length,
    nodesByType,
    totalEdges: data.edges.length,
    depth: 0, // Dagre computes depth internally
  };
};
