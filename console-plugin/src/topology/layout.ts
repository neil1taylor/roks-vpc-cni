import {
  NodeModel,
  EdgeModel,
  Model,
} from '@patternfly/react-topology';
import { createNodeModel, createGroupNode, NODE_TYPES } from './nodes';
import { createEdgeModel, determineEdgeType } from './edges';

/**
 * Topology data structure from API
 */
export interface TopologyNode {
  id: string;
  name: string;
  type: 'vpc' | 'subnet' | 'vni' | 'vm' | 'security-group' | 'acl';
  resourceId?: string;
  parentId?: string;
  details?: Record<string, string>;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
}

export interface TopologyData {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

/**
 * Transform TopologyData from API into PatternFly Graph model
 */
export const transformTopologyData = (data: TopologyData): { nodes: NodeModel[]; edges: EdgeModel[] } => {
  const nodeMap = new Map<string, NodeModel>();
  const edgeModels: EdgeModel[] = [];

  // Group nodes by parent for hierarchical layout
  const nodesByParent = new Map<string, TopologyNode[]>();
  const vpcNodes: TopologyNode[] = [];
  const topLevelNodes: TopologyNode[] = [];

  // First pass: organize nodes by parent
  data.nodes.forEach((node) => {
    if (node.type === 'vpc') {
      vpcNodes.push(node);
      if (!nodesByParent.has('root')) {
        nodesByParent.set('root', []);
      }
      nodesByParent.get('root')!.push(node);
    } else if (node.parentId) {
      if (!nodesByParent.has(node.parentId)) {
        nodesByParent.set(node.parentId, []);
      }
      nodesByParent.get(node.parentId)!.push(node);
    } else {
      topLevelNodes.push(node);
    }
  });

  // Create node models for VPCs (as groups)
  let vpcIndex = 0;
  vpcNodes.forEach((vpcNode) => {
    const childIds = nodesByParent.get(vpcNode.id)?.map((n) => n.id) || [];

    const groupModel = createGroupNode({
      id: vpcNode.id,
      label: vpcNode.name,
      nodeType: NODE_TYPES.VPC,
      resourceId: vpcNode.resourceId,
      details: vpcNode.details,
      children: childIds,
      width: 600,
      height: 400,
      x: vpcIndex * 650,
      y: 50,
    });

    nodeMap.set(vpcNode.id, groupModel);
    vpcIndex++;
  });

  // Create subnet group nodes (children of VPCs)
  let subnetIndex = 0;
  data.nodes.forEach((node) => {
    if (node.type === 'subnet' && node.parentId) {
      const childIds = nodesByParent.get(node.id)?.map((n) => n.id) || [];

      const subnetModel = createGroupNode({
        id: node.id,
        label: node.name,
        nodeType: NODE_TYPES.SUBNET,
        resourceId: node.resourceId,
        details: node.details,
        children: childIds,
        parent: node.parentId,
        width: 350,
        height: 250,
        x: subnetIndex * 50,
        y: subnetIndex * 50,
      });

      nodeMap.set(node.id, subnetModel);
      subnetIndex++;
    }
  });

  // Create leaf nodes (VNI, VM, Security Groups, ACLs)
  let leafIndex = 0;
  data.nodes.forEach((node) => {
    if (node.type === 'subnet' || node.type === 'vpc') {
      return; // Already handled as groups
    }

    const leafModel = createNodeModel({
      id: node.id,
      label: node.name,
      nodeType: node.type as any,
      resourceId: node.resourceId,
      details: node.details,
      parent: node.parentId,
      width: 60,
      height: 60,
      x: leafIndex * 100,
      y: leafIndex * 100,
    });

    nodeMap.set(node.id, leafModel);
    leafIndex++;
  });

  // Create edge models
  data.edges.forEach((edge, index) => {
    const sourceNode = data.nodes.find((n) => n.id === edge.source);
    const targetNode = data.nodes.find((n) => n.id === edge.target);

    if (!sourceNode || !targetNode) {
      return;
    }

    const edgeType = determineEdgeType(sourceNode.type, targetNode.type);
    const edgeModel = createEdgeModel({
      id: edge.id || `edge-${index}`,
      source: edge.source,
      target: edge.target,
      edgeType,
    });

    edgeModels.push(edgeModel);
  });

  return {
    nodes: Array.from(nodeMap.values()),
    edges: edgeModels,
  };
};

/**
 * Create a topology model from nodes and edges
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
 * Build ancestor path for a node
 */
export const getAncestorPath = (
  data: TopologyData,
  nodeId: string
): TopologyNode[] => {
  const path: TopologyNode[] = [];
  let current = findTopologyNode(data, nodeId);

  while (current) {
    path.unshift(current);
    current = current.parentId ? findTopologyNode(data, current.parentId) : undefined;
  }

  return path;
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
  let maxDepth = 0;

  data.nodes.forEach((node) => {
    nodesByType[node.type] = (nodesByType[node.type] || 0) + 1;

    // Calculate depth
    const path = getAncestorPath(data, node.id);
    maxDepth = Math.max(maxDepth, path.length);
  });

  return {
    totalNodes: data.nodes.length,
    nodesByType,
    totalEdges: data.edges.length,
    depth: maxDepth,
  };
};
